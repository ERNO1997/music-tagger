## Why

Now that files carry a Release MBID from identification, the next piece of a fully-managed library is cover art — Cover Art Archive is a fully public, unauthenticated API keyed exactly by that MBID we already have. Lyrics (Genius) are deliberately deferred to a separate future decision, since fetching them requires page-scraping rather than a sanctioned API.

## What Changes

- Add a Cover Art Archive gateway client: given a Release MBID, `GET https://coverartarchive.org/release/{mbid}` to list images, select the front cover (`"front": true`, falling back to the first image if none is explicitly marked), then download the actual image bytes.
- Fetch the "large" thumbnail (~500px) rather than the full-resolution original — good enough for both future tag-embedding and UI display, and avoids storing/managing two separate copies per file.
- Store downloaded cover art as a file on disk under `/data/covers/<release-mbid>.jpg` — not a BLOB in SQLite, consistent with keeping the DB itself small and matching the existing file-based persistence pattern.
- Extend the tracking store with a `CoverArtPath` field (empty until enriched, or if no art is found).
- Add an "Enrich Selected" background job mirroring the existing on-demand Identify pattern exactly: its own `JobManager`/guard (independent of scan and identify), works over a list of paths, requires each already be `identified` (since it needs the Release MBID).
- `POST /api/v1/library/enrich` (paths list) → `202`/`409`; `GET /api/v1/library/enrich/status` → progress — same shape as the identify endpoints.
- `GET /api/v1/library` entries gain a cover-art indicator; a new `GET /api/v1/library/cover?path=...` endpoint serves the actual image bytes so the UI can `<img>` it directly.
- Web UI: a small cover thumbnail in the table row and in the details view, plus an "Enrich Selected" bulk action with the same disabled-while-running + progress-polling UX already used for Identify.

**Explicitly out of scope**: lyrics/Genius (separate future decision), tagging/embedding the art into actual audio files (still a later capability), any rate limiting for Cover Art Archive (no documented limit like MusicBrainz's, per `project.md` §2.3 — not adding an artificial one unless it proves necessary).

## Capabilities

### New Capabilities
- `cover-art-lookup`: Resolving a Release MBID to a front-cover image via Cover Art Archive.

### Modified Capabilities
- `file-tracking-store`: Persists the cover art file path.
- `music-library-scan`: `GET /api/v1/library` gains a cover-art indicator; new enrich trigger/status/image-serving endpoints; UI gains thumbnails + the Enrich Selected action.

## Impact

- New code: `internal/infrastructure/gateways/coverart_client.go`; a new `EnrichFile` usecase + `EnrichManager` (mirrors `IdentifyFile`/`IdentifyManager`, reusing the shared `JobManager`); a new `CoverArtLookup` port.
- Schema: new `cover_art_path` column, same idempotent migration pattern.
- New disk storage under `/data/covers/` — same existing `/data` volume, no new Docker volume needed.
- New API endpoints: `POST /api/v1/library/enrich`, `GET /api/v1/library/enrich/status`, `GET /api/v1/library/cover`.
- No new external dependencies beyond a plain HTTP client (already used for AcoustID/MusicBrainz).
