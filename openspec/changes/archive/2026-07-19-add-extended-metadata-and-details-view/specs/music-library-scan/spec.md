## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, identification status, and (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON array where each entry includes `path`, `format`, `duration_seconds`, `fingerprint`, `status`, and an `error` field populated only when that file's most recent fingerprint attempt failed

#### Scenario: Identified file includes resolved metadata
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has status `identified`
- **THEN** that file's entry SHALL include its resolved artist, album artist, title, track number, release year (when available), disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs

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
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, identification status, and a condensed resolved-metadata summary when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows, provide a bulk action to identify the selected rows, and allow opening a full details view for any single row.

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

### Requirement: Per-file details view
The system SHALL allow a user to open a details view for a single tracked file, showing its complete resolved record — path, format, duration, fingerprint, status, any fingerprint error, and (once identified) artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs. Opening this view SHALL NOT require any request beyond the already-fetched `GET /api/v1/library` data.

#### Scenario: Opening details for a row
- **WHEN** a user clicks a row (other than its selection checkbox)
- **THEN** the UI SHALL display that file's complete resolved record without issuing any new network request

#### Scenario: Selecting a row does not open its details
- **WHEN** a user clicks a row's selection checkbox
- **THEN** the UI SHALL toggle that row's selection state and SHALL NOT open the details view

#### Scenario: Details view for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `missing`
- **THEN** the UI SHALL show the fields available for that status (path, format, duration, fingerprint, status, and any fingerprint error) without fabricating placeholder metadata values
