## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns a page of the currently tracked file list — path, format, duration, identification status, (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, a tagged indicator, and a relocated indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. A file's reported `path` SHALL always be its current, possibly-relocated location. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

The endpoint SHALL accept optional query parameters: `status` (one of `new`, `identified`, `not_found`, `missing`, restricting results to that effective status), `tagged`, `relocated`, and `has_lyrics` (boolean, restricting to files with that outcome true or false), `q` (a case-insensitive substring search matched against path, artist, album, and title), `sort` (one of `path`, `status`, `artist`, `album`, `duration`, `year`) and `order` (`asc` or `desc`, defaulting to `asc`), and `limit`/`offset` for pagination. The response SHALL be a JSON object `{"total": <matching row count>, "entries": [...]}` rather than a bare array, so a client can render pagination controls without a separate count request. The `fingerprint` field, previously included per-row, SHALL NOT be included in this response — it is available on demand via a separate endpoint.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON object containing `total` and an `entries` array where each entry includes `path`, `format`, `duration_seconds`, `status`, and an `error` field populated only when that file's most recent fingerprint attempt failed

#### Scenario: Filtering by lyrics outcome
- **WHEN** a client issues `GET /api/v1/library?has_lyrics=false`
- **THEN** the response SHALL include only files whose stored plain and synced lyrics are both empty, and `total` SHALL reflect that filtered count

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders one page of results as a table showing path, format, duration, identification status, a condensed resolved-metadata summary, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows (or all rows matching the current filter, across pages), provide bulk actions to identify, enrich, tag, and relocate the selected rows, provide a delete action for rows with status `missing`, and allow opening a full details view for any single row. It SHALL provide controls for filtering by status/tagged/relocated/has-lyrics, free-text search, column sorting, and page navigation.

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
