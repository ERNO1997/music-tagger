## Context

`IdentifyFile.Identify` (`internal/usecases/identify_file.go`) calls `AcoustIDLookup.Lookup`, which returns `[]AcoustIDMatch{RecordingID, Score}` ranked by AcoustID's own descending score, and unconditionally uses `matches[0]` — there is no confidence check anywhere in the path. A real false positive was observed: a Daft Punk "Get Lucky (Radio Edit)" file resolved to an unrelated compilation-series recording.

`LRCLIBClient.Lookup` (`internal/infrastructure/gateways/lrclib_client.go`) only calls LRCLIB's `/api/get` endpoint, an exact-match lookup requiring precise artist/title/album strings and duration within a couple of seconds — confirmed to 404 for a song (Lenka, "Everything at Once") that genuinely exists in LRCLIB's database. A live check against LRCLIB's `/api/search?track_name=...&artist_name=...` for that same song returns an array of near-duplicate entries (same lyrics, different album metadata — "Two", "Everything At Once", "Bravo Hits 80", "De Afrekening Volume 54", "Różne" — evidently the same recording cross-listed under several releases/compilations LRCLIB's contributors uploaded separately) with no numeric relevance score, just `id`, `trackName`, `artistName`, `albumName`, `duration`, `instrumental`, `plainLyrics`, `syncedLyrics`.

`GET /api/v1/library` already supports `tagged`/`relocated` boolean filters (`LibraryFilter.Tagged`/`.Relocated` in `internal/usecases/ports.go`, WHERE-clause construction in `internal/infrastructure/persistence/sqlite_store.go`'s `buildLibraryWhere`) — `has_lyrics` is a straightforward third boolean dimension of the same shape.

## Goals / Non-Goals

**Goals:**
- A file only gets `identified` status when AcoustID's confidence in the match clears a minimum bar; otherwise it's `not_found`, matching the user's explicit preference for no metadata over wrong metadata.
- LRCLIB lyrics lookups succeed for songs that exist in LRCLIB's database but don't match our stored metadata closely enough for an exact lookup.
- The library list/UI can filter to files missing lyrics, the same way it already filters on tagged/relocated outcome.

**Non-Goals:**
- Ranking or deduplicating LRCLIB's near-duplicate cross-album entries beyond picking one reasonable candidate — that's a LRCLIB data-quality characteristic, not something this change needs to fully solve.
- Any change to how AcoustID/MusicBrainz results are displayed or selected by a human (that's the separate, larger "manual search and pick a candidate" idea, deliberately deferred).
- Changing the `speed-up-library-scan` change's fingerprinting/duration work — independent, developed in parallel.

## Decisions

### AcoustID confidence threshold lives in `IdentifyFile.Identify`, not in the `acoustid-lookup` gateway
`AcoustIDClient.Lookup` keeps returning every match AcoustID gives back, ranked by score, unfiltered — that capability's job is resolution, not policy. The minimum-confidence decision ("is this good enough for us to trust") is applied where the outcome gets recorded: `IdentifyFile.Identify` checks `matches[0].Score` against a named constant (`minAcoustIDConfidence = 0.7`, easy to retune later) before calling MusicBrainz at all; below it, the file is recorded exactly like a no-match (`domain.StatusNotFound`), no MusicBrainz call made. This mirrors the project's existing convention (established in the pagination change) of keeping lookup/store capabilities dumb and putting acceptance policy in the consuming usecase. Alternative considered: filter inside `AcoustIDClient` itself — rejected, since "how confident is confident enough" is this application's policy, not an inherent property of the AcoustID API resolution step, and burying it in the gateway would make it harder to find/tune and impossible to unit-test independently of an HTTP mock.

### LRCLIB fallback: fuzzy `/api/search`, best candidate chosen by closest duration
When `/api/get` 404s, `LRCLIBClient.Lookup` calls `/api/search?track_name=<title>&artist_name=<artist>` (album omitted — the confirmed real query showed LRCLIB's cross-album duplicates all have correct lyrics regardless of album match, so requiring album equality here would just reintroduce the same over-strict-matching problem this fallback exists to solve). The search response has no relevance score, so among the returned candidates, the one whose `duration` is closest to our already-known `durationSeconds` parameter is chosen (falling back to the first result if `durationSeconds` is 0/unknown) — this is a much better signal than "first result" alone, since the LRCLIB response for "Everything at Once" showed multiple entries with lyrics-identical content but a range of durations (158–159s) reflecting genuinely distinct masters/edits; picking the closest-duration one is the cheapest available correctness check. An empty search result is treated as "not found," identical to today's `/api/get` 404 handling. Alternative considered: use the first search result unconditionally — rejected once the real API response showed multiple candidates without a ranking signal we can trust to already reflect duration-closeness.

### `has_lyrics` filter: identical shape to `tagged`/`relocated`
`LibraryFilter` gains `HasLyrics *bool`; `buildLibraryWhere` gains a clause `(lyrics != '' OR synced_lyrics != '')` (or its negation) when non-nil, exactly parallel to the existing `tagged`/`relocated` boolean clauses; `GET /api/v1/library` gains a `has_lyrics` query parameter parsed the same way as `tagged`/`relocated`; the web UI gains one more filter `<select>`, styled and wired identically to the existing two.

## Risks / Trade-offs

- **[Risk] Raising the AcoustID confidence bar discards some correct-but-lower-confidence matches as `not_found`** → Accepted; this is exactly the trade-off the user explicitly asked for.
- **[Risk] `minAcoustIDConfidence = 0.7` is a guess, not empirically derived** → Mitigated by making it a single named constant, trivial to retune after observing real results; not exposed as a runtime config knob for now since this is a single-operator tool and a code-level tweak is fast enough.
- **[Risk] Choosing "closest duration" among LRCLIB fuzzy-search candidates can still occasionally pick a lyrically-different result** (e.g. a genuine remix with a coincidentally similar duration) → Accepted as a modest, bounded risk; the alternative (no fallback at all) is strictly worse — today's exact-match-only behavior already misses genuinely-present lyrics entirely for cases like this.
- **[Trade-off] `has_lyrics` filtering by `lyrics != '' OR synced_lyrics != ''` treats "has only synced lyrics" and "has only plain lyrics" as equally "has lyrics"** → Consistent with the existing `has_lyrics` indicator already shown per-row elsewhere in the API, so no new inconsistency introduced.

## Migration Plan

- No schema changes. No rollback concerns beyond reverting the code — none of these changes alter stored data in a way the old code couldn't still read.
