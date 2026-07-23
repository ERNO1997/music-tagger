## Context

`FileRecord` already carries `ArtistMBID`, `ReleaseGroupMBID`, `ReleaseMBID`, and `RecordingMBID` once a file is identified (populated by the `musicbrainz-metadata` capability). The `library-browsing` capability's `ListArtists`/`ListAlbums`/`ListTracks` (in `sqlite_store.go`) currently group purely by `COALESCE(NULLIF(artist,''), NULLIF(raw_artist,''), 'Unknown Artist')` string equality — the MBID columns are stored but never read back for grouping. `MusicBrainzClient` already has one release-group-scoped lookup (`Releases`, used for cover art editions) that this change's new lookups follow the same shape as.

## Goals / Non-Goals

**Goals:**
- Group artists/albums by MBID when available, with name as the fallback for unidentified files.
- Surface, not silently resolve, any disagreement a grouping decision papers over.
- Let a user see how much of an artist's or album's MusicBrainz catalog they actually have, on-demand.

**Non-Goals:**
- No UI to customize the release-type filter (Album/EP default) in this change — a fixed default is enough for v1.
- No background/eager completeness pre-fetching across a full artist or album list — single-target, on-demand only.
- No attempt to reconcile which release-edition a mismatched release-group "should" use — mismatches are reported, not auto-fixed.
- Folder tree view and table/grid views are untouched.

## Decisions

### 1. Grouping computed in Go, not SQL
Fetch `(path, artist, raw_artist, artist_mbid, album, raw_album, release_group_mbid)` rows once per request and compute grouping key, representative label, and mismatch flags in Go, rather than expressing the frequency-ranking (which name string is "representative" of an MBID group) as a SQLite window-function query.
**Why over SQL CTEs/window functions**: the mismatch-detection logic (below) needs two passes over the same rows viewed two different ways (by key, and by label) — doable in SQL but hard to read/maintain, and correctness matters more than a DB round trip for a personal-library-sized table. Keeps the tricky logic unit-testable in Go without a live SQLite instance.

### 2. Grouping key and representative label
- Artist grouping key: `artist_mbid` if non-empty, else `"name:" + COALESCE(artist, raw_artist, "Unknown Artist")`.
- Album grouping key: scoped within the artist key; `release_group_mbid` if non-empty, else `"name:" + COALESCE(album, raw_album, "Unknown Album")`.
- Representative label per group: the most-frequent non-blank name string observed among that group's files (ties broken by name, case-insensitive). This is what's rendered in the UI; the key is an internal identifier.

### 3. New `artist_key`/`album_key` response fields, used for drill-down
Today the API drills down by name (`GET /api/v1/library/albums?artist=<name>`). Under MBID grouping this becomes ambiguous exactly in the case we most want to flag: two different groups (different keys) whose representative labels collide. `ListArtists`/`ListAlbums` responses gain an `artist_key`/`album_key` field; `GET /api/v1/library/albums` and `GET /api/v1/library/tracks` (and the new completeness endpoints) accept `artist_key`/`album_key` as the primary way to select a group, keeping `artist`/`album` name params supported for backward compatibility (resolved to a key the same way grouping does, since the frontend must be updated in this same change anyway).
**Why**: without a stable key, two colliding-label groups aren't distinguishable through the API a user would use to drill in — undermining the whole point of flagging the mismatch.

### 4. Mismatch detection covers both directions
- **Name variance within a group**: a group's key is consistent (same MBID) but its members disagree on the name string. Flag: `name_mismatch: true`, with the set of distinct names observed.
- **Label collision across groups**: two different keys (e.g. two different `artist_mbid`s, or one MBID-keyed and one name-keyed group) resolve to the same representative label. Flag both groups: `label_collision: true`.
Both flags are computed the same pass in Go: group by key for the first, then group the resulting labels for the second.
Unidentified (`name:`-keyed) groups can only ever participate in label collisions, never name-variance (there's no MBID to disagree about).

### 5. Completeness check reuses already-known MBIDs, one MB request per check
- **Album completeness**: use the `ReleaseMBID` already recorded on the album's own identified tracks (most-frequent one, if they vary — see Risks) to call the new `ReleaseTracklist(releaseMBID)` lookup once, then diff local `RecordingMBID`s present in the album against the response's recording IDs. Result: `have`/`total` counts plus the missing tracks' titles and track numbers.
- **Artist completeness**: call the new `ArtistReleaseGroups(artistMBID)` lookup once, filtered server-side to primary-type Album/EP with no Compilation/Live/Remix/Soundtrack/DJ-mix secondary type, then diff local `ReleaseGroupMBID`s present under the artist against the response's release-group IDs. Result: `have`/`total` counts plus the missing albums' titles and years.
- Neither check is available for a `name:`-keyed (unidentified) group — there's no MBID to look anything up with; the UI simply doesn't offer the action there.

### 6. Trigger: automatic on drill-in, plus manual refresh, both non-blocking
Opening an artist's album list or an album's track list kicks off its completeness check automatically; the group's own data renders immediately from local store data, with the completeness panel filling in asynchronously (loading → result/error state) so the 1 req/sec MusicBrainz round trip never blocks navigation. A manual "Recheck" action re-issues the same request, bypassing the short in-process cache described below.

### 7. Short in-process cache to absorb repeat navigation
Cache completeness results in-memory, keyed by `(kind, mbid)`, TTL 15 minutes, no persistence. Covers the common case of navigating back and forth between a few artists/albums in one session without re-hitting MusicBrainz every time, while staying simple (no invalidation logic beyond TTL + manual refresh bypass).

## Risks / Trade-offs

- **[Risk]** An album's identified tracks could carry more than one distinct `ReleaseMBID` (e.g. partially identified against different pressings before a re-identify). → Use the most-frequent `ReleaseMBID` for the tracklist call, and this itself is already caught by a variant of the mismatch flag (album-level `release_mismatch`) so the user sees why the completeness count might look off.
- **[Risk]** Backward-compatible `artist`/`album` name params on drill-down endpoints re-derive a key from name, which is lossy exactly when a label collision exists (the ambiguity the key was introduced to solve). → Acceptable since the frontend is updated in this same change to always use keys; the name params remain only as a compatibility shim for any other API consumer, with a doc note that they can't disambiguate collisions.
- **[Risk]** MusicBrainz artist discography responses can be large for prolific artists and are paginated by the API. → `ArtistReleaseGroups` follows pagination internally (single logical call to the caller, multiple HTTP requests under the rate gate if needed); cap at a reasonable page limit and surface a "results may be incomplete" flag if the cap is hit, rather than looping unbounded.
- **[Trade-off]** Fixed Album/EP-only default with no UI override means some users' desired scope (e.g. wanting live albums counted) isn't configurable in v1. Acceptable per Non-Goals; revisit if requested.

## Open Questions

- Exact secondary-type exclusion list for artist discography filtering (Compilation/Live/Remix/Soundtrack/DJ-mix/Mixtape assumed — confirm during implementation against real MusicBrainz data shapes).
- Whether `label_collision` should also compare against `raw_artist`/`raw_album` values of unidentified files more aggressively, or just exact representative-label string equality (assumed: exact, case-insensitive, for v1).
