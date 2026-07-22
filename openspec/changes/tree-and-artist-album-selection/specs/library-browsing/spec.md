## ADDED Requirements

### Requirement: Selecting files in the folder tree and Artist-Album views
The system SHALL allow selecting one or more files directly from the folder tree view's file listing and the Artist-Album view's track listing, using the same selection state (explicit set, or "all matching the current filter") already shared across the table and grid views, so that selections made in — or bulk actions (identify/enrich/tag/relocate) triggered from — any of the four views apply consistently regardless of which view is currently active.

#### Scenario: Selecting a file in the folder tree
- **WHEN** a user checks a file's checkbox while browsing the folder tree view
- **THEN** that file SHALL be added to the current selection, and the selection banner SHALL reflect the updated count

#### Scenario: Selecting a track in the Artist-Album view
- **WHEN** a user checks a track's checkbox while viewing an album's track listing
- **THEN** that track SHALL be added to the current selection, and the selection banner SHALL reflect the updated count

#### Scenario: Selecting all files on the current folder page
- **WHEN** a user checks the "select all" header checkbox while browsing a folder's file listing
- **THEN** every file currently listed on that page SHALL be selected

#### Scenario: Selecting all tracks in the current album
- **WHEN** a user checks the "select all" header checkbox while viewing an album's track listing
- **THEN** every track in that album's listing SHALL be selected

#### Scenario: Selection persists when switching to or from these views
- **WHEN** a user has files selected and switches between the folder tree, Artist-Album, table, or grid views
- **THEN** the selection SHALL remain unchanged, and each view SHALL reflect it (checked rows where the file is visible, an accurate count in the selection banner regardless)

#### Scenario: Directory and artist/album cards are not individually selectable
- **WHEN** a user is browsing the folder tree's directory cards or the Artist-Album view's artist/album cards (not yet drilled into a file listing)
- **THEN** the system SHALL NOT offer a selection checkbox for those cards, since they represent groupings rather than individual tracked files
