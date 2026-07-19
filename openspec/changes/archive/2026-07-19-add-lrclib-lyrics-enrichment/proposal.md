## Why

`project.md`'s original enrichment step always bundled cover art *and* lyrics together as one pipeline stage. We already built the cover-art half; LRCLIB (unlike Genius or Musixmatch) needs no scraping, no paid tier, and no auth — so this is a small, low-risk extension of the enrichment we already have, not a new integration pattern.

## What Changes

- Add an LRCLIB gateway client: given artist, track title, album, and duration (all already stored on every identified file), `GET https://lrclib.net/api/get` and parse `plainLyrics`, `syncedLyrics`, `instrumental`.
- Treat a 404 (`TrackNotFound`) or `instrumental: true` as "no lyrics available" — not an error, same pattern as cover art's 404 handling.
- Reuse the existing "Enrich Selected" action rather than adding a new button/job — per `project.md`'s original design, cover art and lyrics are the same conceptual step. `EnrichFile.Enrich` now does both lookups per file (cover art, then lyrics), each independently — a failure or "not found" in one doesn't skip the other.
- Extend the tracking store with `Lyrics` (plain text) and `SyncedLyrics` (LRC-timed text) fields.
- `GET /api/v1/library` gains a lightweight `has_lyrics` indicator (not the full text — same reasoning as cover art: don't bloat the polling-heavy list response with large text for every row).
- New `GET /api/v1/library/lyrics?path=...` endpoint returns the full lyrics (plain + synced) as JSON, fetched only when the details view actually opens for that file.
- Web UI: details view gains a scrollable lyrics section, fetched on open; table row shows a small lyrics indicator, not the full text.

**Explicitly out of scope**: embedding lyrics into the actual audio file's tags (future tagging capability), any UI for editing/correcting lyrics, submitting corrections back to LRCLIB.

## Capabilities

### New Capabilities
- `lyrics-lookup`: Resolving artist/title/album/duration to plain and synced lyrics via LRCLIB.

### Modified Capabilities
- `file-tracking-store`: Persists `Lyrics`/`SyncedLyrics`; the existing "Enrichment results are recorded per file" requirement broadens to cover lyrics alongside cover art.
- `music-library-scan`: `GET /api/v1/library` gains `has_lyrics`; new `GET /api/v1/library/lyrics` endpoint; "On-demand enrichment action" now resolves lyrics too; details view shows lyrics.

## Impact

- New code: `internal/infrastructure/gateways/lrclib_client.go`; a new `LyricsLookup` port; `EnrichFile.Enrich` extended (not replaced) to call both `CoverArtLookup` and `LyricsLookup`; a new `RecordLyrics` (or similar) store method.
- Schema: new `lyrics`/`synced_lyrics` columns, same idempotent migration pattern.
- One new API endpoint (`GET /api/v1/library/lyrics`); no changes to the trigger/status endpoints (`POST/GET /api/v1/library/enrich` unchanged in shape).
- No new external dependencies, no new job manager, no new UI buttons.
