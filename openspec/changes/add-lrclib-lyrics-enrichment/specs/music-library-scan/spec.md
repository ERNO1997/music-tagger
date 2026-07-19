## ADDED Requirements

### Requirement: Lyrics retrieval via API
The system SHALL expose a `GET /api/v1/library/lyrics` endpoint that, given a tracked file's path, returns that file's stored plain and synced lyrics as JSON.

#### Scenario: Lyrics available
- **WHEN** a client requests lyrics for a file with stored lyrics
- **THEN** the response SHALL be `200 OK` with a JSON body containing the plain lyrics and, when available, synced lyrics

#### Scenario: No lyrics stored
- **WHEN** a client requests lyrics for a file with no stored lyrics
- **THEN** the response SHALL be `404 Not Found`

## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator and a lyrics indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

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
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, identification status, a condensed resolved-metadata summary, a cover art thumbnail when present, and a lyrics indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows, provide bulk actions to identify and to enrich the selected rows, and allow opening a full details view for any single row.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, including its status, any resolved metadata, a cover art thumbnail when present, and a lyrics indicator when present

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

### Requirement: On-demand enrichment action
The system SHALL expose a `POST /api/v1/library/enrich` endpoint accepting a list of one or more file paths, which starts a background job resolving each already-identified path's cover art via Cover Art Archive and lyrics via LRCLIB (per the `cover-art-lookup` and `lyrics-lookup` capabilities) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Enrich job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/enrich` with one or more paths while no enrich job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Concurrent enrich job rejected
- **WHEN** a client issues `POST /api/v1/library/enrich` while an enrich job is already running
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent enrich job SHALL be started

#### Scenario: Enrich job runs independently of scan and identify
- **WHEN** a scan refresh or an identify job is currently running
- **THEN** an enrich job SHALL still be accepted and run concurrently, since none of the three share a concurrency guard

#### Scenario: Cover art and lyrics are both attempted per file
- **WHEN** an enrich job processes an already-identified file
- **THEN** the system SHALL attempt to resolve both cover art and lyrics for that file, recording whichever of the two succeed independently of the other

### Requirement: Per-file details view
The system SHALL allow a user to open a details view for a single tracked file, showing its complete resolved record — path, format, duration, fingerprint, status, any fingerprint error, (once identified) artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) its cover art and lyrics. Opening this view SHALL NOT require any request beyond the already-fetched `GET /api/v1/library` data, except for the cover art image and lyrics text themselves.

#### Scenario: Opening details for a row
- **WHEN** a user clicks a row (other than its selection checkbox)
- **THEN** the UI SHALL display that file's complete resolved record without issuing any new network request beyond loading its cover art image and lyrics, if present

#### Scenario: Selecting a row does not open its details
- **WHEN** a user clicks a row's selection checkbox
- **THEN** the UI SHALL toggle that row's selection state and SHALL NOT open the details view

#### Scenario: Details view for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `missing`
- **THEN** the UI SHALL show the fields available for that status (path, format, duration, fingerprint, status, and any fingerprint error) without fabricating placeholder metadata values

#### Scenario: Details view fetches lyrics on open
- **WHEN** a user opens the details view for a file with a lyrics indicator present
- **THEN** the UI SHALL call `GET /api/v1/library/lyrics` for that file's path and render the returned lyrics text
