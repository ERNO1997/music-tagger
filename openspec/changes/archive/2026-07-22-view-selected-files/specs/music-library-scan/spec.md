## ADDED Requirements

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

## MODIFIED Requirements

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
