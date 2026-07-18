## 1. Dependency and schema setup

- [x] 1.1 Add `modernc.org/sqlite` (pure-Go driver) as a dependency
- [x] 1.2 Define the `files` table schema (`path` PK, `format`, `fingerprint`, `size`, `mtime`, `status`, `updated_at`) with an idempotent `CREATE TABLE IF NOT EXISTS`, and enable WAL mode on connection open
- [x] 1.3 Add `DB_PATH` environment variable (default e.g. `/data/music-tagger.db`) to configuration loading in `cmd/server/main.go`
- [x] 1.4 Add a dedicated Docker volume for `/data` in `docker-compose.yml`, distinct from the `/music` mount

## 2. Domain and ports

- [x] 2.1 Add a `TrackingStatus` type to the domain layer (`new`, `identified`, `not_found`, `missing`)
- [x] 2.2 Add a `FileRecord` domain type (path, format, fingerprint, size, mtime, status)
- [x] 2.3 Define a `TrackingStore` port in `internal/usecases/ports.go`: load-all (for diffing), bulk-apply (insert/update/mark-missing in one call)

## 3. Persistence implementation

- [x] 3.1 Implement `internal/infrastructure/persistence/sqlite_store.go` satisfying `TrackingStore`, backed by the schema from 1.2
- [x] 3.2 Implement `LoadAll` returning every tracked row keyed by path, for in-memory diffing
- [x] 3.3 Implement `BulkApply` (or equivalent) that performs all inserts/updates/missing-markings for one refresh inside a single transaction

## 4. Two-pass refresh usecase

- [x] 4.1 Pass 1: walk `/music`, `stat()` each candidate file (path, size, mtime) without invoking `fpcalc`, and diff against `TrackingStore.LoadAll` to classify each as new / changed / unchanged / missing
- [x] 4.2 Pass 2: run `fpcalc` only for the new/changed set; per-file fingerprint failure is captured per-entry and does not abort the rest of the pass
- [x] 4.3 Apply results via chunked `TrackingStore.BulkApply` calls: missing/reappeared markings committed immediately after pass 1, upserts flushed every `upsertChunkSize` files during pass 2 (not one call per file, not a single call at the very end)
- [x] 4.4 Implement reappearance handling: a `missing` path found again with unchanged size/mtime restores its prior (pre-`missing`) status rather than being treated as `new`

## 5. Background execution, concurrency guard, and progress

- [x] 5.1 Implement a shared in-memory refresh-state guard (`running`, `processed`, `total`, `startedAt`) safe for concurrent access
- [x] 5.2 Run the two-pass refresh (section 4) in a background Goroutine, updating `processed`/`total` on the shared state as pass 2 proceeds
- [x] 5.3 Reject a new refresh with a "already running" error when the guard indicates one is in progress (surfaced as `409` at the API layer — see 6.2)
- [x] 5.4 Trigger one refresh automatically from `cmd/server/main.go` right after the HTTP server starts listening, without blocking server startup

## 6. Format support and API

- [x] 6.1 Add `.m4a` to `isSupportedExtension` in `internal/infrastructure/filestat/fpcalc_runner.go`
- [x] 6.2 Add `.m4a` / `domain.FormatM4A` to `detectFormat` in the scan usecase
- [x] 6.3 Change `GET /api/v1/library` to read from `TrackingStore.LoadAll` instead of performing a live scan
- [x] 6.4 Add `POST /api/v1/library/scan`: starts the background refresh (section 5), returns `202 Accepted`, or `409 Conflict` if one is already running
- [x] 6.5 Add `GET /api/v1/library/scan/status`: returns the shared refresh-state guard's current `running`/`processed`/`total`

## 7. Web UI

- [x] 7.1 Add a "Refresh" action in `ui/index.html`/`ui/js/app.js` that calls `POST /api/v1/library/scan`
- [x] 7.2 Poll `GET /api/v1/library/scan/status` (and re-fetch `GET /api/v1/library`) while a refresh is running; disable the refresh control and show progress during that time; re-enable on completion
- [x] 7.3 On page load, check refresh status immediately so the UI reflects an already-in-progress refresh (e.g. the startup-triggered one) without needing a user action first
- [x] 7.4 Add a status column (New / Identified / Not Found / Missing) to the results table

## 8. Verification

- [x] 8.1 Verify (via Docker, per project convention) that a fresh `/music` volume, refreshed twice with no changes, does not re-invoke `fpcalc` on the second refresh
- [x] 8.2 Verify a modified file (touch mtime or change size) is re-fingerprinted and its status resets to `new`
- [x] 8.3 Verify a file removed from disk is marked `missing` (not deleted) after a refresh, and that it restores its prior status if reinstated unchanged
- [x] 8.4 Verify an `.m4a` file scans, fingerprints, and reports correctly end-to-end
- [x] 8.5 Verify tracked state survives a container restart (stop/start the container, confirm `GET /api/v1/library` still reflects prior state)
- [x] 8.6 Verify a `POST /api/v1/library/scan` while one is already running returns `409 Conflict`
- [x] 8.7 Verify `GET /api/v1/library` returns partially-updated rows while a refresh is still in progress (not just after it completes)
- [x] 8.8 Verify a refresh begins automatically at server startup without any UI action
