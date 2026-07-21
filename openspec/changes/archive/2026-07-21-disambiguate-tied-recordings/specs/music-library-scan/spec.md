## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns a page of the currently tracked file list — path, format, duration, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, a tagged indicator, and a relocated indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. A file's reported `path` SHALL always be its current, possibly-relocated location. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

The endpoint SHALL accept optional query parameters: `status` (one of `new`, `identified`, `not_found`, `ambiguous`, `missing`, restricting results to that effective status), `tagged`, `relocated`, and `has_lyrics` (boolean, restricting to files with that outcome true or false), `q` (a case-insensitive substring search matched against path, artist, album, and title), `sort` (one of `path`, `status`, `artist`, `album`, `duration`, `year`) and `order` (`asc` or `desc`, defaulting to `asc`), and `limit`/`offset` for pagination. The response SHALL be a JSON object `{"total": <matching row count>, "entries": [...]}` rather than a bare array, so a client can render pagination controls without a separate count request. The `fingerprint` field, previously included per-row, SHALL NOT be included in this response — it is available on demand via a separate endpoint.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON object containing `total` and an `entries` array where each entry includes `path`, `format`, `duration_seconds`, `status`, and an `error` field populated only when that file's most recent duration-read attempt failed

#### Scenario: Filtering by lyrics outcome
- **WHEN** a client issues `GET /api/v1/library?has_lyrics=false`
- **THEN** the response SHALL include only files whose stored plain and synced lyrics are both empty, and `total` SHALL reflect that filtered count

#### Scenario: Filtering by the ambiguous status
- **WHEN** a client issues `GET /api/v1/library?status=ambiguous`
- **THEN** the response SHALL include only files whose status is `ambiguous`, and `total` SHALL reflect that filtered count

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders one page of results as a table showing path, format, duration, identification status, a condensed resolved-metadata summary, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows (or all rows matching the current filter, across pages), provide bulk actions to identify, enrich, tag, and relocate the selected rows, provide a delete action for rows with status `missing`, provide a resolve action for rows with status `ambiguous`, and allow opening a full details view for any single row. It SHALL provide controls for filtering by status/tagged/relocated/has-lyrics, free-text search, column sorting, and page navigation.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file for the current page, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes

#### Scenario: Rows can be selected for bulk identification
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable an "Identify Selected" action that, when triggered, submits the selected files for identification

#### Scenario: Identify action disabled while an identify job is running
- **WHEN** an identification job is currently running
- **THEN** the UI's identify action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk enrichment
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable an "Enrich Selected" action that, when triggered, submits the selected files for cover art and lyrics enrichment

#### Scenario: Enrich action disabled while an enrich job is running
- **WHEN** an enrichment job is currently running
- **THEN** the UI's enrich action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk tagging
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable a "Tag Selected" action that, when triggered, submits the selected files for tag writing

#### Scenario: Tag action disabled while a tag job is running
- **WHEN** a tag job is currently running
- **THEN** the UI's tag action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Rows can be selected for bulk relocation
- **WHEN** a user selects one or more rows in the table
- **THEN** the UI SHALL enable a "Relocate Selected" action that, when triggered, submits the selected files for relocation

#### Scenario: Relocate action disabled while a relocate job is running
- **WHEN** a relocate job is currently running
- **THEN** the UI's relocate action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Refresh trigger disabled while a relocate job is running
- **WHEN** a relocate job is currently running
- **THEN** the UI's refresh trigger control SHALL be disabled, same as while a refresh itself is running

#### Scenario: Relocate action disabled while a refresh is running
- **WHEN** a background refresh is currently running
- **THEN** the UI's relocate action SHALL be disabled, same as while a relocate job itself is running

#### Scenario: Filtering, searching, and sorting the table
- **WHEN** a user sets a status/tagged/relocated/has-lyrics filter, enters search text, or clicks a sortable column header
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with the corresponding query parameters and re-render the table to reflect only the current page of matching, sorted results

#### Scenario: Navigating between pages
- **WHEN** a user changes page size or navigates to another page
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with the corresponding `limit`/`offset` and replace the currently rendered rows with that page's results

#### Scenario: Selecting all rows matching the current filter, not just the current page
- **WHEN** a user chooses "select all matching" while a filter and/or search is active
- **THEN** the UI SHALL treat the selection as "every file matching the current filter" (potentially far more than the current page's rows) rather than only the rows currently rendered, and SHALL visibly distinguish this from an explicit, page-scoped selection

#### Scenario: Bulk actions submit the active selection mode
- **WHEN** a bulk action (identify, enrich, tag, or relocate) is triggered
- **THEN** the UI SHALL submit either the explicit set of selected paths, or the active filter criteria if "select all matching" is active, matching whichever selection mode is currently shown to the user

#### Scenario: Large-selection notice before identifying
- **WHEN** a user triggers "Identify Selected" over a selection larger than a small threshold
- **THEN** the UI SHALL show an estimated completion time computed from the selection size and MusicBrainz's 1 request/second pace, before or as part of starting the job

#### Scenario: Deleting a missing file's tracked entry
- **WHEN** a user triggers the delete action on a row with status `missing`
- **THEN** the UI SHALL prompt for confirmation before calling the delete endpoint, and SHALL remove that row from the table once the deletion succeeds

#### Scenario: Delete action is not available for non-missing files
- **WHEN** a row's status is not `missing`
- **THEN** the UI SHALL NOT offer a delete action for that row

#### Scenario: Filtering by lyrics outcome in the UI
- **WHEN** a user sets the lyrics filter to "missing lyrics"
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with `has_lyrics=false` and re-render the table accordingly

#### Scenario: Ambiguous rows are visually distinguished
- **WHEN** a row's status is `ambiguous`
- **THEN** the UI SHALL show that status with its own distinct label/indicator, separate from `identified` and `not_found`

#### Scenario: Resolving an ambiguous file from the details view
- **WHEN** a user opens the details view for a row with status `ambiguous`
- **THEN** the UI SHALL fetch and display that file's stored candidates and SHALL let the user choose one, calling the resolve endpoint and reflecting the row as `identified` with the chosen candidate's metadata once resolution succeeds

#### Scenario: Resolve action is not available for non-ambiguous files
- **WHEN** a row's status is not `ambiguous`
- **THEN** the UI SHALL NOT offer a candidate-resolve action for that row

#### Scenario: Browsing alternate covers from the details view
- **WHEN** a user opens the details view for an `identified` row and triggers "browse other covers"
- **THEN** the UI SHALL fetch and display cover-art candidates across that file's release-group's sibling editions, and SHALL let the user choose one, calling the choose endpoint and reflecting the row's cover art with the chosen image once the choice succeeds

#### Scenario: Cover-browsing action is not available for unidentified files
- **WHEN** a row's status is not `identified`
- **THEN** the UI SHALL NOT offer a cover-browsing action for that row

### Requirement: On-demand identification action
The system SHALL expose a `POST /api/v1/library/identify` endpoint accepting either a list of one or more file paths, or a filter (in the same shape accepted by `GET /api/v1/library`'s `status`/`tagged`/`relocated`/`has_lyrics`/`q` query parameters), which starts a background job resolving each matching path's canonical metadata via AcoustID and MusicBrainz (per the `acoustid-lookup` and `musicbrainz-metadata` capabilities) and returns immediately rather than blocking for the duration of the job. When a filter is given, the system SHALL resolve it to the current set of matching paths at the moment the job starts, not at some earlier time the filter's matching count may have been displayed. A path with no fingerprint already stored SHALL have one computed as part of this job, per the `file-tracking-store` capability's "Fingerprint computed lazily during identification" requirement, rather than being skipped. A path whose accepted AcoustID match ties multiple distinct recordings SHALL be recorded `ambiguous` with its candidates stored, per the `file-tracking-store` capability's "Ambiguous identification is recorded with candidate metadata" requirement, rather than one recording being auto-picked.

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

#### Scenario: A path with tied recordings is recorded ambiguous, not auto-picked
- **WHEN** an identify job reaches a path whose accepted AcoustID match ties multiple recordings resolving to distinct identities
- **THEN** the system SHALL record that path `ambiguous` with its candidates stored, and continue processing the rest of the job

## ADDED Requirements

### Requirement: Candidate retrieval via API
The system SHALL expose a `GET /api/v1/library/candidates` endpoint that, given a tracked file's path, returns its stored candidate list (each candidate's resolved artist, album, title, track number, and other resolved metadata) as JSON, separately from the main list endpoint.

#### Scenario: Candidates available
- **WHEN** a client requests candidates for a file with status `ambiguous`
- **THEN** the response SHALL be `200 OK` with a JSON body containing that file's stored candidate list

#### Scenario: No candidates stored
- **WHEN** a client requests candidates for a tracked file with no stored candidates (never ambiguous, or already resolved)
- **THEN** the response SHALL be `200 OK` with an empty candidate list, distinct from the `404` given for a path that isn't tracked at all

#### Scenario: Unknown path
- **WHEN** a client requests candidates for a path that is not tracked
- **THEN** the response SHALL be `404 Not Found`

### Requirement: On-demand candidate resolution action
The system SHALL expose a `POST /api/v1/library/identify/resolve` endpoint accepting a tracked file's path and one of its stored candidates' recording ID, which records that candidate as the file's resolved identification (per the `file-tracking-store` capability's "A stored candidate can be chosen to resolve an ambiguous file" requirement) and responds synchronously, since resolving a stored candidate requires no external network call.

#### Scenario: Resolving a valid candidate succeeds
- **WHEN** a client issues `POST /api/v1/library/identify/resolve` with a path and a recording ID matching one of that file's stored candidates
- **THEN** the response SHALL be `200 OK`, the file's status SHALL become `identified` with that candidate's metadata, and its other stored candidates SHALL be discarded

#### Scenario: Resolving an unrecognized candidate is rejected
- **WHEN** a client issues `POST /api/v1/library/identify/resolve` with a path and a recording ID that does not match any of that file's stored candidates
- **THEN** the response SHALL be `404 Not Found` and the file's status and stored candidates SHALL remain unchanged

### Requirement: Cover-art candidate retrieval via API
The system SHALL expose a `GET /api/v1/library/cover/candidates` endpoint that, given an identified tracked file's path, returns front-cover candidates (release ID, release title, thumbnail URL, and image URL) across that file's release-group's sibling editions (per the `cover-art-lookup` and `musicbrainz-metadata` capabilities), separately from the main list endpoint and from the single automatically-resolved cover.

#### Scenario: Candidates available
- **WHEN** a client requests cover candidates for an identified file whose release-group has at least one sibling edition with a front cover uploaded
- **THEN** the response SHALL be `200 OK` with a JSON body containing those candidates

#### Scenario: No candidates found
- **WHEN** a client requests cover candidates for an identified file whose release-group has no sibling edition with a front cover uploaded
- **THEN** the response SHALL be `200 OK` with an empty candidate list

#### Scenario: Unknown or unidentified path
- **WHEN** a client requests cover candidates for a path that is not tracked, or is tracked but not yet `identified`
- **THEN** the response SHALL be `404 Not Found`

### Requirement: On-demand cover-art choice action
The system SHALL expose a `POST /api/v1/library/cover/choose` endpoint accepting a tracked file's path, a candidate's release ID, and its image URL, which downloads that image and records it as the file's cover art (per the `file-tracking-store` capability's existing cover-art recording, identically to automatic enrichment) and responds synchronously.

#### Scenario: Choosing a candidate succeeds
- **WHEN** a client issues `POST /api/v1/library/cover/choose` with a path and a release ID/image URL previously returned by the candidates endpoint
- **THEN** the response SHALL be `200 OK` and the file's cover art SHALL become the chosen image

#### Scenario: Choosing an image outside Cover Art Archive's host is rejected
- **WHEN** a client issues `POST /api/v1/library/cover/choose` with an image URL whose host is not Cover Art Archive's own domain
- **THEN** the system SHALL refuse the request with an error and SHALL NOT fetch it
