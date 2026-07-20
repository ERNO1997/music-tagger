## Why

Real-world use surfaced two separate accuracy gaps: (1) `IdentifyFile.Identify` accepts AcoustID's top-ranked match completely unconditionally, with no confidence check at all, which produced a real false positive — a Daft Punk "Get Lucky (Radio Edit)" file got tagged with metadata from an unrelated compilation-series recording. (2) `LRCLIBClient.Lookup` only calls LRCLIB's exact-match endpoint, which 404s whenever the stored artist/title/album/duration don't line up precisely enough with LRCLIB's own data — even for songs that genuinely exist in LRCLIB's database and are trivially found via its fuzzy search (confirmed: Lenka's "Everything at Once"). Separately, there's no way to filter the library list down to files missing lyrics, even though `tagged`/`relocated` boolean filters already exist for the equivalent purpose on other enrichment outcomes.

## What Changes

- `IdentifyFile.Identify` gains a minimum AcoustID confidence score threshold. A file whose best AcoustID match scores below that threshold is recorded as `not_found` instead of `identified` — preferring no metadata over wrong metadata, per explicit user preference. The threshold is a named, easily-tunable constant.
- `LRCLIBClient.Lookup` falls back to LRCLIB's fuzzy `/api/search` endpoint when the exact-match `/api/get` endpoint returns no result, taking the best-scoring candidate rather than giving up. Fully contained inside the client — the `LyricsLookup` port signature is unchanged.
- `GET /api/v1/library` gains a `has_lyrics` boolean filter, mirroring the existing `tagged`/`relocated` filters exactly (same `LibraryFilter` shape, same WHERE-clause pattern, same UI filter-control treatment).

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `file-tracking-store`: identification now records `not_found` (rather than `identified`) when AcoustID's best match is below a minimum confidence threshold.
- `lyrics-lookup`: lyrics resolution now falls back to a fuzzy, ranked search when an exact-match lookup finds nothing, before giving up.
- `music-library-scan`: `GET /api/v1/library` and the web UI gain a `has_lyrics` filter dimension, alongside the existing `tagged`/`relocated` filters.

## Impact

- Changed code: `internal/usecases/identify_file.go` (confidence threshold check before accepting a match), `internal/infrastructure/gateways/lrclib_client.go` (fuzzy-search fallback, self-contained), `internal/usecases/ports.go` / `internal/infrastructure/persistence/sqlite_store.go` / `internal/infrastructure/web/v1/library_handler.go` / `ui/index.html` / `ui/js/app.js` (new `has_lyrics` filter dimension, same shape as `tagged`/`relocated`).
- No schema changes — `lyrics`/`synced_lyrics` columns already exist; `has_lyrics` is derived (`lyrics != '' OR synced_lyrics != ''`), same as the existing `has_lyrics` indicator already shown per-row.
- No new external dependencies — LRCLIB's `/api/search` endpoint is part of the same public API already in use; AcoustID's `score` field is already returned today, just not yet checked against a threshold.
- Independent of, and can be developed in parallel with, the separate `speed-up-library-scan` change (that change touches scan/fingerprint timing; this one touches match-acceptance policy and lyrics search/filtering — no shared files beyond both eventually needing `go build ./...` to pass together).
