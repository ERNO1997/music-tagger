# Music Tagger — Project Specification

| | |
|---|---|
| **Document Type** | OpenSpec Project Charter |
| **Status** | Active |
| **Owner** | Principal Architecture |
| **Scope** | Full-system technical definition for Spec-Driven Development (SDD) |

---

## 1. Executive Summary & Workflows

### 1.1 Mission Statement

**Music Tagger** is a self-contained, Dockerized automation utility for identifying, tagging, and organizing digital audio libraries. It replaces unreliable, filename-based heuristics with acoustic-fingerprint identification, sourcing authoritative metadata, cover artwork, and lyrics from upstream catalog services. The system exposes two distinct operational modes through a single Go binary: a **batch/background workflow** for whole-library remediation, and a **synchronous workflow** for one-off, interactive file corrections.

### 1.2 System Composition

- **Backend**: Go 1.22+ web application compiled to a static binary, exposing a versioned REST API.
- **Frontend**: Lightweight, dark-mode **Vanilla JS + Tailwind CSS** single-page interface — no frontend build toolchain (no Node/React/webpack). Served directly as static assets by the Go binary.
- **Runtime**: Single Docker container bundling the Go binary alongside the native `fpcalc` and `ffmpeg` executables required for fingerprinting and audio inspection.

### 1.3 Workflow 1 — Bulk Local Automation & Tracking

Triggered via the web UI or API against a Docker-mounted `/music` volume. This workflow is split into two independently-triggerable phases so that MusicBrainz's rate limit (§4.2) is respected by construction rather than by careful pacing of a large batch job.

**Phase A — Discovery & Tracking (automatic on scan)**

1. **Scan**: Recursively walk the `/music` directory for supported audio files (`.mp3`, `.flac`, `.m4a`).
2. **Fingerprint**: For each file, invoke `fpcalc` to compute a Chromaprint acoustic fingerprint — never inferred from filename or existing (potentially incorrect) tags.
3. **Track**: Persist each file's path, fingerprint, and identification status (`new`, `identified`, `not_found`, `missing`) to a local embedded database (§2.4), so re-scanning only touches files that changed and previously-resolved files are never silently reprocessed.

**Phase B — Identification (on-demand, per file)**

4. **Identify**: Triggered individually by the user from the web UI — not automatically as part of the scan. Submits the tracked file's fingerprint + duration to **AcoustID**, resolves the returned MusicBrainz Recording ID, then queries **MusicBrainz** for canonical artist/album/track/track-number data. The result updates that file's row in the tracking store.

**Phase C — Enrichment, Tagging & Relocation (future)**

5. **Enrich**: Query **Cover Art Archive** for high-resolution artwork and **Genius** for English lyrics, once a file is identified.
6. **Tag**: Write ID3v2 (MP3), Vorbis Comment (FLAC), or MP4/iTunes-style atom (M4A) metadata, embed cover art (`APIC` / `PICTURE` block / `covr` atom), and embed lyrics (`USLT` / `LYRICS` comment / `©lyr` atom).
7. **Relocate**: Physically move the tagged file into the canonical `Artist/Album/Track - Title` hierarchy on the same volume.

Phase A runs as a background Goroutine — triggered automatically once at server startup and on demand from the UI — rather than inline with the HTTP request, so a large library's discovery/fingerprint/track pass never holds a connection open. Its trigger endpoint returns immediately and reports progress separately, distinct from (and simpler than) the `task_id` registry §5's `POST /api/v1/scan-local` contract reserves for the eventual full bulk-remediation pipeline. This is orthogonal to Phase B, which stays on-demand and human-paced regardless.

### 1.4 Workflow 2 — Real-time Upload (Synchronous)

Triggered via drag-and-drop on the web UI.

1. **Upload**: Client posts a single audio file as `multipart/form-data` to the API.
2. **Stage**: The server buffers the file to a temp path (or in-memory buffer for small files).
3. **Process**: The identical fingerprint → identify → enrich → tag pipeline from Workflow 1 executes synchronously, in-request.
4. **Stream Back**: The server streams the corrected binary directly back in the HTTP response body with the appropriate audio `Content-Type`, triggering a native browser download. No file is persisted to the `/music` volume in this workflow.

---

## 2. Technical Stack Definitions

### 2.1 Core Language & Framework

- **Go 1.22+** — chosen for static-binary distribution, first-class concurrency primitives (Goroutines/channels for the rate-limited scan pipeline), and zero-runtime-dependency deployment.
- **HTTP Framework**: **Go Fiber** (Express-inspired, `fasthttp`-based router) for high-throughput routing, built-in multipart handling, and native streaming response support. Echo is an acceptable substitute if idiomatic `net/http` compatibility is preferred; Fiber is the default recommendation for this project.

### 2.2 Audio Engineering & Tagging

All tagging is performed with **pure Go libraries** — CGO is disabled (`CGO_ENABLED=0`) for the entire build to guarantee static, portable binaries and to simplify the multi-stage Docker build.

| Concern | Approach |
|---|---|
| Acoustic Fingerprinting | Native orchestration of the system binary `fpcalc` (Chromaprint) via `os/exec`. Go never re-implements fingerprinting logic; it shells out and parses stdout (JSON mode: `fpcalc -json`). `fpcalc` decodes via `ffmpeg`'s libraries, so `.mp3`, `.flac`, and `.m4a` are all supported without additional fingerprinting-side work. |
| MP3 Tagging | Pure-Go ID3v2 library for text frames (`TIT2`, `TPE1`, `TALB`, `TRCK`, etc.), `USLT` (unsynchronized lyrics) frames, and `APIC` (attached picture) frames. |
| FLAC Tagging | Pure-Go FLAC metadata library for Vorbis comment blocks (`TITLE`, `ARTIST`, `ALBUM`, `TRACKNUMBER`, `LYRICS`) and `PICTURE` metadata blocks. |
| M4A/MP4 Tagging | Pure-Go MP4 atom library for iTunes-style metadata atoms (`©nam`, `©ART`, `©alb`, `trkn`), the `covr` atom for cover art, and the `©lyr` atom for lyrics. |

### 2.3 External API Gateways

| Service | Purpose | Constraint |
|---|---|---|
| **AcoustID** | Resolves a Chromaprint fingerprint + duration to a MusicBrainz Recording ID | API-key gated |
| **MusicBrainz** | Canonical catalog data: artist, release (album), recording (track title), track position | **Hard 1 req/sec limit** — see §4.2 |
| **Cover Art Archive** | High-resolution front-cover artwork keyed by MusicBrainz Release ID | Public, unauthenticated |
| **Genius** | English-language lyrics search and scraping/fetch | API-key gated |

### 2.4 Persistence Layer

- **Embedded database**: **SQLite**, accessed via a pure-Go driver (e.g. `modernc.org/sqlite`) to preserve the `CGO_ENABLED=0` static-binary constraint (§2.2).
- **Purpose**: Track every file discovered under the mounted `/music` volume — path, fingerprint, size/mtime, and identification status (`new`, `identified`, `not_found`, `missing`) — so the web UI can distinguish untouched files from previously-identified ones, and identification (§1.3 Phase B) can be triggered on demand per file rather than automatically across an entire scan.
- **Location**: A single SQLite file stored on its own Docker volume (not the `/music` mount itself), so tracking state survives container restarts independently of the music library.
- **Scope**: This store tracks *identification/tagging status*, not a full mirror of canonical metadata — resolved artist/album/track data is sourced fresh from MusicBrainz on each identification call, though caching resolved metadata alongside the status row is a reasonable future optimization to be scoped in the change that introduces this store.

### 2.5 Containerization

- **Multi-stage Dockerfile**:
  - **Stage 1 (builder)**: `golang:1.22-bookworm` — compiles the application via `CGO_ENABLED=0 GOOS=linux go build` into a single static binary.
  - **Stage 2 (runtime)**: `debian:bookworm-slim` — minimal base image with `fpcalc` (via `libchromaprint-tools`) and `ffmpeg` pre-installed via `apt-get`, plus the compiled Go binary and `/ui` static assets copied in.
- The `/music` directory is a **user-provided bind mount**, never baked into the image.
- The container exposes a single HTTP port (default `8080`).

---

## 3. Go Clean Architecture Layout

The codebase strictly follows dependency-inversion boundaries: `domain` depends on nothing; `usecases` depends only on `domain`; `infrastructure` implements interfaces defined by `domain`/`usecases` and depends inward, never the reverse.

```
music-tagger/
├── cmd/
│   └── server/
│       └── main.go                  # Composition root: wiring, config load, server bootstrap
│
├── internal/
│   ├── domain/                      # Enterprise business rules — zero external imports
│   │   ├── audiofile.go             #   AudioFile entity (path, format, duration, checksum)
│   │   ├── tagmetadata.go           #   TagMetadata value object (artist, album, title, track#)
│   │   ├── lyrics.go                #   LyricsResult value object
│   │   ├── fingerprint.go           #   Fingerprint value object (chroma string, duration)
│   │   └── errors.go                #   Domain-level sentinel/typed errors
│   │
│   ├── usecases/                    # Application business rules — orchestrators
│   │   ├── scan_local_volume.go     #   ScanLocalVolume: walk, fingerprint, persist tracking rows
│   │   ├── identify_file.go         #   IdentifyFile: on-demand AcoustID/MusicBrainz lookup for one file
│   │   ├── process_web_upload.go    #   ProcessWebUpload: in-memory pipeline, returns tagged bytes
│   │   ├── ports.go                 #   Interfaces: Fingerprinter, MetadataGateway, Tagger, Relocator, TrackingStore
│   │   └── taskmanager.go           #   In-memory task_id registry + status tracking
│   │
│   ├── infrastructure/               # Frameworks & drivers — implements usecases ports
│   │   ├── web/
│   │   │   └── v1/
│   │   │       ├── router.go        #   Fiber/Echo route registration
│   │   │       ├── scan_handler.go  #   POST /api/v1/scan-local controller
│   │   │       ├── identify_handler.go#  On-demand per-file identification controller
│   │   │       ├── upload_handler.go#   POST /api/v1/upload-file controller
│   │   │       ├── dto.go           #   Request/response DTOs (OpenAPI-aligned)
│   │   │       └── openapi.yaml     #   Embedded OpenAPI 3.0.3 contract (see §5)
│   │   │
│   │   ├── gateways/
│   │   │   ├── acoustid_client.go   #   AcoustID HTTP client
│   │   │   ├── musicbrainz_client.go#   MusicBrainz HTTP client (centralized rate-limit gate, §4.2)
│   │   │   ├── coverart_client.go   #   Cover Art Archive HTTP client
│   │   │   └── genius_client.go     #   Genius HTTP client / lyrics scraper
│   │   │
│   │   ├── filestat/
│   │   │   ├── fpcalc_runner.go     #   os/exec wrapper invoking `fpcalc -json`
│   │   │   ├── id3_tagger.go        #   Pure-Go MP3 ID3v2/USLT/APIC read-write wrapper
│   │   │   ├── flac_tagger.go       #   Pure-Go FLAC Vorbis-comment/PICTURE read-write wrapper
│   │   │   ├── mp4_tagger.go        #   Pure-Go MP4 atom read-write wrapper (M4A: ©nam/©ART/©alb/trkn/covr/©lyr)
│   │   │   └── path_sanitizer.go    #   Filesystem-safe path construction (§4.3)
│   │   │
│   │   └── persistence/
│   │       └── sqlite_store.go      #   SQLite-backed TrackingStore (§2.4): file path, fingerprint, status
│   │
│   └── config/
│       └── config.go                 # Environment-driven configuration (API keys, paths, port)
│
├── ui/                                # Frontend static assets (embedded via go:embed)
│   ├── index.html
│   ├── css/
│   │   └── app.css                   # Tailwind (dark-mode) compiled output
│   └── js/
│       └── app.js                    # Vanilla JS: drag-and-drop, fetch, task polling
│
├── Dockerfile
├── go.mod
└── openspec/
    └── project.md                    # This document
```

### 3.1 Dependency Rule Enforcement

- `domain` types (`AudioFile`, `TagMetadata`, `LyricsResult`, `Fingerprint`) contain **no** references to Fiber, `os/exec`, or any HTTP client.
- `usecases` define **ports** (Go interfaces) such as `Fingerprinter`, `MetadataGateway`, `Tagger`; concrete implementations live exclusively in `infrastructure` and are injected at composition time in `cmd/server/main.go`.
- Controllers in `infrastructure/web/v1/` translate HTTP requests into usecase invocations and usecase results back into HTTP responses — they contain no business logic.

---

## 4. Core Invariants & System Rules

These rules are non-negotiable architectural constraints. Any implementation or change proposal that violates them must be rejected at review.

### 4.1 Acoustic-First Identification Rule

> The system **must never** rely on filenames, embedded (pre-existing) tags, or directory structure to identify a track's canonical metadata.

Every file entering either workflow **must** have its physical acoustic fingerprint computed via `fpcalc` before any upstream identification call is made. Existing filenames and tags are treated as untrusted, potentially corrupt input — the fingerprint is the sole source of truth for identity resolution.

### 4.2 MusicBrainz Rate Limiting Rule

> MusicBrainz enforces a strict **1 request/second** ceiling per client. Violation risks IP-level banning.

- The limit is enforced **once, centrally, inside the MusicBrainz gateway** (`internal/infrastructure/gateways/musicbrainz_client.go`) via a shared minimum-interval gate keyed on the timestamp of the last request — every caller is paced identically regardless of who's asking. This rule is enforced by the gateway itself, not reimplemented by each caller.
- **On-demand, per-file identification** (§1.3 Phase B, triggered from the web UI) issues one request at a time by construction; the gateway-level gate is the hard backstop against rapid repeated clicks or any future bulk-trigger UI, not the primary pacing mechanism.
- Should a future bulk/background scan mode be introduced (see §5's reserved `POST /api/v1/scan-local` contract), it must call MusicBrainz serially through the same gateway:
  ```go
  for _, file := range files {
      result, err := mbClient.Lookup(ctx, file.Fingerprint) // rate gate enforced inside mbClient
      // ... handle result ...
  }
  ```
- Concurrent/fan-out fetching against MusicBrainz is explicitly forbidden — regardless of caller, requests are serialized at the gateway boundary.
- **Workflow 2 (single upload)** goes through the same gateway and is therefore automatically covered by the same gate.

### 4.3 File Relocation & Path Sanitization Rule

> Restructured paths must strictly conform to: `/music/{artist}/{album}/{track} - {title}.ext`

- Path segments (`{artist}`, `{album}`, `{title}`) are derived from upstream metadata and **must** be sanitized via native Go string manipulation before use in any filesystem call. The sanitizer strips or replaces OS-prohibited characters:
  ```
  \  /  :  *  ?  "  <  >  |
  ```
- The sanitization step executes **before** `os.MkdirAll` (directory creation) and **before** `os.Rename` (file relocation) — never after.
- Track numbers are zero-padded (e.g., `07`) for correct lexicographic sort order within an album directory.
- Relocation is atomic per-file: `os.MkdirAll` provisions the destination tree, followed by `os.Rename` to move the tagged file; on any error, the source file is left untouched and the failure is recorded against the task's status rather than partially applied.

---

## 5. OpenAPI Contract

The following OpenAPI 3.0.3 contract is the authoritative interface definition for `internal/infrastructure/web/v1/`. Controllers are validated against this schema; any endpoint change must update this block first (contract-first / spec-driven development).

```yaml
openapi: 3.0.3
info:
  title: Music Tagger API
  description: >
    REST API for the Music Tagger automation utility. Exposes bulk local-volume
    remediation (asynchronous) and single-file interactive correction (synchronous).
  version: 1.0.0
  contact:
    name: Music Tagger Engineering

servers:
  - url: /api/v1
    description: Base path for all v1 endpoints

tags:
  - name: Local Automation
    description: Asynchronous bulk processing of the mounted /music volume
  - name: Interactive Upload
    description: Synchronous single-file processing and streaming download

paths:
  /scan-local:
    post:
      tags:
        - Local Automation
      summary: Trigger an asynchronous scan and remediation of the mounted /music volume
      description: >
        Initiates a background Goroutine that recursively walks the /music volume,
        fingerprints each audio file via fpcalc, resolves canonical metadata through
        AcoustID/MusicBrainz (rate-limited to 1 req/sec), embeds cover art and lyrics,
        and relocates each file into the Artist/Album/Track - Title hierarchy.
        Returns immediately with a task identifier for status polling.
      operationId: scanLocalVolume
      requestBody:
        required: false
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ScanLocalRequest'
      responses:
        '202':
          description: Scan accepted and scheduled for background processing
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ScanLocalAcceptedResponse'
        '409':
          description: A scan task is already in progress
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal server error scheduling the task
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

  /upload-file:
    post:
      tags:
        - Interactive Upload
      summary: Upload a single audio file for synchronous tagging and streamed download
      description: >
        Accepts a single MP3, FLAC, or M4A file via multipart/form-data. The file is
        fingerprinted, identified, enriched with metadata/cover art/lyrics, and
        tagged in-place in a temp buffer. The corrected binary is streamed back
        in the response body with the matching audio Content-Type. No file is
        persisted to the /music volume as part of this workflow.
      operationId: processWebUpload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              required:
                - file
              properties:
                file:
                  type: string
                  format: binary
                  description: A single .mp3, .flac, or .m4a audio file
      responses:
        '200':
          description: Successfully tagged audio file streamed back to the client
          content:
            audio/mpeg:
              schema:
                type: string
                format: binary
            audio/flac:
              schema:
                type: string
                format: binary
            audio/mp4:
              schema:
                type: string
                format: binary
        '400':
          description: Missing file, unsupported format, or malformed multipart payload
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '422':
          description: Fingerprint could not be identified against AcoustID/MusicBrainz
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: Internal processing error (fingerprinting, tagging, or upstream gateway failure)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

components:
  schemas:
    ScanLocalRequest:
      type: object
      properties:
        dry_run:
          type: boolean
          default: false
          description: >
            When true, executes the full identify/enrich pipeline and logs the
            resulting metadata and target paths without writing tags or moving
            any files on disk.
      example:
        dry_run: false

    ScanLocalAcceptedResponse:
      type: object
      required:
        - status
        - task_id
      properties:
        status:
          type: string
          enum: [accepted]
          description: Confirms the scan has been scheduled
        task_id:
          type: string
          format: uuid
          description: Identifier used to poll scan progress/status
      example:
        status: accepted
        task_id: 3fa85f64-5717-4562-b3fc-2c963f66afa6

    ErrorResponse:
      type: object
      required:
        - error
      properties:
        error:
          type: string
          description: Human-readable error message
        code:
          type: string
          description: Machine-readable error code
      example:
        error: "unsupported audio format: .wav"
        code: "UNSUPPORTED_FORMAT"

  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key

security: []
```

---

## 6. Non-Goals

To keep the system's scope well-bounded, the following are explicitly **out of scope** for the initial architecture:

- Multi-user authentication/authorization (single-operator, trusted-network deployment assumed).
- Non-English lyrics sourcing (Genius integration is English-only by design).
- Audio transcoding or format conversion (`ffmpeg` is bundled solely for auxiliary audio inspection, not format conversion, in v1).
- Persistent storage of background *task run* history — the `task_id`/status registry (§3, `taskmanager.go`) remains in-memory and scoped to the process lifetime. This is distinct from the SQLite-backed file-tracking store (§2.4), which persists file identification state across restarts by design.
