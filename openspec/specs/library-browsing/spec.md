## Purpose

Alternative ways to browse the tracked music library beyond the flat, paginated table/grid: a folder tree reflecting the mounted `/music` volume's actual on-disk directory structure, and an Artist-Album grouping derived from resolved (or raw-tag-fallback) metadata. Both are read-only views layered over the same tracking-store data exposed by the `music-library-scan` capability's `GET /api/v1/library` endpoint.

## Requirements

### Requirement: Folder tree browsing via API
The system SHALL expose a `GET /api/v1/library/tree` endpoint that, given an optional `path` prefix (defaulting to the music root), returns the immediate subdirectories under that prefix (each with its total tracked file count and identified-file count) and the tracked files directly at that level (in the same shape and accepting the same filter/sort/pagination query parameters as `GET /api/v1/library`), reflecting the mounted `/music` volume's actual on-disk directory structure.

#### Scenario: Browsing the root
- **WHEN** a client issues `GET /api/v1/library/tree` with no `path` parameter
- **THEN** the response SHALL include the immediate subdirectories of the music root, each with its total and identified file counts, and any tracked files directly at the root level

#### Scenario: Browsing a subdirectory
- **WHEN** a client issues `GET /api/v1/library/tree?path=<subdirectory>` for a subdirectory containing tracked files
- **THEN** the response SHALL include that subdirectory's own immediate subdirectories and its directly-contained tracked files

#### Scenario: Filters and search apply within the current directory level
- **WHEN** a client issues `GET /api/v1/library/tree` with a `status`, `tagged`, `relocated`, `has_lyrics`, `has_cover_art`, or `q` parameter
- **THEN** the returned subdirectory counts and direct files SHALL reflect only tracked files matching that filter

#### Scenario: An empty or non-existent prefix returns an empty result
- **WHEN** a client issues `GET /api/v1/library/tree?path=<prefix>` for a prefix with no tracked files under it
- **THEN** the response SHALL be `200 OK` with no subdirectories and no files, rather than an error

### Requirement: Artist and album browsing via API
The system SHALL expose `GET /api/v1/library/artists`, `GET /api/v1/library/albums?artist=<name>`, and `GET /api/v1/library/tracks?artist=<name>&album=<name>` endpoints for browsing the tracked library grouped by artist and album. Grouping SHALL use a tracked file's resolved artist/album when identified, falling back to its raw tag snapshot's artist/album when not, so unidentified files are not excluded from this view; a file with neither is grouped under a distinguished "unknown" bucket.

#### Scenario: Listing artists
- **WHEN** a client issues `GET /api/v1/library/artists`
- **THEN** the response SHALL include every distinct artist name (resolved or raw-tag-derived) present in the library, each with its total track count

#### Scenario: Listing albums for an artist
- **WHEN** a client issues `GET /api/v1/library/albums?artist=<name>` for an artist with one or more albums
- **THEN** the response SHALL include every distinct album for that artist, each with its track count

#### Scenario: Listing tracks for an artist and album
- **WHEN** a client issues `GET /api/v1/library/tracks?artist=<name>&album=<name>`
- **THEN** the response SHALL include that album's tracks, sorted by track number, in the same shape as `GET /api/v1/library`'s entries

#### Scenario: Unidentified files with raw tags appear grouped by their raw artist/album
- **WHEN** a tracked file has no resolved metadata but does have a captured raw tag snapshot
- **THEN** it SHALL appear under its raw artist/album in the artist/album listings, rather than being omitted

#### Scenario: Files with neither resolved nor raw metadata are grouped as unknown
- **WHEN** a tracked file has neither resolved metadata nor a captured raw tag snapshot
- **THEN** it SHALL be grouped under a distinguished "unknown artist"/"unknown album" bucket rather than omitted or erroring

#### Scenario: Filters and search apply to artist/album/track listings
- **WHEN** any of these three endpoints is issued with a `status`, `tagged`, `relocated`, `has_lyrics`, or `has_cover_art` filter
- **THEN** the returned artists, albums, or tracks SHALL reflect only tracked files matching that filter

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
