## MODIFIED Requirements

### Requirement: Recursive discovery of audio files in the mounted volume
The system SHALL recursively walk the configured `/music` directory and identify all files with `.mp3`, `.flac`, or `.m4a` extensions as candidate audio files, at any subdirectory depth.

#### Scenario: Nested directories are included
- **WHEN** `/music` contains audio files nested at arbitrary subdirectory depths
- **THEN** all matching files SHALL be included in the scan result regardless of depth

#### Scenario: Non-audio files are ignored
- **WHEN** `/music` contains files with extensions other than `.mp3`/`.flac`/`.m4a`
- **THEN** those files SHALL be excluded from the scan result and SHALL NOT be passed to the fingerprinting component

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, and identification status — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON array where each entry includes `path`, `format`, `duration_seconds`, `fingerprint`, `status`, and an `error` field populated only when that file's most recent fingerprint attempt failed

#### Scenario: Read reflects an in-progress refresh
- **WHEN** a background refresh is currently running and has already committed some but not all files
- **THEN** `GET /api/v1/library` SHALL return the tracked state as committed so far, without waiting for the refresh to finish

#### Scenario: Empty or never-refreshed store
- **WHEN** no refresh has ever run, or the tracking store contains no records
- **THEN** the endpoint SHALL return `200 OK` with an empty array rather than an error

#### Scenario: No external calls or filesystem writes occur
- **WHEN** `GET /api/v1/library` is called
- **THEN** the system SHALL NOT call AcoustID, MusicBrainz, Cover Art Archive, or Genius, and SHALL NOT call `os.MkdirAll` or `os.Rename` on any file under `/music`, and SHALL NOT perform a disk walk or invoke `fpcalc`

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, and identification status, and SHALL reflect whether a refresh is currently running.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, including its status, displaying no cover art, tag, or lyrics data

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes

## ADDED Requirements

### Requirement: Asynchronous refresh action
The system SHALL expose a `POST /api/v1/library/scan` endpoint that starts the disk walk, fingerprinting, and tracking-store update (per the `file-tracking-store` capability) in the background and returns immediately, rather than blocking for the duration of the refresh.

#### Scenario: Refresh accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/scan` while no refresh is running
- **THEN** the response SHALL be `202 Accepted` and the walk/fingerprint/update SHALL proceed asynchronously, without the HTTP request blocking until it finishes

#### Scenario: Per-file fingerprint failure does not abort the refresh
- **WHEN** one file in `/music` fails fingerprinting during a refresh (e.g. a corrupt audio file)
- **THEN** that file SHALL be reported with an error indicator in its tracked record and the refresh SHALL continue processing the remaining files

### Requirement: Concurrent refresh prevention
The system SHALL allow at most one refresh to run at a time.

#### Scenario: Refresh requested while one is already running
- **WHEN** a client issues `POST /api/v1/library/scan` while a refresh is already in progress
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent refresh SHALL be started

### Requirement: Refresh triggered automatically at server startup
The system SHALL start one refresh automatically when the server starts, without requiring a manual trigger.

#### Scenario: Server starts with an unpopulated or stale store
- **WHEN** the server process starts
- **THEN** it SHALL begin a background refresh immediately, and SHALL accept HTTP requests (including `GET /api/v1/library`) without waiting for that refresh to complete

### Requirement: Refresh progress is observable
The system SHALL expose a way for clients to determine whether a refresh is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries refresh status while a refresh is in progress
- **THEN** the response SHALL indicate that a refresh is running and SHALL include how many of the files needing fingerprinting have been processed so far

#### Scenario: Idle status when no refresh is running
- **WHEN** a client queries refresh status while no refresh is in progress
- **THEN** the response SHALL indicate that no refresh is currently running
