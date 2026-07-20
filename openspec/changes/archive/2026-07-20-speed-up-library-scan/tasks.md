## 1. Domain and ports

- [x] 1.1 Add a `DurationReader` interface (`ReadDuration(ctx context.Context, path string) (time.Duration, error)`) to `internal/usecases/ports.go`
- [x] 1.2 Add `RecordFingerprint(ctx context.Context, path string, fingerprint string, durationSeconds float64, fingerprintErr string) error` to the `TrackingStore` interface in `internal/usecases/ports.go`

## 2. Infrastructure

- [x] 2.1 Implement a `DurationReader` in `internal/infrastructure/filestat/` backed by `taglib.ReadProperties(...).Length`, routed through the existing `withCorrectExtension` helper so a mislabeled-extension file's duration is read against its real, content-sniffed format

## 3. Usecases

- [x] 3.1 Update `ScanLocalVolume` (`internal/usecases/scan_local_volume.go`) to take a `DurationReader` instead of a `Fingerprinter`; in the per-file walk loop, replace the fingerprint call with a duration read — on success set `DurationSeconds` (leave `Fingerprint` blank), on failure set `FingerprintError` (reused field, now meaning "most recent duration-read or fingerprint failure") and leave `DurationSeconds` at its zero value
- [x] 3.2 Confirm (read-only check, no code change expected) that `BulkApply`'s existing `ON CONFLICT ... SET fingerprint = excluded.fingerprint` clause already clears a changed file's stale fingerprint to blank, since `ScanLocalVolume` now always upserts an empty `Fingerprint` — if this doesn't hold for some code path, fix it so it does
- [x] 3.3 Add a `fingerprinter Fingerprinter` field to `IdentifyFile` and its constructor; change `Identify`'s signature from `Identify(ctx, path, fingerprint string, durationSeconds float64) error` to `Identify(ctx context.Context, path string) (skipped bool, err error)`, self-loading the tracked record via `store.Get` (mirroring `TagFile.Tag`'s pattern) and returning `skipped=true` for an unknown path
- [x] 3.4 Inside the updated `Identify`, if the loaded record's `Fingerprint` is empty, call `Fingerprinter.Fingerprint(ctx, path)`; on success, call the new `store.RecordFingerprint` with the computed fingerprint/duration before proceeding to the existing AcoustID/MusicBrainz flow; on failure, call `store.RecordFingerprint` with the failure reason and return `skipped=true` without calling AcoustID
- [x] 3.5 If the loaded record's `Fingerprint` is already non-empty, use it directly (and its stored `DurationSeconds`) without recomputing
- [x] 3.6 Update `IdentifyManager.Start` (`internal/usecases/identify_manager.go`): drop its own `rec.Fingerprint == ""` pre-check (that branch moves inside `Identify`), call `m.identify.Identify(ctx, path)` for every path found in the loaded records, and log when it returns `skipped=true`

## 4. Persistence

- [x] 4.1 Implement `SQLiteStore.RecordFingerprint` in `internal/infrastructure/persistence/sqlite_store.go`: a single `UPDATE files SET fingerprint = ?, duration_seconds = ?, fingerprint_error = ?, updated_at = ? WHERE path = ?`, mirroring `RecordTagged`'s dual-outcome shape

## 5. API

- [x] 5.1 Confirm `FingerprintHandler.Get` (`internal/infrastructure/web/v1/fingerprint_handler.go`) already returns `200` with an empty `fingerprint` string for a tracked file with no fingerprint yet (since `store.Get` returns `found=true` regardless of whether `Fingerprint` is empty) and `404` only when `found=false` — no code change expected, but verify directly rather than assuming

## 6. Composition root

- [x] 6.1 Update `cmd/server/main.go`: construct the new `DurationReader` and pass it to `usecases.NewScanLocalVolume` in place of the `Fingerprinter`; pass the existing `Fingerprinter` (fpcalc runner) into `usecases.NewIdentifyFile`'s constructor instead

## 7. Verification

- [x] 7.1 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 7.2 Seed or scan a test library and confirm duration is still populated correctly for new files, without `fpcalc` being invoked during scan
- [x] 7.3 Confirm a file with no stored fingerprint gets one computed the first time Identify runs on it, and that a second Identify call on the same, unchanged file does not recompute it
- [x] 7.4 Confirm a changed file (modified on disk, re-scanned) has its previous fingerprint cleared and gets a freshly computed one on its next Identify
- [x] 7.5 Confirm a fingerprint-computation failure during an identify job doesn't abort the batch — other paths in the same job still get processed
- [x] 7.6 Confirm `GET /api/v1/library/fingerprint` returns `200` with an empty string for a tracked-but-unfingerprinted file, and `404` for an untracked path
- [x] 7.7 Rebuild and run via `docker compose up --build` against the user's real ~2,800-file music library as a final sanity check: confirm a full scan now completes in well under a minute (versus the previous ~10 minutes), and that Identify still correctly resolves files as before
