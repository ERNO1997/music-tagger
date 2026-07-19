## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns the currently tracked file list — path, format, duration, fingerprint, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, and a tagged indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

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
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, fingerprint, identification status, a condensed resolved-metadata summary, a cover art thumbnail when present, a lyrics indicator when present, and a tagged indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows, provide bulk actions to identify, enrich, and tag the selected rows, and allow opening a full details view for any single row.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, and a tagged indicator when present

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

### Requirement: Per-file details view
The system SHALL allow a user to open a details view for a single tracked file, showing its complete resolved record — path, format, duration, fingerprint, status, any fingerprint error, (once identified) artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) its cover art and lyrics, and (once tagged) the file's actual embedded tags read live from disk, shown alongside the resolved metadata for visual comparison. Opening this view SHALL NOT require any request beyond the already-fetched `GET /api/v1/library` data, except for the cover art image, lyrics text, and embedded tags themselves.

#### Scenario: Opening details for a row
- **WHEN** a user clicks a row (other than its selection checkbox)
- **THEN** the UI SHALL display that file's complete resolved record without issuing any new network request beyond loading its cover art image, lyrics, and embedded tags, if present

#### Scenario: Selecting a row does not open its details
- **WHEN** a user clicks a row's selection checkbox
- **THEN** the UI SHALL toggle that row's selection state and SHALL NOT open the details view

#### Scenario: Details view for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `missing`
- **THEN** the UI SHALL show the fields available for that status (path, format, duration, fingerprint, status, and any fingerprint error) without fabricating placeholder metadata values

#### Scenario: Details view fetches lyrics on open
- **WHEN** a user opens the details view for a file with a lyrics indicator present
- **THEN** the UI SHALL call `GET /api/v1/library/lyrics` for that file's path and render the returned lyrics text

#### Scenario: Details view fetches and displays embedded tags for a tagged file
- **WHEN** a user opens the details view for a file with a `tagged` indicator present
- **THEN** the UI SHALL call `GET /api/v1/library/tags` for that file's path and render the returned embedded title/artist/album/album artist/track number/disc number/year and lyrics/cover-art-present indicators in a section visually distinct from (and directly comparable to) the resolved metadata

#### Scenario: Embedded tags are not fetched for an untagged file
- **WHEN** a user opens the details view for a file with no `tagged` indicator present
- **THEN** the UI SHALL NOT call `GET /api/v1/library/tags` for that file

## ADDED Requirements

### Requirement: On-demand tagging action
The system SHALL expose a `POST /api/v1/library/tag` endpoint accepting a list of one or more file paths, which starts a background job writing each already-identified path's resolved metadata, cover art, and lyrics into the physical audio file's own tags (per the `audio-tag-writing` capability) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Tag job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/tag` with one or more paths while no tag job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Trigger rejects an empty path list
- **WHEN** a client issues `POST /api/v1/library/tag` with an empty or missing paths list
- **THEN** the response SHALL be `400 Bad Request` and no job SHALL be started

#### Scenario: Concurrent tag job rejected
- **WHEN** a client issues `POST /api/v1/library/tag` while a tag job is already running
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent tag job SHALL be started

#### Scenario: Tag job runs independently of scan, identify, and enrich
- **WHEN** a scan refresh, an identify job, or an enrich job is currently running
- **THEN** a tag job SHALL still be accepted and run concurrently, since none of the four share a concurrency guard

### Requirement: Tagging progress is observable
The system SHALL expose a `GET /api/v1/library/tag/status` endpoint reporting whether a tag job is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries tag status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no tag job is running
- **WHEN** a client queries tag status while no tag job is in progress
- **THEN** the response SHALL indicate that no tag job is currently running

### Requirement: Embedded tag retrieval via API
The system SHALL expose a `GET /api/v1/library/tags` endpoint that, given a tracked file's path, reads that file's actual embedded tags directly from disk (per the `audio-tag-writing` capability) and returns them as JSON, independent of the resolved metadata cached in the tracking store.

#### Scenario: Embedded tags available
- **WHEN** a client requests embedded tags for a tracked file
- **THEN** the response SHALL be `200 OK` with a JSON body containing that file's actual embedded title, artist, album, album artist, track number, disc number, year, and whether lyrics and cover art are embedded

#### Scenario: File not found on disk
- **WHEN** a client requests embedded tags for a tracked path that is currently missing from disk
- **THEN** the response SHALL be `404 Not Found`
