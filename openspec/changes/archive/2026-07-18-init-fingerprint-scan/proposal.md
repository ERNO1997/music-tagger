## Why

The project currently has no code, only its architecture charter (`openspec/project.md`). Before any external API integration (AcoustID, MusicBrainz, Cover Art Archive, Genius) can be wired in, the system needs a working vertical slice that proves the two foundational primitives every later workflow depends on: (1) a bootstrapped Go project matching the clean-architecture layout, and (2) trustworthy acoustic identity for a file, computed via `fpcalc` rather than its filename. Shipping this first, with no upstream network calls, gives us a verifiable, inspectable baseline (fingerprints visible in a browser) before layering in identification, enrichment, and file relocation.

## What Changes

- Bootstrap the Go module and directory skeleton defined in `project.md` (`cmd/server`, `internal/domain`, `internal/usecases`, `internal/infrastructure/{web/v1,filestat}`, `ui/`).
- Add an `os/exec`-based `fpcalc` runner that computes a Chromaprint fingerprint + duration for a single audio file.
- Add a read-only scan usecase that walks the mounted `/music` volume, filters to `.mp3`/`.flac` files, and computes a fingerprint for each.
- Add a `GET /api/v1/library` endpoint that runs the scan synchronously and returns the discovered files with their fingerprints as JSON.
- Add a minimal dark-mode Vanilla JS/Tailwind page that calls this endpoint and renders a table of `path / format / duration / fingerprint`.
- Provide a development Dockerfile stage (or documented local setup) with `fpcalc` installed, since the runner shells out to it.

**Explicitly out of scope for this change** (deferred to later changes): AcoustID/MusicBrainz/Cover Art Archive/Genius integration, ID3/FLAC tag writing, file relocation, the async `/api/v1/scan-local` task-based endpoint, and the `/api/v1/upload-file` streaming workflow. This change only *reads* the volume and *computes* fingerprints — it writes nothing to disk and calls no external service.

## Capabilities

### New Capabilities
- `audio-fingerprinting`: Computing a Chromaprint acoustic fingerprint and duration for a given local audio file by shelling out to `fpcalc`, independent of any filename or existing tag data.
- `music-library-scan`: Recursively discovering supported audio files under the mounted `/music` volume and reporting each file's path, format, duration, and computed fingerprint through a read-only API and web listing page.

### Modified Capabilities
<!-- none: no existing specs yet -->

## Impact

- **New code**: `cmd/server/main.go`; `internal/domain/audiofile.go`, `internal/domain/fingerprint.go`, `internal/domain/errors.go`; `internal/usecases/scan_local_volume.go` (read-only variant) and its `Fingerprinter` port; `internal/infrastructure/filestat/fpcalc_runner.go`; `internal/infrastructure/web/v1/router.go` and a `library_handler.go`; `ui/index.html`, `ui/js/app.js`, `ui/css/app.css`.
- **Runtime dependency**: the `fpcalc` binary must be present on `PATH` wherever the server runs (documented in the Dockerfile/dev setup, per `project.md` §2.4).
- **No external network calls, no writes**: no API keys required yet; `/music` is treated as read-only for this change's purposes (no `os.Rename`/`os.MkdirAll` performed).
- **Sets up, but does not implement**, the ports later changes will fill in (`MetadataGateway`, `Tagger`, `Relocator` remain unwritten stubs/interfaces for now).
