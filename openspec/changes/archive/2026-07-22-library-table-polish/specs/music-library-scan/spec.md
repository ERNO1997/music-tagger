## MODIFIED Requirements

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
