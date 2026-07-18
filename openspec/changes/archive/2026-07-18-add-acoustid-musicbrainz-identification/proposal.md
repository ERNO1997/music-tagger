## Why

Every tracked file currently sits at status `new` forever — there's no way to actually resolve its canonical artist/album/title. This change adds the missing identification step: on-demand, single or bulk via user selection, triggered from the UI, so the user decides which files to identify and when, rather than the system blasting the whole library against MusicBrainz's 1 req/sec-limited API automatically. This is the piece that turns "we know this file exists" into "we know what this file is."

## What Changes

- Add an AcoustID gateway client: submits a fingerprint + duration, returns the resolved MusicBrainz Recording ID(s) (requires an `ACOUSTID_API_KEY`).
- Add a MusicBrainz gateway client: given a Recording ID, resolves canonical artist, release (album), track title, and track number. Enforces the 1 req/sec rate limit **centrally inside the client itself** (a shared minimum-interval gate) so every caller is paced identically regardless of who's asking, per `project.md` §4.2. Requires a MusicBrainz-compliant `User-Agent` (app name/version/contact, configurable).
- Add `POST /api/v1/library/identify` accepting `{"paths": ["...", "..."]}` — always a list, so a single-row click and a bulk selection use the same code path. Starts a background job (its own guard, separate from the scan/refresh job since they touch different resources) that works through the list at the enforced 1 req/sec pace and returns `202 Accepted` immediately, or `409` if an identify job is already running.
- Add `GET /api/v1/library/identify/status` reporting running/processed/total, mirroring the existing scan-status endpoint.
- For each file processed: match found → status `identified`, resolved artist/album/title/track number stored on its tracking-store record; no match → status `not_found`; gateway/network error → status left unchanged, error surfaced.
- Extend the tracking store's schema with the resolved metadata fields (artist, album, title, track_number) — already anticipated as a future step when the store was first introduced.
- Update the web UI: row checkboxes + an "Identify Selected" bulk action (a single-row "Identify" button is just a 1-item selection), reusing the same disabled-while-running + progress-polling pattern already built for Refresh, and displaying resolved artist/album/title/track once a row becomes `identified`.

**Explicitly out of scope**: no cover art (Cover Art Archive) or lyrics (Genius) fetching, no tagging, no file relocation, and no *automatic* identification of the whole library on scan — identification is always a deliberate selection, never implicit.

## Capabilities

### New Capabilities
- `acoustid-lookup`: Resolving a fingerprint + duration to MusicBrainz Recording ID(s) via AcoustID.
- `musicbrainz-metadata`: Resolving a Recording ID to canonical artist/release/track/track-number data, with the 1 req/sec limit enforced centrally regardless of caller.

### Modified Capabilities
- `file-tracking-store`: Adds resolved-metadata fields (artist, album, title, track_number) and records real `identified`/`not_found` outcomes from an actual identification attempt.
- `music-library-scan`: `GET /api/v1/library` entries include resolved metadata when present; adds the identify trigger/status endpoints; UI gains row selection, bulk Identify action, progress, and metadata display.

## Impact

- New code: `internal/infrastructure/gateways/acoustid_client.go`, `musicbrainz_client.go`; a new identify usecase plus its own background-job manager (parallel in shape to `RefreshManager`); new `AcoustIDLookup`/`MusicBrainzLookup` ports in `internal/usecases/ports.go`; schema/`FileRecord` additions in the persistence layer; a new `identify_handler.go` and two new routes.
- New required configuration: `ACOUSTID_API_KEY`, a MusicBrainz `User-Agent` string — both plumbed through `docker-compose.yml`/`.env.example`.
- No tagging, no relocation, no cover art/lyrics code yet.
