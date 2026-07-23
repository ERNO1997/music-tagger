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
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` (or, for the Folder and Artist-Album groupings, the `library-browsing` capability's tree/artist/album/track endpoints) and renders one page of results according to two independent choices: a **grouping** (All — the flat list; Folder — the mounted volume's directory structure; Artist-Album — resolved-or-raw-tag artist/album drill-down) and a **presentation** (Table or cover-forward Grid) applied to whatever file/track listing the current grouping is showing. It SHALL show, per entry, path, format, duration, identification status, a condensed resolved-metadata summary (or, when a file is not yet identified, its raw tag snapshot when captured, so a poorly-named file's actual title/artist is still visible), a cover art thumbnail when present, a lyrics indicator when present, a metadata-completeness indicator on the resolved-metadata summary for identified entries (distinguishing whether artist, album, title, and track number are all present, or naming whichever are missing), and a canonical-path indicator alongside the path itself (distinguishing a path that already matches the entry's computed relocation destination from one that doesn't, or where a relocation attempt has failed). It SHALL reflect whether a refresh is currently running, allow selecting one or more entries (or all entries matching the current filter, across pages) regardless of grouping or presentation, provide bulk actions to identify, enrich, tag, and relocate the selected entries, provide a delete action for entries with status `missing`, provide a resolve action for entries with status `ambiguous`, provide a manual search action available from any entry's details view regardless of status, and allow opening a full details view for any single entry. It SHALL provide controls for filtering by status/tagged/relocated/has-lyrics/has-cover-art, free-text search, column sorting (in table presentation), page navigation, and switching grouping and presentation independently.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL default to the All grouping in table presentation, call `GET /api/v1/library`, and render one row per returned file for the current page, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, a metadata-completeness indicator when identified, and a canonical-path indicator alongside its path

#### Scenario: An unidentified file's row shows its raw tag snapshot instead of blank metadata
- **WHEN** an entry's file has status `new`, `not_found`, or `ambiguous` and a captured raw tag snapshot
- **THEN** its metadata summary SHALL show the raw title/artist/album, visually distinguished (e.g. styled or labeled differently) from a resolved-metadata summary shown for an `identified` entry

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes, regardless of the active grouping or presentation

#### Scenario: Entries can be selected for bulk identification
- **WHEN** a user selects one or more entries, in any grouping or presentation
- **THEN** the UI SHALL enable an "Identify Selected" action that, when triggered, submits the selected files for identification

#### Scenario: Identify action disabled while an identify job is running
- **WHEN** an identification job is currently running
- **THEN** the UI's identify action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Entries can be selected for bulk enrichment
- **WHEN** a user selects one or more entries, in any grouping or presentation
- **THEN** the UI SHALL enable an "Enrich Selected" action that, when triggered, submits the selected files for cover art and lyrics enrichment

#### Scenario: Enrich action disabled while an enrich job is running
- **WHEN** an enrichment job is currently running
- **THEN** the UI's enrich action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Entries can be selected for bulk tagging
- **WHEN** a user selects one or more entries, in any grouping or presentation
- **THEN** the UI SHALL enable a "Tag Selected" action that, when triggered, submits the selected files for tag writing

#### Scenario: Tag action disabled while a tag job is running
- **WHEN** a tag job is currently running
- **THEN** the UI's tag action SHALL be disabled and SHALL display progress, re-enabling only once the job completes

#### Scenario: Entries can be selected for bulk relocation
- **WHEN** a user selects one or more entries, in any grouping or presentation
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

#### Scenario: Filtering, searching, and sorting
- **WHEN** a user sets a status/tagged/relocated/has-lyrics/has-cover-art filter, enters search text, or (in table presentation) clicks a sortable column header
- **THEN** the UI SHALL re-fetch the active grouping's data with the corresponding query parameters and re-render the current page of matching, sorted results

#### Scenario: Navigating between pages
- **WHEN** a user changes page size or navigates to another page within a grouping
- **THEN** the UI SHALL re-fetch that grouping's data with the corresponding `limit`/`offset` and replace the currently rendered entries with that page's results, regardless of which presentation is active

#### Scenario: Selecting all entries matching the current filter, not just the current page
- **WHEN** a user chooses "select all matching" while a filter and/or search is active
- **THEN** the UI SHALL treat the selection as "every file matching the current filter" (potentially far more than the current page's entries) rather than only the entries currently rendered, and SHALL visibly distinguish this from an explicit, page-scoped selection

#### Scenario: Bulk actions submit the active selection mode
- **WHEN** a bulk action (identify, enrich, tag, or relocate) is triggered
- **THEN** the UI SHALL submit either the explicit set of selected paths, or the active filter criteria if "select all matching" is active, matching whichever selection mode is currently shown to the user

#### Scenario: Large-selection notice before identifying
- **WHEN** a user triggers "Identify Selected" over a selection larger than a small threshold
- **THEN** the UI SHALL show an estimated completion time computed from the selection size and MusicBrainz's 1 request/second pace, before or as part of starting the job

#### Scenario: Deleting a missing file's tracked entry
- **WHEN** a user triggers the delete action on an entry with status `missing`
- **THEN** the UI SHALL prompt for confirmation before calling the delete endpoint, and SHALL remove that entry from the current listing once the deletion succeeds

#### Scenario: Delete action is not available for non-missing files
- **WHEN** an entry's status is not `missing`
- **THEN** the UI SHALL NOT offer a delete action for that entry

#### Scenario: Filtering by lyrics outcome in the UI
- **WHEN** a user sets the lyrics filter to "missing lyrics"
- **THEN** the UI SHALL re-fetch the active grouping's data with `has_lyrics=false` and re-render accordingly

#### Scenario: Filtering by cover art outcome in the UI
- **WHEN** a user sets the cover art filter to "missing cover"
- **THEN** the UI SHALL re-fetch the active grouping's data with `has_cover_art=false` and re-render accordingly

#### Scenario: Ambiguous entries are visually distinguished
- **WHEN** an entry's status is `ambiguous`
- **THEN** the UI SHALL show that status with its own distinct label/indicator, separate from `identified` and `not_found`

#### Scenario: Resolving an ambiguous file from the details view
- **WHEN** a user opens the details view for an entry with status `ambiguous`
- **THEN** the UI SHALL fetch and display that file's stored candidates and SHALL let the user choose one, calling the resolve endpoint and reflecting the entry as `identified` with the chosen candidate's metadata once resolution succeeds

#### Scenario: Resolve action is not available for non-ambiguous files
- **WHEN** an entry's status is not `ambiguous`
- **THEN** the UI SHALL NOT offer a candidate-resolve action for that entry

#### Scenario: Browsing alternate covers from the details view
- **WHEN** a user opens the details view for an `identified` entry and triggers "browse other covers"
- **THEN** the UI SHALL fetch and display cover-art candidates across that file's release-group's sibling editions, and SHALL let the user choose one, calling the choose endpoint and reflecting the entry's cover art with the chosen image once the choice succeeds

#### Scenario: Cover-browsing action is not available for unidentified files
- **WHEN** an entry's status is not `identified`
- **THEN** the UI SHALL NOT offer a cover-browsing action for that entry

#### Scenario: Details view shows the raw tag snapshot for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `ambiguous` and a captured raw tag snapshot
- **THEN** the UI SHALL display the raw title/artist/album/album-artist fields, labeled as embedded-in-file data rather than resolved metadata

#### Scenario: Manual search is available for any entry
- **WHEN** a user opens the details view for a tracked file, regardless of its current status
- **THEN** the UI SHALL offer a manual search control accepting free-text (or artist/title/album) input

#### Scenario: Manual search results use the existing candidate picker
- **WHEN** a manual search returns one or more candidates
- **THEN** the UI SHALL render them using the same candidate-list/"Use this" component already used for ambiguous AcoustID results, and choosing one SHALL call the existing resolve endpoint

#### Scenario: Manual search on an already-identified file warns before discarding its resolved metadata
- **WHEN** a user triggers a manual search for a file whose status is currently `identified`
- **THEN** the UI SHALL prompt for confirmation before submitting the search, since submitting it discards the file's current resolved metadata and stored candidates immediately

#### Scenario: Manual search with no results leaves the file's entry unchanged
- **WHEN** a manual search returns zero candidates
- **THEN** the UI SHALL indicate no matches were found and SHALL NOT alter the displayed entry's status or metadata

#### Scenario: Grouping and presentation are switched independently
- **WHEN** a user changes the active grouping (All / Folder / Artist-Album) or the active presentation (Table / Grid)
- **THEN** only the changed one SHALL take effect — the other SHALL remain exactly as it was, and the active filter, search, sort, and selection SHALL all be preserved

#### Scenario: Presentation applies within Folder and (at the track level) Artist-Album groupings
- **WHEN** a user switches presentation to Grid while browsing the Folder grouping, or while viewing an album's track listing in the Artist-Album grouping
- **THEN** that grouping's current file/track listing SHALL render as cover-forward cards instead of table rows, without changing the active drill-down position, filter, search, sort, or selection

#### Scenario: Presentation toggle is not offered where there is no file listing
- **WHEN** a user is browsing the Artist-Album grouping's artists or albums level (not yet drilled into a specific album's tracks)
- **THEN** the UI SHALL NOT offer a presentation toggle at that level, and SHALL continue showing artist/album cards regardless of the stored presentation preference

#### Scenario: Pagination is shared across presentations within a grouping
- **WHEN** a user navigates to another page while browsing a grouping's file listing
- **THEN** that page position SHALL apply regardless of which presentation is currently active, and switching presentation afterward SHALL NOT change or reset the current page

#### Scenario: Metadata-completeness indicator names exactly what's missing
- **WHEN** an identified entry is missing one or more of artist, album, title, or track number
- **THEN** the UI SHALL show a warning indicator on that entry's metadata summary whose tooltip names exactly which of the four fields are missing, rather than a generic warning

#### Scenario: Metadata-completeness indicator is not shown for unidentified entries
- **WHEN** an entry's status is not `identified`
- **THEN** the UI SHALL NOT show a metadata-completeness indicator for that entry

#### Scenario: Canonical-path indicator reflects the entry's relocated state
- **WHEN** an identified, tagged entry's current path already matches its computed relocation destination (whether reached via an explicit relocate action or detected passively)
- **THEN** the UI SHALL show a check indicator alongside that entry's path

#### Scenario: Canonical-path indicator surfaces a failed relocation attempt
- **WHEN** an entry has a recorded relocation failure
- **THEN** the UI SHALL show a warning indicator alongside that entry's path, with the failure reason available as a tooltip

#### Scenario: Canonical-path indicator is absent when not applicable
- **WHEN** an entry has neither been relocated (or passively detected as already at its destination) nor had a relocation attempt fail
- **THEN** the UI SHALL show neither indicator alongside that entry's path

#### Scenario: An empty listing shows an explicit "no items" indicator
- **WHEN** a grouping's current file/track listing has zero entries, whether because the library itself is empty, the active filter matches nothing, or the current folder/album has no files
- **THEN** the UI SHALL show an explicit "no items" message in place of the row/card list, rather than an empty table header or blank grid area

### Requirement: Toggling the table/grid view to show only the current selection
The system SHALL expose a `POST /api/v1/library/selection` endpoint accepting the same request body shape as `POST /api/v1/library/identify` (`paths` or `filter`), plus the same `sort`/`order`/`limit`/`offset` query parameters as `GET /api/v1/library`, and returning a page of matching entries in the same shape as `GET /api/v1/library`'s response. The system SHALL provide a "show selected only" toggle, available in the table and grid views whenever one or more files are explicitly selected, that — while enabled — fetches that view's rows/cards from the selection endpoint instead of `GET /api/v1/library`, using that view's own current sort and pagination state, so the existing row/card checkboxes (rather than a separate control) serve as the way to remove a file from the selection. The toggle SHALL be unavailable (or a no-op) while "select all matching" (filter mode) is active, since the currently filtered listing already is the selection in that mode.

#### Scenario: Enabling the toggle in explicit selection mode
- **WHEN** a user has explicitly selected one or more files (whether or not all are on the current page) and enables "show selected only" in the table or grid view
- **THEN** the UI SHALL replace that view's rows/cards with a paginated listing of exactly those files, fetched via the selection endpoint's `paths` request, honoring the view's current sort and page size

#### Scenario: Toggle unavailable in filter-mode selection
- **WHEN** the current selection is "all matching" (filter mode)
- **THEN** the UI SHALL NOT offer the "show selected only" toggle (or SHALL treat it as a no-op), since the currently displayed filtered listing already is the selection

#### Scenario: Unchecking a row while the toggle is enabled removes it from the selection
- **WHEN** a user unchecks a row's checkbox while "show selected only" is enabled
- **THEN** that file SHALL no longer be selected, the selection banner's count SHALL reflect the removal immediately, and the row SHALL be excluded from the view the next time its page is (re)fetched

#### Scenario: Disabling the toggle restores the normal filtered listing
- **WHEN** a user disables "show selected only"
- **THEN** the view SHALL return to fetching via `GET /api/v1/library` under the currently active filter, unaffected by having viewed the selection

#### Scenario: The toggle is available in both table and grid views
- **WHEN** a user switches between table and grid view while "show selected only" is enabled
- **THEN** the newly active view SHALL also fetch via the selection endpoint, preserving the toggle state across the view switch

#### Scenario: An explicit path list narrows a read to exactly those paths
- **WHEN** `POST /api/v1/library/selection` is given a non-empty `paths` list
- **THEN** the response SHALL include exactly the tracked entries at those paths, regardless of whether they'd match any other filter field, and SHALL NOT apply any other filter field

### Requirement: Asynchronous refresh action
The system SHALL expose a `POST /api/v1/library/scan` endpoint that starts the disk walk and tracking-store update (per the `file-tracking-store` capability) in the background and returns immediately, rather than blocking for the duration of the refresh. A refresh SHALL NOT fingerprint any file — fingerprinting happens lazily during identification (see the `file-tracking-store` capability's "Fingerprint computed lazily during identification" requirement).

#### Scenario: Refresh accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/scan` while no refresh is running
- **THEN** the response SHALL be `202 Accepted` and the walk/duration-read/update SHALL proceed asynchronously, without the HTTP request blocking until it finishes

#### Scenario: Per-file duration-read failure does not abort the refresh
- **WHEN** one file in `/music` fails to have its duration read during a refresh (e.g. a corrupt audio file)
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
- **THEN** the response SHALL indicate that a refresh is running and SHALL include how many of the files needing a duration read have been processed so far

#### Scenario: Idle status when no refresh is running
- **WHEN** a client queries refresh status while no refresh is in progress
- **THEN** the response SHALL indicate that no refresh is currently running

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

### Requirement: Identification progress is observable
The system SHALL expose a `GET /api/v1/library/identify/status` endpoint reporting whether an identify job is currently running and its progress.

#### Scenario: Progress reported while running
- **WHEN** a client queries identify status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no identify job is running
- **WHEN** a client queries identify status while no identify job is in progress
- **THEN** the response SHALL indicate that no identify job is currently running

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

### Requirement: On-demand manual search action
The system SHALL expose a `POST /api/v1/library/identify/search` endpoint accepting a tracked file's path and a free-text query, which searches MusicBrainz directly (per the `musicbrainz-metadata` capability's free-text recording search, independent of any audio fingerprint) and records the results as that file's candidates (per the `file-tracking-store` capability), responding synchronously with the resulting candidate list.

#### Scenario: Search with results
- **WHEN** a client issues `POST /api/v1/library/identify/search` with a path and a query that matches one or more MusicBrainz recordings
- **THEN** the response SHALL be `200 OK` with the resulting candidate list, and the file's status SHALL become `ambiguous`

#### Scenario: Search with no results
- **WHEN** a client issues `POST /api/v1/library/identify/search` with a path and a query that matches no MusicBrainz recordings
- **THEN** the response SHALL be `200 OK` with an empty candidate list, and the file's prior status and metadata SHALL remain unchanged

#### Scenario: Search for an untracked path
- **WHEN** a client issues `POST /api/v1/library/identify/search` with a path that is not tracked
- **THEN** the response SHALL be `404 Not Found`

#### Scenario: Search request failure
- **WHEN** the underlying MusicBrainz search request fails
- **THEN** the response SHALL indicate a server error distinguishable from "no matches found", and the file's prior status and metadata SHALL remain unchanged

### Requirement: Per-file details view
The system SHALL allow a user to open a details view for a single tracked file, showing its complete resolved record — path, format, duration, status, any fingerprint error, (once identified) artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and (once enriched) its cover art and lyrics, and (once tagged) the file's actual embedded tags read live from disk, shown alongside the resolved metadata for visual comparison. Opening this view SHALL NOT require any request beyond the already-fetched `GET /api/v1/library` data, except for the fingerprint, cover art image, lyrics text, and embedded tags themselves, each fetched on demand.

#### Scenario: Opening details for a row
- **WHEN** a user clicks a row (other than its selection checkbox)
- **THEN** the UI SHALL display that file's complete resolved record without issuing any new network request beyond loading its fingerprint, cover art image, lyrics, and embedded tags, if present

#### Scenario: Selecting a row does not open its details
- **WHEN** a user clicks a row's selection checkbox
- **THEN** the UI SHALL toggle that row's selection state and SHALL NOT open the details view

#### Scenario: Details view for an unidentified file
- **WHEN** a user opens the details view for a file with status `new`, `not_found`, or `missing`
- **THEN** the UI SHALL show the fields available for that status (path, format, duration, status, and any fingerprint error) without fabricating placeholder metadata values

#### Scenario: Details view fetches the fingerprint on open
- **WHEN** a user opens the details view for any tracked file
- **THEN** the UI SHALL call `GET /api/v1/library/fingerprint` for that file's path and render the returned fingerprint

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
The system SHALL expose a `POST /api/v1/library/enrich` endpoint accepting either a list of one or more file paths, or a filter (the same shape accepted by identify), which starts a background job resolving each matching already-identified path's cover art via Cover Art Archive and lyrics via LRCLIB (per the `cover-art-lookup` and `lyrics-lookup` capabilities) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Enrich job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/enrich` with one or more paths while no enrich job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Enrich job accepted with a filter instead of explicit paths
- **WHEN** a client issues `POST /api/v1/library/enrich` with a filter instead of an explicit path list
- **THEN** the system SHALL resolve every currently-tracked file matching that filter into a path list and process it the same as an explicitly-submitted list

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

### Requirement: Lyrics retrieval via API
The system SHALL expose a `GET /api/v1/library/lyrics` endpoint that, given a tracked file's path, returns that file's stored plain and synced lyrics as JSON.

#### Scenario: Lyrics available
- **WHEN** a client requests lyrics for a file with stored lyrics
- **THEN** the response SHALL be `200 OK` with a JSON body containing the plain lyrics and, when available, synced lyrics

#### Scenario: No lyrics stored
- **WHEN** a client requests lyrics for a file with no stored lyrics
- **THEN** the response SHALL be `404 Not Found`

### Requirement: On-demand tagging action
The system SHALL expose a `POST /api/v1/library/tag` endpoint accepting either a list of one or more file paths, or a filter (the same shape accepted by identify), which starts a background job writing each matching already-identified path's resolved metadata, cover art, and lyrics into the physical audio file's own tags (per the `audio-tag-writing` capability) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Tag job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/tag` with one or more paths while no tag job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Tag job accepted with a filter instead of explicit paths
- **WHEN** a client issues `POST /api/v1/library/tag` with a filter instead of an explicit path list
- **THEN** the system SHALL resolve every currently-tracked file matching that filter into a path list and process it the same as an explicitly-submitted list

#### Scenario: Trigger rejects an empty path list and an empty filter
- **WHEN** a client issues `POST /api/v1/library/tag` with an empty or missing paths list and no filter
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
The system SHALL expose a `POST /api/v1/library/relocate` endpoint accepting either a list of one or more file paths, or a filter (the same shape accepted by identify), which starts a background job physically relocating each matching already-identified-and-tagged path into the canonical directory hierarchy (per the `file-relocation` capability) and returns immediately rather than blocking for the duration of the job.

#### Scenario: Relocate job accepted and runs in the background
- **WHEN** a client issues `POST /api/v1/library/relocate` with one or more paths while no relocate job is running
- **THEN** the response SHALL be `202 Accepted` and the paths SHALL be processed asynchronously, without the HTTP request blocking until the job finishes

#### Scenario: Relocate job accepted with a filter instead of explicit paths
- **WHEN** a client issues `POST /api/v1/library/relocate` with a filter instead of an explicit path list
- **THEN** the system SHALL resolve every currently-tracked file matching that filter into a path list and process it the same as an explicitly-submitted list

#### Scenario: Trigger rejects an empty path list and an empty filter
- **WHEN** a client issues `POST /api/v1/library/relocate` with an empty or missing paths list and no filter
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
The system SHALL expose a `GET /api/v1/library/relocate/status` endpoint reporting whether a relocate job is currently running, its progress, and the old-path-to-new-path mapping for every file the job has successfully relocated so far in the current (or most recently completed) job — so a client that tracks a selection by path can update stale paths once a file moves. The system SHALL provide a UI that, on each relocate-status poll, updates any currently-selected path found in that mapping to its reported new path, so a relocated file remains selected under its new path rather than silently dropping out of the selection.

#### Scenario: Progress reported while running
- **WHEN** a client queries relocate status while a job is in progress
- **THEN** the response SHALL indicate that a job is running and SHALL include how many of the submitted paths have been processed so far

#### Scenario: Idle status when no relocate job is running
- **WHEN** a client queries relocate status while no relocate job is in progress
- **THEN** the response SHALL indicate that no relocate job is currently running

#### Scenario: Relocated paths are reported in the status response
- **WHEN** a relocate job successfully moves one or more files
- **THEN** the status response SHALL include, for each successfully relocated file, its path before and after the move

#### Scenario: A selected file relocated mid-job remains selected under its new path
- **WHEN** a file that is part of the current explicit selection is successfully relocated while a relocate job runs
- **THEN** the UI SHALL, on its next status poll, remove the file's old path from the selection and add its new path, keeping the selection count and contents accurate without user action

#### Scenario: Non-relocated files' selection is unaffected
- **WHEN** a relocate job completes
- **THEN** paths not reported in the job's relocation mapping SHALL remain selected or unselected exactly as they were before the job ran

### Requirement: Embedded tag retrieval via API
The system SHALL expose a `GET /api/v1/library/tags` endpoint that, given a tracked file's path, reads that file's actual embedded tags directly from disk (per the `audio-tag-writing` capability) and returns them as JSON, independent of the resolved metadata cached in the tracking store.

#### Scenario: Embedded tags available
- **WHEN** a client requests embedded tags for a tracked file
- **THEN** the response SHALL be `200 OK` with a JSON body containing that file's actual embedded title, artist, album, album artist, track number, disc number, year, and whether lyrics and cover art are embedded

#### Scenario: File not found on disk
- **WHEN** a client requests embedded tags for a tracked path that is currently missing from disk
- **THEN** the response SHALL be `404 Not Found`

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

### Requirement: Deletion of missing tracked files
The system SHALL expose a way to delete a tracked file's record from the tracking store, allowed only when that file's effective status is `missing`. Deleting a tracked record SHALL NOT delete any cover art file on disk, since cover art may be shared across multiple tracks on the same release.

#### Scenario: Deleting a missing file succeeds
- **WHEN** a client requests deletion of a tracked file whose effective status is `missing`
- **THEN** the system SHALL remove that file's tracking record and respond successfully

#### Scenario: Deleting a non-missing file is rejected
- **WHEN** a client requests deletion of a tracked file whose effective status is not `missing`
- **THEN** the system SHALL reject the request with `409 Conflict` and SHALL NOT remove the tracking record

#### Scenario: Deleting an unknown path
- **WHEN** a client requests deletion of a path that is not tracked
- **THEN** the system SHALL respond `404 Not Found`
