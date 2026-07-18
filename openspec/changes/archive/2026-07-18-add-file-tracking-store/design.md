## Context

The `music-library-scan` and `audio-fingerprinting` capabilities (archived under `openspec/changes/archive/2026-07-18-init-fingerprint-scan/`) currently make every `GET /api/v1/library` call re-walk `/music` and re-run `fpcalc` on every file, with no memory between requests. That's fine for a stateless "show me fingerprints" demo, but it can't answer "is this file new since last time?" or "has this one already been identified?" — both of which the upcoming on-demand identification change needs, and it doesn't scale to large libraries. This change adds the missing persistence layer, reshapes the scan into a read/refresh split, and makes the refresh asynchronous with visible progress, per `project.md` §2.4 and §1.3 Phase A.

## Goals / Non-Goals

**Goals:**
- Persist one row per discovered file (path, format, fingerprint, size, mtime, status) across process restarts.
- Detect three transitions on every refresh: new files, changed files (re-fingerprint), and missing files (previously tracked, no longer on disk) — without deleting history for the missing case.
- Avoid redundant `fpcalc` invocations for files that haven't changed since the last refresh.
- Make the refresh **cheap on the database side** by batching writes, and **cheap on the user's connection** by running in the background rather than holding one long HTTP request open.
- Run a refresh automatically once at server startup, in addition to being triggerable on demand from the UI.
- Prevent two refreshes from running concurrently, and make the running/idle state visible to the UI so it can disable its own trigger and show progress.
- Add `.m4a` as a third supported format across scanning and fingerprinting.

**Non-Goals:**
- No AcoustID/MusicBrainz calls, no tagging, no relocation — this change is purely about tracking state.
- No content-addressed deduplication across paths (two identical files at different paths get two independent rows).
- No migration framework — schema is small enough for a single idempotent `CREATE TABLE IF NOT EXISTS` at startup.
- No multi-task history/registry — only one refresh can ever be in flight at a time, so a single shared "is a refresh running" state is sufficient; this is simpler than (and does not need) the general `task_id` registry `project.md` reserves for the future full bulk-remediation pipeline.

## Decisions

- **Storage: SQLite via `modernc.org/sqlite` (pure Go, no CGO).** Keeps the static-binary constraint (`project.md` §2.2/§2.4) intact — the only alternative considered, `mattn/go-sqlite3`, requires CGO and was rejected for that reason. A full external DB (Postgres, etc.) was rejected as overkill for a single-writer, embedded, single-container app.
- **Schema**: one `files` table — `path TEXT PRIMARY KEY, format TEXT, fingerprint TEXT, size INTEGER, mtime INTEGER, status TEXT, updated_at INTEGER`. `path` is the natural key within one `/music` mount. WAL mode is enabled for safer concurrent read/write from a single process.
- **Two-pass refresh, not one file at a time.** Pass 1: walk `/music` collecting `(path, size, mtime)` for every candidate file — cheap `stat()`-only work — and load all currently tracked rows in a single `SELECT`. Diffing these two in memory classifies every file as unchanged / new / changed / missing without touching `fpcalc` or the database per file. Pass 2: run `fpcalc` only for the new/changed set. This is also what makes real progress reporting cheap: `total` is known (size of the new/changed set) before any fingerprinting starts.
- **Chunked bulk writes, not one commit per file and not one commit for the whole refresh.** Missing/reappeared paths are fully known right after pass 1 (they don't depend on fingerprinting) and are committed immediately in one transaction. Pass 2's upserts are committed in chunks of `upsertChunkSize` (25) files rather than individually or all at once at the end. SQLite's dominant per-write cost is the commit/fsync, not the statement itself, so this still turns "N commits" into "N/25 commits" — most of the savings of one-commit-per-refresh — while ensuring `GET /api/v1/library` actually sees new rows well before a large refresh finishes, rather than only at the very end.
- **Refresh runs as a background Goroutine, not inline with the HTTP request.** `POST /api/v1/library/scan` starts the two-pass process above in a Goroutine and returns immediately (`202 Accepted`). This avoids holding an HTTP connection open for the duration of a large-library refresh (a real risk behind reverse-proxy idle timeouts) and lets the UI show live progress instead of an opaque spinner.
- **A single in-memory "refresh in progress" guard, not a task registry.** Since only one refresh is meaningful at a time, a shared boolean/mutex state (`running`, `processed`, `total`, `startedAt`) is enough. A `POST` while one is already running returns `409 Conflict` (mirroring the same status code `project.md`'s reserved `/scan-local` contract already uses for this exact situation), and `GET /api/v1/library/scan/status` exposes the current state for the UI to poll.
- **Refresh also runs once automatically at server startup.** `cmd/server/main.go` kicks off the same background refresh right after the HTTP server starts listening (non-blocking — the server accepts requests immediately; the first refresh runs concurrently), so the tracking store is warmed up without requiring a manual UI action on first run.
- **Endpoint split**: `GET /api/v1/library` always reads directly from the store (fast, DB-only, safe to poll repeatedly) and reflects whatever the in-progress or most recent refresh has written so far — rows update incrementally as the background job commits. `POST /api/v1/library/scan` only starts a refresh; it does not return file data itself (the caller polls `GET /api/v1/library` and/or the status endpoint for that).
- **`.m4a` support**: `fpcalc` already decodes `.m4a` via its `ffmpeg`-based backend, so this only requires adding `.m4a` to the extension whitelist in `isSupportedExtension` (fingerprinting) and `detectFormat` (scanning) — no new fingerprinting logic.
- **DB location**: a new Docker volume distinct from `/music` (e.g. `/data/music-tagger.db`, configurable via a `DB_PATH` env var), so tracking state survives container recreation independently of the music library mount, and so nothing is ever written under `/music` — preserving the read-only-to-the-library invariant this capability has held since the first change.

## Risks / Trade-offs

- **Path-as-primary-key breaks continuity across directory renames** → if a user reorganizes folders on disk between refreshes, the old path is marked `missing` and the new path appears as `new`, losing its identification status. Mitigation: acceptable for this slice; a future enhancement could re-key on fingerprint match to recognize a moved-but-unchanged file, but that adds real complexity (a file can legitimately have duplicate fingerprints) and isn't needed yet.
- **A mid-refresh crash loses at most the current in-flight chunk (up to 25 files' worth), not the whole refresh** → mitigated by WAL mode and chunked commits; everything committed in earlier chunks (plus the immediate missing/reappeared commit) survives, so a crash costs re-fingerprinting one partial chunk on the next refresh, not redoing the whole pass.
- **`missing` rows accumulate forever with no prune action** → acceptable for now; if this becomes noisy in practice, a follow-up change can add an explicit "forget this file" UI action.
- **Two refresh triggers (startup + on-demand) racing at boot** → if a user clicks "Refresh" in the UI in the same instant the startup refresh kicks off, the second request simply gets `409 Conflict`, which the UI treats the same as "a refresh is already running" (start polling status) rather than as an error.

## Open Questions

- Should `processed`/`total` progress be file-count-based (simplest, decided above) or weighted by file size/duration for a more accurate progress bar? File-count is good enough for v1; revisit only if large libraries make count-based progress visibly misleading (e.g. one huge FLAC skewing perceived speed).
