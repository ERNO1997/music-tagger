## MODIFIED Requirements

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that returns a page of the currently tracked file list — path, format, duration, identification status, a raw tag snapshot (title, artist, album, album artist as embedded in the file itself, when captured), (once identified) resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) a cover art indicator, a lyrics indicator, a tagged indicator, and a relocated indicator — read directly from the persistent tracking store (see the `file-tracking-store` capability), without performing a disk walk or fingerprinting on every call. A file's reported `path` SHALL always be its current, possibly-relocated location. The endpoint SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

The endpoint SHALL accept optional query parameters: `status` (one of `new`, `identified`, `not_found`, `ambiguous`, `missing`, restricting results to that effective status), `tagged`, `relocated`, `has_lyrics`, and `has_cover_art` (boolean, restricting to files with that outcome true or false), `q` (a case-insensitive substring search matched against path, artist, album, title, and raw title/artist/album), `sort` (one of `path`, `status`, `artist`, `album`, `duration`, `year`) and `order` (`asc` or `desc`, defaulting to `asc`), and `limit`/`offset` for pagination. The response SHALL be a JSON object `{"total": <matching row count>, "entries": [...]}` rather than a bare array, so a client can render pagination controls without a separate count request. The `fingerprint` field, previously included per-row, SHALL NOT be included in this response — it is available on demand via a separate endpoint.

#### Scenario: Successful read of tracked state
- **WHEN** a client issues `GET /api/v1/library` after at least one refresh has run
- **THEN** the response SHALL be `200 OK` with a JSON object containing `total` and an `entries` array where each entry includes `path`, `format`, `duration_seconds`, `status`, and an `error` field populated only when that file's most recent duration-read attempt failed

#### Scenario: Identified file includes resolved metadata
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has status `identified`
- **THEN** that file's entry SHALL include its resolved artist, album artist, title, track number, release year (when available), disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs

#### Scenario: Unidentified file includes its raw tag snapshot
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has a captured raw tag snapshot but no resolved metadata (status `new`, `not_found`, or `ambiguous`)
- **THEN** that file's entry SHALL include whichever of raw title/artist/album/album-artist were captured, distinguishable from resolved metadata fields

#### Scenario: Identified file's raw tag snapshot is still available alongside resolved metadata
- **WHEN** a client issues `GET /api/v1/library` and a tracked file has both a raw tag snapshot and resolved metadata
- **THEN** that file's entry SHALL include both, without the presence of resolved metadata suppressing the raw tag fields from the response

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
- **THEN** the endpoint SHALL return `200 OK` with `total` of `0` and an empty `entries` array rather than an error

#### Scenario: No external calls or filesystem writes occur
- **WHEN** `GET /api/v1/library` is called
- **THEN** the system SHALL NOT call AcoustID, MusicBrainz, Cover Art Archive, or LRCLIB, and SHALL NOT call `os.MkdirAll` or `os.Rename` on any file under `/music`, and SHALL NOT perform a disk walk or invoke `fpcalc`

#### Scenario: Filtering by status
- **WHEN** a client issues `GET /api/v1/library?status=missing`
- **THEN** the response SHALL include only tracked files whose effective status is `missing`, and `total` SHALL reflect that filtered count, not the full tracked count

#### Scenario: Filtering by tagged or relocated outcome
- **WHEN** a client issues `GET /api/v1/library?tagged=false` or `?relocated=false`
- **THEN** the response SHALL include only files whose tagged (or relocated) outcome matches the given boolean

#### Scenario: Filtering by lyrics outcome
- **WHEN** a client issues `GET /api/v1/library?has_lyrics=false`
- **THEN** the response SHALL include only files whose stored plain and synced lyrics are both empty, and `total` SHALL reflect that filtered count

#### Scenario: Filtering by cover art outcome
- **WHEN** a client issues `GET /api/v1/library?has_cover_art=false`
- **THEN** the response SHALL include only files with no stored cover art path, and `total` SHALL reflect that filtered count

#### Scenario: Filtering by the ambiguous status
- **WHEN** a client issues `GET /api/v1/library?status=ambiguous`
- **THEN** the response SHALL include only files whose status is `ambiguous`, and `total` SHALL reflect that filtered count

#### Scenario: Free-text search across path, artist, album, and title
- **WHEN** a client issues `GET /api/v1/library?q=rasmus`
- **THEN** the response SHALL include only files whose path, artist, album, or title contains that text, case-insensitively

#### Scenario: Searching by raw tag data
- **WHEN** a client issues `GET /api/v1/library?q=<text>` matching a tracked file's raw title, artist, or album but not its path or resolved metadata
- **THEN** that file SHALL be included in the response

#### Scenario: Sorting results
- **WHEN** a client issues `GET /api/v1/library?sort=artist&order=desc`
- **THEN** the response's `entries` SHALL be ordered by artist, descending, with ties broken deterministically so repeated requests against unchanged data return the same order

#### Scenario: Paginating results
- **WHEN** a client issues `GET /api/v1/library?limit=50&offset=100`
- **THEN** the response SHALL include at most 50 entries starting after the first 100 matching rows, and `total` SHALL reflect the full matching count regardless of `limit`/`offset`

#### Scenario: Filters, search, sort, and pagination compose together
- **WHEN** a client issues `GET /api/v1/library` with a combination of `status`, `q`, `sort`, and `limit`/`offset`
- **THEN** the system SHALL apply the filter and search first, then sort, then paginate the result, consistently with `total` reflecting the post-filter, pre-pagination count

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders one page of results as a table showing path, format, duration, identification status, a condensed resolved-metadata summary (or, when a file is not yet identified, its raw tag snapshot when captured, so a poorly-named file's actual title/artist is still visible), a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows (or all rows matching the current filter, across pages), provide bulk actions to identify, enrich, tag, and relocate the selected rows, provide a delete action for rows with status `missing`, provide a resolve action for rows with status `ambiguous`, and allow opening a full details view for any single row. It SHALL provide controls for filtering by status/tagged/relocated/has-lyrics/has-cover-art, free-text search, column sorting, and page navigation.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file for the current page, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present

#### Scenario: An unidentified file's row shows its raw tag snapshot instead of blank metadata
- **WHEN** a table row's file has status `new`, `not_found`, or `ambiguous` and a captured raw tag snapshot
- **THEN** the row's metadata summary SHALL show the raw title/artist/album, visually distinguished (e.g. styled or labeled differently) from a resolved-metadata summary shown for an `identified` row

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
- **WHEN** a user sets a status/tagged/relocated/has-lyrics/has-cover-art filter, enters search text, or clicks a sortable column header
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

#### Scenario: Filtering by cover art outcome in the UI
- **WHEN** a user sets the cover art filter to "missing cover"
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with `has_cover_art=false` and re-render the table accordingly

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

#### Scenario: Details view shows the raw tag snapshot for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `ambiguous` and a captured raw tag snapshot
- **THEN** the UI SHALL display the raw title/artist/album/album-artist fields, labeled as embedded-in-file data rather than resolved metadata
