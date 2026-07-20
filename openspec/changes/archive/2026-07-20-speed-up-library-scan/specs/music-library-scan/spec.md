## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns a page of the currently tracked file list — path, format, duration, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, a tagged indicator, and a relocated indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. A file's reported `path` SHALL always be its current, possibly-relocated location. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

The endpoint SHALL accept optional query parameters: `status` (one of `new`, `identified`, `not_found`, `missing`, restricting results to that effective status), `tagged` and `relocated` (boolean, restricting to files with that outcome true or false), `q` (a case-insensitive substring search matched against path, artist, album, and title), `sort` (one of `path`, `status`, `artist`, `album`, `duration`, `year`) and `order` (`asc` or `desc`, defaulting to `asc`), and `limit`/`offset` for pagination. The response SHALL be a JSON object `{"total": <matching row count>, "entries": [...]}` rather than a bare array, so a client can render pagination controls without a separate count request. The `fingerprint` field, previously included per-row, SHALL NOT be included in this response — it is available on demand via a separate endpoint.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON object containing `total` and an `entries` array where each entry includes `path`, `format`, `duration_seconds`, `status`, and an `error` field populated only when that file's most recent duration-read attempt failed

### Requirement: Asynchronous refresh action
The system SHALL expose a `POST /api/v1/library/scan` endpoint that starts the disk walk and tracking-store update (per the `file-tracking-store` capability) in the background and returns immediately, rather than blocking for the duration of the refresh. A refresh SHALL NOT fingerprint any file — fingerprinting happens lazily during identification (see the `file-tracking-store` capability's "Fingerprint computed lazily during identification" requirement).

#### Scenario: Refresh accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/scan` while no refresh is running
- **THEN** the response SHALL be `202 Accepted` and the walk/duration-read/update SHALL proceed asynchronously, without the HTTP request blocking until it finishes

#### Scenario: Per-file duration-read failure does not abort the refresh
- **WHEN** one file in `/music` fails to have its duration read during a refresh (e.g. a corrupt audio file)
- **THEN** that file SHALL be reported with an error indicator in its tracked record and the refresh SHALL continue processing the remaining files

### Requirement: Refresh progress is observable
The system SHALL expose a way for clients to determine whether a refresh is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries refresh status while a refresh is in progress
- **THEN** the response SHALL indicate that a refresh is running and SHALL include how many of the files needing a duration read have been processed so far

#### Scenario: Idle status when no refresh is running
- **WHEN** a client queries refresh status while no refresh is in progress
- **THEN** the response SHALL indicate that no refresh is currently running

### Requirement: On-demand identification action
The system SHALL expose a `POST /api/v1/library/identify` endpoint accepting either a list of one or more file paths, or a filter (in the same shape accepted by `GET /api/v1/library`'s `status`/`tagged`/`relocated`/`q` query parameters), which starts a background job resolving each matching path's canonical metadata via AcoustID and MusicBrainz (per the `acoustid-lookup` and `musicbrainz-metadata` capabilities) and returns immediately rather than blocking for the duration of the job. When a filter is given, the system SHALL resolve it to the current set of matching paths at the moment the job starts, not at some earlier time the filter's matching count may have been displayed. A path with no fingerprint already stored SHALL have one computed as part of this job, per the `file-tracking-store` capability's "Fingerprint computed lazily during identification" requirement, rather than being skipped.

#### Scenario: Identify job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/identify` with one or more paths while no identify job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, one at a time, without the HTTP request blocking until the job finishes

#### Scenario: Identify job accepted with a filter instead of explicit paths
- **WHEN** a client issues `POST /api/v1/library/identify` with a filter instead of an explicit path list
- **THEN** the system SHALL resolve every currently-tracked file matching that filter into a path list and process it the same as an explicitly-submitted list

#### Scenario: Concurrent identify job rejected
- **WHEN** a client issues `POST /api/v1/library/identify` while an identify job is already running
- **THEN** the response SHALL be `409 Conflict` and no second, concurrent identify job SHALL be started

#### Scenario: Identify job runs independently of a scan refresh
- **WHEN** a scan refresh is currently running
- **THEN** an identify job SHALL still be accepted and run concurrently, since the two do not share a concurrency guard

#### Scenario: A path with no stored fingerprint is processed, not skipped
- **WHEN** an identify job reaches a path with no fingerprint stored yet
- **THEN** the system SHALL compute that fingerprint as part of processing that path, rather than skipping it

### Requirement: Fingerprint retrieval via API
The system SHALL expose a `GET /api/v1/library/fingerprint` endpoint that, given a tracked file's path, returns that file's stored Chromaprint fingerprint as JSON, separately from the main list endpoint.

#### Scenario: Fingerprint available
- **WHEN** a client requests the fingerprint for a tracked file that has one stored
- **THEN** the response SHALL be `200 OK` with a JSON body containing that file's fingerprint string

#### Scenario: Fingerprint not yet computed
- **WHEN** a client requests the fingerprint for a tracked file that has not yet been fingerprinted (no identify attempt has run for it)
- **THEN** the response SHALL be `200 OK` with an empty fingerprint string, distinct from the `404` given for a path that isn't tracked at all

#### Scenario: Unknown path
- **WHEN** a client requests the fingerprint for a path that is not tracked
- **THEN** the response SHALL be `404 Not Found`
