## 1. MusicBrainz gateway: new lookups

- [x] 1.1 Add `mbArtistReleaseGroups`/`mbReleaseGroupSummary` response shapes and an `ArtistReleaseGroups(ctx, artistMBID) ([]usecases.ArtistReleaseGroupSummary, error)` method to `internal/infrastructure/gateways/musicbrainz_client.go`, querying `GET /artist/{id}?inc=release-groups&fmt=json`, filtering to primary-type Album/EP and excluding Compilation/Live/Remix/Soundtrack/DJ-mix/Mixtape secondary types, following pagination up to a bounded page limit, all through the existing `waitForRateGate`.
- [x] 1.2 Add `mbReleaseRecordings` response shapes and a `ReleaseTracklist(ctx, releaseMBID) ([]usecases.ReleaseTrackSummary, error)` method querying `GET /release/{id}?inc=recordings&fmt=json`, returning each track's recording MBID, title, and track number across all media.
- [x] 1.3 Define `usecases.ArtistReleaseGroupSummary` and `usecases.ReleaseTrackSummary` types, and `usecases.MusicBrainzDiscographyLookup` port interface in `internal/usecases/ports.go`; assert `*MusicBrainzClient` satisfies it alongside the existing port assertions.
- [x] 1.4 Unit tests for both new client methods: successful resolution, non-qualifying release-groups excluded, pagination followed and bounded, request failure surfaced as an error (not empty result).

## 2. Grouping key, representative label, and mismatch detection

- [x] 2.1 Add a Go-side grouping helper (e.g. `internal/usecases/library_grouping.go` or alongside the store) that takes rows of `(artist, raw_artist, artist_mbid)` and computes: grouping key (`artist_mbid` if non-empty else `"name:" + fallback`), representative label (most-frequent non-blank name, ties broken case-insensitively), and `name_mismatch`/`label_collision` flags, per design.md decisions 2 and 4.
- [x] 2.2 Apply the same helper pattern scoped within an artist group for albums (`release_group_mbid` / album name fallback).
- [x] 2.3 Update `SQLiteStore.ListArtists` to fetch the raw rows needed and return grouped+flagged results via the new helper, replacing the current `GROUP BY artist_name` SQL.
- [x] 2.4 Update `SQLiteStore.ListAlbums` similarly, accepting an `artist_key` (falling back to resolving `artist` name to a key first, for backward compatibility).
- [x] 2.5 Update `SQLiteStore.ListTracks` to accept `artist_key`/`album_key` (with `artist`/`album` name fallback resolution), matching rows by the same grouping logic used for listing.
- [x] 2.6 Update `usecases.ArtistSummary`/`usecases.AlbumSummary` (in `internal/usecases/ports.go`) to add `Key`, `NameMismatch`, `LabelCollision` fields (and distinct-names list for the mismatch case).
- [x] 2.7 Unit tests: two artists sharing a name string but different MBIDs stay separate; one artist with inconsistent name strings under one MBID stays merged with `name_mismatch` set; label collision flags both sides; unidentified groups never get `name_mismatch`; existing name-based filter/pagination behavior still passes.

## 3. Completeness check use case

- [x] 3.1 Add `internal/usecases/check_completeness.go` with an `ArtistCompleteness`/`AlbumCompleteness` use case: given an `artist_key`/`album_key`, resolve the underlying MBID (error/empty-result if the group has none), call the gateway lookup, diff against local `ReleaseGroupMBID`s (artist case) or `RecordingMBID`s (album case, using the group's most-frequent `ReleaseMBID` per design.md decision 5), and return have/total counts plus the missing items.
- [x] 3.2 Add an in-process, TTL-based (15 min) cache keyed by `(kind, mbid)` in front of the gateway calls, with a bypass path for manual "recheck" requests.
- [x] 3.3 Unit tests: partial completeness (some missing), full completeness (nothing missing), unavailable for a name-keyed group, gateway failure surfaced distinctly from "nothing missing," cache hit avoids a second gateway call, manual recheck bypasses the cache.

## 4. API layer

- [x] 4.1 Update `ArtistEntry`/`AlbumEntry` response structs in `internal/infrastructure/web/v1/artist_album_handler.go` to include `artist_key`/`album_key`, `name_mismatch`, `label_collision` (and distinct-names detail where applicable).
- [x] 4.2 Update `Albums`/`Tracks` handlers to accept `artist_key`/`album_key` query params, falling back to the existing `artist`/`album` name params when keys are absent.
- [x] 4.3 Add `GET /api/v1/library/artists/completeness?artist_key=<key>` and `GET /api/v1/library/albums/completeness?artist_key=<key>&album_key=<key>` handlers wired to the new use case, plus a `refresh=true` param to bypass the cache; wire routes in `internal/infrastructure/web/v1/router.go`.
- [x] 4.4 Handler-level tests: key-based and name-based drill-down both work, completeness endpoints return the expected shape for a mocked gateway, unavailable-for-unidentified-group returns the documented error/empty response.

## 5. Frontend: Artist-Album view

- [x] 5.1 Update `ArtistAlbumGroupingView.vue` (and any shared API client module) to read and pass `artist_key`/`album_key` for navigation instead of raw names.
- [x] 5.2 Add a mismatch warning indicator on artist/album cards with `name_mismatch` or `label_collision` set, with a tooltip/expansion showing the distinct names or the colliding grouping's identity.
- [x] 5.3 Add a completeness panel that loads asynchronously (spinner → have/total + missing list, or error state with retry) when entering an artist's album list or an album's track list; only rendered/offered for MBID-keyed groups.
- [x] 5.4 Add a manual "Recheck" action that re-triggers the completeness call with the cache-bypass param.
- [x] 5.5 Verify selection state (per the existing `library-browsing` selection requirement) is unaffected by the key-based navigation change — confirmed by inspection: selection is keyed purely by file path in store.js, independent of artist/album grouping.

## 6. End-to-end verification

- [x] 6.1 Run the full Go test suite (all packages pass) and the frontend build (`npm run build`, no test suite exists in this project — confirmed no Vue/JS test framework configured); no regressions from the grouping-query rewrite.
- [x] 6.2 Manually verified against a running server (real MusicBrainz calls, no mocks): (1) a seeded label-collision (two artists sharing the "Overlap" name string, different MBIDs) correctly flagged `label_collision: true` on both via `GET /library/artists`, with `GET /library/albums?artist_key=...` correctly scoping to only that artist's own album; (2) a real artist MBID (Radiohead) returned its actual MusicBrainz discography (10 Album/EP release-groups, live/compilation/single excluded) via the completeness endpoint; (3) an unidentified/never-seen artist name correctly returned 422 (`ErrCompletenessUnavailable`) from both `artist_key` and name-fallback paths. Did not additionally drive the Vue UI in a browser (no real fingerprinted/identified audio files available in this environment to click through), but the full API pipeline the UI depends on is verified end-to-end.
