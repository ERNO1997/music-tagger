## Context

`openspec/project.md` defines the full target architecture (clean-architecture Go layout, Fiber/Echo HTTP layer, pure-Go tagging, four external gateways, Docker packaging). Nothing has been built yet. This change is the first vertical slice: it stands up the skeleton and proves acoustic identification end-to-end, without touching any of the upstream identification/enrichment/tagging/relocation machinery described in `project.md` §1.3–§1.4. Everything here is additive scaffolding for two new capabilities (`audio-fingerprinting`, `music-library-scan`); no existing specs are modified.

## Goals / Non-Goals

**Goals:**
- Stand up the Go module and the `cmd/server`, `internal/domain`, `internal/usecases`, `internal/infrastructure/{web/v1,filestat}`, `ui/` directories exactly as laid out in `project.md` §3.
- Compute a real, correct Chromaprint fingerprint for every `.mp3`/`.flac` file under `/music` via `fpcalc`, never via filename or existing tags (project.md §4.1).
- Serve those results over one read-only JSON endpoint and one static dark-mode HTML page.
- Keep the seams open for later changes: `Fingerprinter` is defined as a usecase port now so the AcoustID/MusicBrainz identify step (a future change) can be inserted after it without touching this change's code.

**Non-Goals:**
- No AcoustID/MusicBrainz/Cover Art Archive/Genius network calls.
- No ID3v2/Vorbis-comment writing, no cover art or lyrics embedding.
- No `os.MkdirAll`/`os.Rename` relocation logic.
- No async task-based `/api/v1/scan-local` endpoint or `/api/v1/upload-file` streaming endpoint — those are separate future changes per `project.md`'s OpenAPI contract. This change's `GET /api/v1/library` is a new, simpler endpoint scoped to this change only (it does not appear in `project.md`'s existing OpenAPI block and does not need to, since that block covers later workflows).
- No persistence layer — each request to `/api/v1/library` re-scans and re-fingerprints; no caching or database.

## Decisions

- **HTTP framework: Go Fiber.** `project.md` §2.1 names Fiber as the default. Its native multipart handling and streaming response support aren't exercised by this change, but adopting it now avoids a router migration when the upload workflow (Workflow 2) lands later.
- **Scan is synchronous, in-request.** `project.md`'s eventual `/api/v1/scan-local` is asynchronous (background Goroutine + `task_id`), but that's because it also identifies, tags, and relocates files against a 1 req/sec-limited MusicBrainz. This change does neither — fingerprinting a library with only local `fpcalc` calls is fast enough to run synchronously within a single HTTP request. `GET /api/v1/library` is therefore a plain synchronous GET, not the async-with-task-id pattern; that pattern is deferred to the change that adds real identification.
- **`Fingerprinter` defined as a usecase port now.** `internal/usecases/ports.go` declares `Fingerprinter` (implemented by `internal/infrastructure/filestat/fpcalc_runner.go`) even though only one implementation exists today, matching the dependency-inversion rule in `project.md` §3.1 and letting the scan usecase be unit-tested against a fake in later changes.
- **fpcalc invoked with `-json`.** Parsing structured JSON from `fpcalc -json` (rather than scraping plain-text output) is more robust to `fpcalc` version differences and gives duration + fingerprint in one parse.
- **Per-file isolation of failures.** The scan usecase collects one result-or-error per file rather than aborting on the first failure, so a single corrupt file doesn't hide the fingerprints of the rest of the library — this is a hard requirement of the `music-library-scan` spec's "Per-file fingerprint failure" scenario.
- **UI is static, embedded, no build step.** Per `project.md` §1.2, `ui/` stays Vanilla JS + Tailwind (pre-built CSS, not a Tailwind CLI/PostCSS pipeline) and is served via `go:embed` from the same binary — no Node toolchain is introduced.

## Risks / Trade-offs

- **`fpcalc` must exist on the host/container `PATH`.** → Documented explicitly in the proposal's Impact section and covered by a task to install it in the dev/Docker environment; the runner returns a clear domain error (not a panic) when it's missing.
- **Synchronous full-library scan could be slow on very large libraries** (each file pays an `fpcalc` process-spawn cost). → Acceptable for this change since it's a read-only diagnostic endpoint, not the production scan workflow; if this becomes a real problem, a later change can add the same async/`task_id` pattern already specified for `/api/v1/scan-local`. Not solved here to avoid scope creep.
- **No caching means repeated page loads re-fingerprint everything.** → Acceptable trade-off for a first slice focused on correctness/visibility; revisit if it makes manual testing painful.

## Open Questions

- Should `GET /api/v1/library` live under `/api/v1/` alongside the future `scan-local`/`upload-file` endpoints (as planned here), or under a distinct `/api/v1/debug/` prefix to signal it's a diagnostic, pre-identification view? Defaulting to `/api/v1/library` for now; easy to move before it has external consumers.
