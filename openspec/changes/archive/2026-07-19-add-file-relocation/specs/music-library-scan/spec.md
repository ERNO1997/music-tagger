## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, a tagged indicator, and a relocated indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. A file's reported `path` SHALL always be its current, possibly-relocated location. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON array where each entry includes `path`, `format`, `duration_seconds`, `fingerprint`, `status`, and an `error` field populated only when that file's most recent fingerprint attempt failed

#### Scenario: Identified file includes resolved metadata
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has status `identified`
- **THEN** that file's entry SHALL include its resolved artist, album artist, title, track number, release year (when available), disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs

#### Scenario: Enriched file includes a cover art indicator
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has a stored cover art path
- **THEN** that file's entry SHALL include a way to retrieve its cover art image

#### Scenario: Enriched file includes a lyrics indicator
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has stored lyrics
- **THEN** that file's entry SHALL include a `has_lyrics` indicator, without including the lyrics text itself

#### Scenario: Tagged file includes a tagged indicator
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has been successfully tagged
- **THEN** that file's entry SHALL include a `tagged` indicator; if tagging was attempted and failed, the entry SHALL indicate the failure instead

#### Scenario: Relocated file includes a relocated indicator and its current path
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has been successfully relocated
- **THEN** that file's entry SHALL include a `relocated` indicator and its `path` SHALL reflect the file's new, post-relocation location; if relocation was attempted and failed, the entry SHALL indicate the failure instead, and `path` SHALL remain the file's original (unmoved) location

#### Scenario: Read reflects an in-progress refresh
- **WHEN** a background refresh is currently running and has already committed some but not all files
- **THEN** `GET /api/v1/library` SHALL return the tracked state as committed so far, without waiting for the refresh to finish

#### Scenario: Empty or never-refreshed store
- **WHEN** no refresh has ever run, or the tracking store contains no records
- **THEN** the endpoint SHALL return `200 OK` with an empty array rather than an error

#### Scenario: No external calls or filesystem writes occur
- **WHEN** `GET /api/v1/library` is called
- **THEN** the system SHALL NOT call AcoustID, MusicBrainz, Cover Art Archive, or LRCLIB, and SHALL NOT call `os.MkdirAll` or `os.Rename` on any file under `/music`, and SHALL NOT perform a disk walk or invoke `fpcalc`

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, identification status, a condensed resolved-metadata summary, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows, provide bulk actions to identify, enrich, tag, and relocate the selected rows, and allow opening a full details view for any single row.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes

#### Scenario: Rows can be selected for bulk identification
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable an "Identify Selected" action that, when triggered, submits the selected files' paths for identification

#### Scenario: Identify action disabled while an identify job is running
- **WHEN** an identification job is currently running
- **THEN** the UI's identify action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk enrichment
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable an "Enrich Selected" action that, when triggered, submits the selected files' paths for cover art and lyrics enrichment

#### Scenario: Enrich action disabled while an enrich job is running
- **WHEN** an enrichment job is currently running
- **THEN** the UI's enrich action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk tagging
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable a "Tag Selected" action that, when triggered, submits the selected files' paths for tag writing

#### Scenario: Tag action disabled while a tag job is running
- **WHEN** a tag job is currently running
- **THEN** the UI's tag action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk relocation
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable a "Relocate Selected" action that, when triggered, submits the selected files' paths for relocation

#### Scenario: Relocate action disabled while a relocate job is running
- **WHEN** a relocate job is currently running
- **THEN** the UI's relocate action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Refresh trigger disabled while a relocate job is running
- **WHEN** a relocate job is currently running
- **THEN** the UI's refresh trigger control SHALL be disabled, same as while a refresh itself is running

#### Scenario: Relocate action disabled while a refresh is running
- **WHEN** a background refresh is currently running
- **THEN** the UI's relocate action SHALL be disabled, same as while a relocate job itself is running

### Requirement: Concurrent refresh prevention
The system SHALL allow at most one refresh to run at a time, and SHALL NOT allow a refresh to start while a relocate job is running.

#### Scenario: Refresh requested while one is already running
- **WHEN** a client issues `POST /api/v1/library/scan` while a refresh is already in progress
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent refresh SHALL be started

#### Scenario: Refresh requested while a relocate job is running
- **WHEN** a client issues `POST /api/v1/library/scan` while a relocate job is currently running
- **THEN** the response SHALL be `409 Conflict` and no refresh SHALL be started, since a scan walking `/music` concurrently with a file being moved could observe it as both missing at its old location and new at its new one

## ADDED Requirements

### Requirement: On-demand relocation action
The system SHALL expose a `POST /api/v1/library/relocate` endpoint accepting a list of one or more file paths, which starts a background job physically relocating each already-identified-and-tagged path into the canonical directory hierarchy (per the `file-relocation` capability) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Relocate job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/relocate` with one or more paths while no relocate job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Trigger rejects an empty path list
- **WHEN** a client issues `POST /api/v1/library/relocate` with an empty or missing paths list
- **THEN** the response SHALL be `400 Bad Request` and no job SHALL be started

#### Scenario: Concurrent relocate job rejected
- **WHEN** a client issues `POST /api/v1/library/relocate` while a relocate job is already running
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent relocate job SHALL be started

#### Scenario: Relocate job runs independently of identify and enrich and tag
- **WHEN** an identify job, an enrich job, or a tag job is currently running
- **THEN** a relocate job SHALL still be accepted and run concurrently, since relocate shares no concurrency guard with any of the three

#### Scenario: Relocate job rejected while a scan refresh is running
- **WHEN** a client issues `POST /api/v1/library/relocate` while a background scan refresh is currently running
- **THEN** the response SHALL be `409 Conflict` and no relocate job SHALL be started, since a relocate job moving a file concurrently with a scan walking `/music` could cause that file to be observed as both missing at its old location and new at its new one

### Requirement: Relocation progress is observable
The system SHALL expose a `GET /api/v1/library/relocate/status` endpoint reporting whether a relocate job is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries relocate status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no relocate job is running
- **WHEN** a client queries relocate status while no relocate job is in progress
- **THEN** the response SHALL indicate that no relocate job is currently running
