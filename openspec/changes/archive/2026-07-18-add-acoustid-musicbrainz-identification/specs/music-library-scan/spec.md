## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, identification status, and (once identified) resolved artist, album, title, and track number — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON array where each entry includes `path`, `format`, `duration_seconds`, `fingerprint`, `status`, and an `error` field populated only when that file's most recent fingerprint attempt failed

#### Scenario: Identified file includes resolved metadata
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has status `identified`
- **THEN** that file's entry SHALL include its resolved artist, album, title, and track number

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
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, identification status, and resolved metadata when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows, and provide a bulk action to identify the selected rows.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, including its status and any resolved metadata, displaying no cover art or lyrics data

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes

#### Scenario: Rows can be selected for bulk identification
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable an "Identify Selected" action that, when triggered, submits the selected files' paths for identification

#### Scenario: Identify action disabled while an identify job is running
- **WHEN** an identification job is currently running
- **THEN** the UI's identify action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

## ADDED Requirements

### Requirement: On-demand identification action
The system SHALL expose a `POST /api/v1/library/identify` endpoint accepting a list of one or more file paths, which starts a background job resolving each path's canonical metadata via AcoustID and MusicBrainz (per the `acoustid-lookup` and `musicbrainz-metadata` capabilities) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Identify job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/identify` with one or more paths while no identify job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, one at a time, without the HTTP request blocking until the job finishes

#### Scenario: Concurrent identify job rejected
- **WHEN** a client issues `POST /api/v1/library/identify` while an identify job is already running
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent identify job SHALL be started

#### Scenario: Identify job runs independently of a scan refresh
- **WHEN** a scan refresh is currently running
- **THEN** an identify job SHALL still be accepted and run concurrently, since the two do not share a concurrency guard

### Requirement: Identification progress is observable
The system SHALL expose a `GET /api/v1/library/identify/status` endpoint reporting whether an identify job is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries identify status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no identify job is running
- **WHEN** a client queries identify status while no identify job is in progress
- **THEN** the response SHALL indicate that no identify job is currently running
