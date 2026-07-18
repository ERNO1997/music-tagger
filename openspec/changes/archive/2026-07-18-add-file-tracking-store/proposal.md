## Why

Every capability built so far treats each scan as stateless — `GET /api/v1/library` re-walks and re-fingerprints the entire volume on every request, with no memory of what was there before. That breaks down as soon as identification (a follow-up change) needs to know which files are new, which already have confirmed metadata, and which have disappeared from disk — and it forces identification to happen inline with scanning rather than on demand. This change introduces persistent, per-file state so the system can answer "what's the status of this file?" without re-deriving it every time, and so identification can later be triggered per file, independently, whenever the user chooses.

## What Changes

- Add a SQLite-backed store (pure-Go driver, no CGO) that persists one row per discovered file: path, format, fingerprint, size, mtime, and a status (`new`, `identified`, `not_found`, `missing`).
- Change scan behavior to a two-pass, batched refresh instead of a per-file scan-and-write:
  - Pass 1: cheaply stat every file on disk and diff against the currently tracked rows (one query) to classify each as new / changed / unchanged / missing — no `fpcalc`, no DB writes yet.
  - Pass 2: run `fpcalc` only for the new/changed set.
  - All resulting inserts/updates/missing-markings are then applied in a single batched database transaction, rather than one commit per file.
  - A file previously tracked but no longer found on disk → marked `missing` (row is kept, not deleted, so history isn't silently lost); reappearing unchanged restores its prior status.
- Make the refresh **asynchronous**: `POST /api/v1/library/scan` starts the two-pass process above in a background Goroutine and returns immediately (`202 Accepted`), instead of blocking the request for the whole library. A status endpoint reports whether a refresh is running and its progress (`processed`/`total`).
- Run a refresh automatically once at server startup, in addition to being triggerable on demand from the UI.
- Prevent concurrent refreshes: a second trigger while one is running gets `409 Conflict` rather than starting a duplicate pass.
- Split the current single "scan" behavior into two distinct actions: a fast read (`GET /api/v1/library`, always served from the store, reflecting live progress as the background refresh commits) and the async refresh trigger above.
- Extend supported formats to `.m4a` in both scanning and fingerprinting (`fpcalc` already decodes `.m4a` via `ffmpeg`, so this is primarily an extension-whitelist change, not new fingerprinting logic).
- Update the web UI: show each file's tracked status, disable the "Refresh" trigger while a refresh is in progress, and poll for progress so the user can see the scan is happening.

**Explicitly out of scope**: no AcoustID/MusicBrainz calls (that's the next change), no tagging, no relocation. This change only makes state durable and queryable.

## Capabilities

### New Capabilities
- `file-tracking-store`: SQLite-backed persistence of per-file discovery/identification state, with new/changed/missing detection across scans.

### Modified Capabilities
- `music-library-scan`: read path now serves from the store instead of scanning live; scanning becomes an explicit, separate refresh action; adds `.m4a` to supported formats.
- `audio-fingerprinting`: adds `.m4a` to the supported-extension whitelist.

## Impact

- New dependency: a pure-Go SQLite driver (e.g. `modernc.org/sqlite`).
- New code: `internal/infrastructure/persistence/sqlite_store.go`; a `TrackingStore` port in `internal/usecases/ports.go`; a rewritten `scan_local_volume.go` implementing the two-pass, batched-write refresh as a background Goroutine; a shared in-memory refresh-state guard; a new/changed `library_handler.go` split across read/refresh-trigger/refresh-status; `cmd/server/main.go` triggers a refresh once at startup.
- New runtime state: a SQLite file needs a persistent location — likely a new Docker volume distinct from `/music` (per `project.md` §2.4), configured via `docker-compose.yml`.
- API surface grows by two endpoints (refresh trigger, refresh status) beyond the existing read; no AcoustID/MusicBrainz/tagging/relocation code yet.
