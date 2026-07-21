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
