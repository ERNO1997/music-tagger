## 1. Project bootstrap

- [x] 1.1 Initialize the Go module (`go mod init`) and pin Go 1.22+ in `go.mod`
- [x] 1.2 Create the directory skeleton: `cmd/server/`, `internal/domain/`, `internal/usecases/`, `internal/infrastructure/web/v1/`, `internal/infrastructure/filestat/`, `ui/`
- [x] 1.3 Add Go Fiber as the HTTP dependency
- [x] 1.4 Document (or script) local dev setup requiring `fpcalc` on `PATH`; add an `apt-get install libchromaprint-tools` step to a dev Dockerfile/Makefile

## 2. Domain layer

- [x] 2.1 Define `AudioFile` entity in `internal/domain/audiofile.go` (path, format, duration)
- [x] 2.2 Define `Fingerprint` value object in `internal/domain/fingerprint.go` (chroma string, duration seconds)
- [x] 2.3 Define domain errors in `internal/domain/errors.go` (e.g. `ErrFingerprintFailed`, `ErrUnsupportedFormat`)

## 3. Fingerprinting (audio-fingerprinting capability)

- [x] 3.1 Implement `Fingerprinter` port interface in `internal/usecases/ports.go`
- [x] 3.2 Implement `fpcalc_runner.go` in `internal/infrastructure/filestat/`: shell out to `fpcalc -json <path>` via `os/exec`, parse JSON output into a `Fingerprint`
- [x] 3.3 Handle `fpcalc` missing/non-zero-exit as a domain error, never falling back to filename/tag data
- [x] 3.4 Reject non-`.mp3`/`.flac` files before invoking `fpcalc`, reporting them as unsupported

## 4. Library scan usecase (music-library-scan capability)

- [x] 4.1 Implement `ScanLocalVolume` (read-only variant) in `internal/usecases/scan_local_volume.go`: recursively walk `/music`, filter to supported extensions, call `Fingerprinter` per file
- [x] 4.2 Ensure a single file's fingerprinting failure is captured per-entry and does not abort the overall scan
- [x] 4.3 Return an empty result (not an error) when `/music` has no supported files or does not exist

## 5. Web API

- [x] 5.1 Wire up `internal/infrastructure/web/v1/router.go` with Fiber
- [x] 5.2 Implement `GET /api/v1/library` handler in `library_handler.go`: invoke the scan usecase synchronously, serialize results (`path`, `format`, `duration_seconds`, `fingerprint`, optional `error`) as JSON
- [x] 5.3 Wire `cmd/server/main.go` composition root: load config (music volume path, port), construct the fpcalc-backed `Fingerprinter`, the scan usecase, the router, and start the server

## 6. Web UI

- [x] 6.1 Build `ui/index.html` dark-mode page with Tailwind (pre-built CSS, no build step) shell and a results table
- [x] 6.2 Implement `ui/js/app.js`: fetch `GET /api/v1/library` on page load, render one row per file (path, format, duration, fingerprint), show per-file errors inline
- [x] 6.3 Serve `ui/` static assets from the Go binary via `go:embed`

## 7. Verification

- [x] 7.1 Manually verify against a local `/music` directory containing at least one `.mp3` and one `.flac` file: correct fingerprints returned, correct table rendering
- [x] 7.2 Verify a corrupt/unsupported file in `/music` does not abort the scan and is reported per-item
- [x] 7.3 Verify no network calls are made and no files under `/music` are modified during a scan (e.g. via file mtime/checksum check before/after)
