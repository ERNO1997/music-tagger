## 1. Shared background-job concurrency guard

- [x] 1.1 Extract a generic `usecases.JobManager` (running/processed/total/startedAt, `Start(work func(report func(processed, total int))) error`, `Status() JobStatus`) from `RefreshManager`'s existing guard logic
- [x] 1.2 Refactor `RefreshManager` to compose `JobManager` instead of its own duplicated mutex/state — no public API or behavior change
- [x] 1.3 Re-run the existing scan-refresh Docker verification scenarios (startup auto-refresh, 409 on concurrent trigger, progress polling) to confirm no regression from the refactor

## 2. Configuration

- [x] 2.1 Add `ACOUSTID_API_KEY` and `MUSICBRAINZ_USER_AGENT` environment variables to configuration loading in `cmd/server/main.go`
- [x] 2.2 Fail identification requests with a clear error (not a silent no-op or unauthenticated/non-compliant request) when either is unset — checked upfront in `IdentifyHandler.Trigger` (immediate `400`) in addition to the per-client guards in `IdentifyFile`
- [x] 2.3 Add both variables to `docker-compose.yml`/`.env.example`

## 3. AcoustID gateway client

- [x] 3.1 Define an `AcoustIDLookup` port in `internal/usecases/ports.go`
- [x] 3.2 Implement `internal/infrastructure/gateways/acoustid_client.go`: GET `api.acoustid.org/v2/lookup` with `client`, `fingerprint`, `duration`, `meta=recordings` (releasegroups data isn't needed — release/track-number comes entirely from the separate MusicBrainz recording lookup)
- [x] 3.3 Parse the response into a ranked list of MusicBrainz Recording IDs; return an empty (not error) result when AcoustID reports no matches
- [x] 3.4 Return a distinguishable error for network/HTTP/malformed-response failures, never conflated with "no match"

## 4. MusicBrainz gateway client

- [x] 4.1 Define a `MusicBrainzLookup` port in `internal/usecases/ports.go`
- [x] 4.2 Implement `internal/infrastructure/gateways/musicbrainz_client.go`: GET `/ws/2/recording/{mbid}?inc=releases+media+release-groups+artist-credits` with the configured `User-Agent` — verified against the real MusicBrainz API during implementation (real recording MBID, confirmed JSON field names/nesting)
- [x] 4.3 Implement the centralized rate gate: a shared minimum-1-second-interval mechanism inside the client, applied to every call regardless of caller — found and fixed a real bug in the first implementation (see below), then verified ~984ms spacing against the live API
- [x] 4.4 Implement the release-selection heuristic: prefer release-group primary type "Album" + status "Official", else the first release with track data
- [x] 4.5 Parse the selected release into artist (joined artist-credit + joinphrase), album title, track title, track number
- [x] 4.6 Return a distinguishable error for network/HTTP/malformed-response failures, never conflated with "no releases" — verified with a bad recording ID against the live API (HTTP 400 surfaced as an error, not `ErrNoMusicBrainzRelease`)

## 5. Domain and persistence: resolved metadata

- [x] 5.1 Extend `domain.FileRecord` with `Artist`, `Album`, `Title`, `TrackNumber`, `RecordingMBID` fields
- [x] 5.2 Extend the SQLite schema with corresponding columns — found during Docker verification that `CREATE TABLE IF NOT EXISTS` alone is a no-op against a database created by a prior version (a real pre-existing volume hit "no such column: artist"), so added an idempotent `PRAGMA table_info` + `ALTER TABLE ADD COLUMN` migration step that runs on every startup regardless of which prior schema version a database started from
- [x] 5.3 Update `LoadAll` to read the new columns
- [x] 5.4 Add a `TrackingStore.RecordIdentification` method: updates one file's status + resolved metadata (identified) or status only (not_found) in a single per-file commit — no chunking needed here (see design.md)

## 6. Identify usecase and background job

- [x] 6.1 Implement an identify-one-file usecase: load the file's fingerprint/duration from the tracking store, call `AcoustIDLookup`, then `MusicBrainzLookup` on the top match, and call `RecordIdentification` with the resulting outcome (identified / not_found / error)
- [x] 6.2 Implement `IdentifyManager` composing `JobManager`: `Start(paths []string) error` processes the list sequentially, updating progress per file
- [x] 6.3 Confirm `IdentifyManager` and `RefreshManager` use independent guards (both can run concurrently; each individually rejects a second concurrent trigger of the same kind)

## 7. API endpoints

- [x] 7.1 Add `POST /api/v1/library/identify`: parses `{"paths": [...]}`, calls `IdentifyManager.Start`, returns `202 Accepted` or `409 Conflict`
- [x] 7.2 Add `GET /api/v1/library/identify/status`: returns running/processed/total
- [x] 7.3 Update the `LibraryEntry` JSON shape to include `artist`/`album`/`title`/`track_number` (omitted when not yet identified)

## 8. Web UI

- [x] 8.1 Add row selection checkboxes and an "Identify Selected" button
- [x] 8.2 Wire the button to `POST /api/v1/library/identify` with the selected paths
- [x] 8.3 Poll `GET /api/v1/library/identify/status` (and re-fetch `GET /api/v1/library`) while a job is running; disable the identify action and show progress; re-enable on completion
- [x] 8.4 Display resolved artist/album/title/track number columns when present on a row

## 9. Verification

- [x] 9.1 **Prerequisite**: obtain a real AcoustID API key (free registration) and choose a MusicBrainz-compliant `User-Agent` string — user registered an AcoustID application and provided real credentials via `.env`
- [x] 9.2 Verify (via Docker) the happy path: a real, identifiable file resolves to correct artist/album/title/track number and status `identified` — verified against the user's real `docker compose` stack with 3 real files (1 mp3, 2 m4a); all 3 correctly identified (e.g. "Taylor Swift — The Fate of Ophelia — The Life of a Showgirl — track 1")
- [x] 9.3 Verify a synthetic/unrecognizable file resolves to status `not_found` with no metadata written — verified in a disposable scratch volume (kept separate from the user's real data) using the same real credentials
- [x] 9.4 Verify a bulk identify request (2+ paths) processes them at the enforced ≥1 req/sec pace (observable via timing between MusicBrainz calls) — confirmed both in isolation (precise ~984ms/~999ms waits against the live API) and integrated (3-file job progressed 0→1→2→3 over ~2 wall-clock seconds, not instantaneously)
- [x] 9.5 Verify a second `POST /api/v1/library/identify` while one is running returns `409 Conflict`
- [x] 9.6 Verify an identify job and a scan refresh can run concurrently without error
- [x] 9.7 Verify missing `ACOUSTID_API_KEY`/`MUSICBRAINZ_USER_AGENT` produces a clear error rather than an unauthenticated/non-compliant request
- [x] 9.8 Verify `GET /api/v1/library`'s "no external calls" invariant still holds (no AcoustID/MusicBrainz traffic from a plain read)
