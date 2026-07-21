## MODIFIED Requirements

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders one page of results, in the user's currently-selected view (table, grid, folder tree, or Artist-Album — see the `library-browsing` capability for the latter two), showing path, format, duration, identification status, a condensed resolved-metadata summary (or, when a file is not yet identified, its raw tag snapshot when captured, so a poorly-named file's actual title/artist is still visible), a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present. It SHALL reflect whether a refresh is currently running, allow selecting one or more rows (or all rows matching the current filter, across pages), provide bulk actions to identify, enrich, tag, and relocate the selected rows, provide a delete action for rows with status `missing`, provide a resolve action for rows with status `ambiguous`, provide a manual search action available from any row's details view regardless of status, and allow opening a full details view for any single row or card. It SHALL provide controls for filtering by status/tagged/relocated/has-lyrics/has-cover-art, free-text search, column sorting (in table view), page navigation, and switching between views.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL default to the table view, call `GET /api/v1/library`, and render one row per returned file for the current page, including its status, any resolved metadata, a cover art thumbnail when present, a lyrics indicator when present, a tagged indicator when present, and a relocated indicator when present

#### Scenario: An unidentified file's row shows its raw tag snapshot instead of blank metadata
- **WHEN** a table row's file has status `new`, `not_found`, or `ambiguous` and a captured raw tag snapshot
- **THEN** the row's metadata summary SHALL show the raw title/artist/album, visually distinguished (e.g. styled or labeled differently) from a resolved-metadata summary shown for an `identified` row

#### Scenario: Refresh trigger disabled while running
- **WHEN** a refresh is currently running (whether started by this user, another tab, or automatically at server startup)
- **THEN** the UI's refresh trigger control SHALL be disabled and SHALL display that a scan is in progress, re-enabling only once the refresh completes, regardless of which view is active

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
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with the corresponding query parameters and re-render the currently active view to reflect only the current page of matching, sorted results

#### Scenario: Navigating between pages
- **WHEN** a user changes page size or navigates to another page
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with the corresponding `limit`/`offset` and replace the currently rendered rows/cards with that page's results

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
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with `has_lyrics=false` and re-render the currently active view accordingly

#### Scenario: Filtering by cover art outcome in the UI
- **WHEN** a user sets the cover art filter to "missing cover"
- **THEN** the UI SHALL re-fetch `GET /api/v1/library` with `has_cover_art=false` and re-render the currently active view accordingly

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

#### Scenario: Manual search is available for any row
- **WHEN** a user opens the details view for a tracked file, regardless of its current status
- **THEN** the UI SHALL offer a manual search control accepting free-text (or artist/title/album) input

#### Scenario: Manual search results use the existing candidate picker
- **WHEN** a manual search returns one or more candidates
- **THEN** the UI SHALL render them using the same candidate-list/"Use this" component already used for ambiguous AcoustID results, and choosing one SHALL call the existing resolve endpoint

#### Scenario: Manual search on an already-identified file warns before discarding its resolved metadata
- **WHEN** a user triggers a manual search for a file whose status is currently `identified`
- **THEN** the UI SHALL prompt for confirmation before submitting the search, since submitting it discards the file's current resolved metadata and stored candidates immediately

#### Scenario: Manual search with no results leaves the file's row unchanged
- **WHEN** a manual search returns zero candidates
- **THEN** the UI SHALL indicate no matches were found and SHALL NOT alter the displayed row's status or metadata

#### Scenario: Switching to grid view
- **WHEN** a user selects the grid view
- **THEN** the UI SHALL render the same currently-fetched (and future) `GET /api/v1/library` results as cover-forward cards instead of table rows, without changing the active filter, search, sort, or selection

#### Scenario: Grid view supports the same actions as table view
- **WHEN** a user is in grid view
- **THEN** selection, bulk actions, and opening the details view for a card SHALL behave identically to their table-view equivalents

#### Scenario: Switching views preserves the active filter
- **WHEN** a user switches from one view to another while a status/tagged/relocated/has-lyrics/has-cover-art filter or search term is active
- **THEN** the newly active view SHALL reflect the same filter/search, not reset to an unfiltered view
