## Purpose

Discovery, persistent tracking, fingerprint reporting, on-demand identification, on-demand cover art/lyrics enrichment, on-demand tag writing, and on-demand file relocation for the mounted local `/music` volume: recursively finding supported audio files, keeping a durable per-file record of their status (via the `file-tracking-store` capability), resolving canonical metadata on demand (via the `acoustid-lookup` and `musicbrainz-metadata` capabilities), resolving cover art and lyrics on demand (via the `cover-art-lookup` and `lyrics-lookup` capabilities), writing resolved metadata/cover art/lyrics into the physical audio file on demand (via the `audio-tag-writing` capability), physically relocating already-tagged files into a canonical directory hierarchy on demand (via the `file-relocation` capability), and surfacing all of this through an API and a web listing page.

## Requirements

### Requirement: Recursive discovery of audio files in the mounted volume
The system SHALL recursively walk the configured `/music` directory and identify all files with `.mp3`, `.flac`, or `.m4a` extensions as candidate audio files, at any subdirectory depth.

#### Scenario: Nested directories are included
- **WHEN** `/music` contains audio files nested at arbitrary subdirectory depths
- **THEN** all matching files SHALL be included in the scan result regardless of depth

#### Scenario: Non-audio files are ignored
- **WHEN** `/music` contains files with extensions other than `.mp3`/`.flac`/`.m4a`
- **THEN** those files SHALL be excluded from the scan result and SHALL NOT be passed to the fingerprinting component

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

### Requirement: Asynchronous refresh action
The system SHALL expose a `POST /api/v1/library/scan` endpoint that starts the disk walk, fingerprinting, and tracking-store update (per the `file-tracking-store` capability) in the background and returns immediately, rather than blocking for the duration of the refresh.

#### Scenario: Refresh accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/scan` while no refresh is running
- **THEN** the response SHALL be `202 Accepted` and the walk/fingerprint/update SHALL proceed asynchronously, without the HTTP request blocking until it finishes

#### Scenario: Per-file fingerprint failure does not abort the refresh
- **WHEN** one file in `/music` fails fingerprinting during a refresh (e.g. a corrupt audio file)
- **THEN** that file SHALL be reported with an error indicator in its tracked record and the refresh SHALL continue processing the remaining files

### Requirement: Concurrent refresh prevention
The system SHALL allow at most one refresh to run at a time, and SHALL NOT allow a refresh to start while a relocate job is running.

#### Scenario: Refresh requested while one is already running
- **WHEN** a client issues `POST /api/v1/library/scan` while a refresh is already in progress
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent refresh SHALL be started

#### Scenario: Refresh requested while a relocate job is running
- **WHEN** a client issues `POST /api/v1/library/scan` while a relocate job is currently running
- **THEN** the response SHALL be `409 Conflict` and no refresh SHALL be started, since a scan walking `/music` concurrently with a file being moved could observe it as both missing at its old location and new at its new one

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

### Requirement: Enrichment progress is observable
The system SHALL expose a `GET /api/v1/library/enrich/status` endpoint reporting whether an enrich job is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries enrich status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no enrich job is running
- **WHEN** a client queries enrich status while no enrich job is in progress
- **THEN** the response SHALL indicate that no enrich job is currently running

### Requirement: Cover art image serving
The system SHALL expose a `GET /api/v1/library/cover` endpoint that, given a tracked file's path, serves that file's stored cover art image bytes with the correct content type.

#### Scenario: Cover art available
- **WHEN** a client requests cover art for a file with a stored cover art path
- **THEN** the response SHALL be `200 OK` with the image bytes and an appropriate `Content-Type`

#### Scenario: No cover art stored
- **WHEN** a client requests cover art for a file with no stored cover art path
- **THEN** the response SHALL be `404 Not Found`

### Requirement: Lyrics retrieval via API
The system SHALL expose a `GET /api/v1/library/lyrics` endpoint that, given a tracked file's path, returns that file's stored plain and synced lyrics as JSON.

#### Scenario: Lyrics available
- **WHEN** a client requests lyrics for a file with stored lyrics
- **THEN** the response SHALL be `200 OK` with a JSON body containing the plain lyrics and, when available, synced lyrics

#### Scenario: No lyrics stored
- **WHEN** a client requests lyrics for a file with no stored lyrics
- **THEN** the response SHALL be `404 Not Found`

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

### Requirement: Embedded tag retrieval via API
The system SHALL expose a `GET /api/v1/library/tags` endpoint that, given a tracked file's path, reads that file's actual embedded tags directly from disk (per the `audio-tag-writing` capability) and returns them as JSON, independent of the resolved metadata cached in the tracking store.

#### Scenario: Embedded tags available
- **WHEN** a client requests embedded tags for a tracked file
- **THEN** the response SHALL be `200 OK` with a JSON body containing that file's actual embedded title, artist, album, album artist, track number, disc number, year, and whether lyrics and cover art are embedded

#### Scenario: File not found on disk
- **WHEN** a client requests embedded tags for a tracked path that is currently missing from disk
- **THEN** the response SHALL be `404 Not Found`
