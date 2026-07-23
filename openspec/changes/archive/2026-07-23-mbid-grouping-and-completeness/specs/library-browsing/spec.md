## MODIFIED Requirements

### Requirement: Artist and album browsing via API
The system SHALL expose `GET /api/v1/library/artists`, `GET /api/v1/library/albums`, and `GET /api/v1/library/tracks` endpoints for browsing the tracked library grouped by artist and album. Grouping SHALL key on a tracked file's `artist_mbid` (for artists) and `release_group_mbid` scoped within the artist group (for albums) when identified, falling back to its resolved name, then its raw tag snapshot's name, then a distinguished "unknown" bucket, when no MBID is available — so unidentified files are not excluded from this view. Each artist/album grouping SHALL expose a stable `artist_key`/`album_key` (the MBID when present, otherwise a name-derived key) alongside its display name, and `GET /api/v1/library/albums` and `GET /api/v1/library/tracks` SHALL accept `artist_key`/`album_key` query parameters to select a group unambiguously; the previously-supported `artist`/`album` name query parameters SHALL continue to be accepted for backward compatibility, resolved to a key the same way grouping does.

#### Scenario: Listing artists
- **WHEN** a client issues `GET /api/v1/library/artists`
- **THEN** the response SHALL include every distinct artist grouping present in the library, each with its `artist_key`, display name, and total track count

#### Scenario: Listing albums for an artist by key
- **WHEN** a client issues `GET /api/v1/library/albums?artist_key=<key>` for an artist grouping with one or more albums
- **THEN** the response SHALL include every distinct album grouping for that artist, each with its `album_key`, display name, and track count

#### Scenario: Listing tracks for an artist and album by key
- **WHEN** a client issues `GET /api/v1/library/tracks?artist_key=<key>&album_key=<key>`
- **THEN** the response SHALL include that album's tracks, sorted by track number, in the same shape as `GET /api/v1/library`'s entries

#### Scenario: Listing albums or tracks by name remains supported
- **WHEN** a client issues `GET /api/v1/library/albums?artist=<name>` or `GET /api/v1/library/tracks?artist=<name>&album=<name>` without a key parameter
- **THEN** the system SHALL resolve the name to a grouping key the same way artist/album grouping does, and return the same result as the equivalent key-based request

#### Scenario: Two different artists grouped by MBID even when they share a name string
- **WHEN** two tracked files have the same resolved artist name but different `artist_mbid` values
- **THEN** they SHALL appear as two distinct artist groupings, each with its own `artist_key`

#### Scenario: Same artist grouped together despite differing name strings
- **WHEN** two tracked files have the same `artist_mbid` but different resolved artist name strings
- **THEN** they SHALL appear under a single artist grouping keyed by that `artist_mbid`

#### Scenario: Unidentified files with raw tags appear grouped by their raw artist/album
- **WHEN** a tracked file has no resolved metadata but does have a captured raw tag snapshot
- **THEN** it SHALL appear under a name-derived grouping key for its raw artist/album, rather than being omitted

#### Scenario: Files with neither resolved nor raw metadata are grouped as unknown
- **WHEN** a tracked file has neither resolved metadata nor a captured raw tag snapshot
- **THEN** it SHALL be grouped under a distinguished "unknown artist"/"unknown album" bucket rather than omitted or erroring

#### Scenario: Filters and search apply to artist/album/track listings
- **WHEN** any of these three endpoints is issued with a `status`, `tagged`, `relocated`, `has_lyrics`, or `has_cover_art` filter
- **THEN** the returned artists, albums, or tracks SHALL reflect only tracked files matching that filter

## ADDED Requirements

### Requirement: Mismatch flagging in artist/album grouping
The system SHALL flag an artist or album grouping when the grouping decision papers over a disagreement in the underlying data, rather than silently resolving it: (a) **name mismatch**, when a group's files share the same MBID but disagree on the resolved/raw name string; and (b) **label collision**, when two distinct groupings (different `artist_key`s or `album_key`s) resolve to the same display label. Both flags SHALL be included in the `GET /api/v1/library/artists` and `GET /api/v1/library/albums` responses.

#### Scenario: Name mismatch within an MBID-keyed group
- **WHEN** an artist grouping's files share one `artist_mbid` but at least two distinct resolved or raw artist name strings
- **THEN** that grouping's response entry SHALL include `name_mismatch: true` along with the distinct names observed

#### Scenario: Label collision across two groupings
- **WHEN** two artist (or album, within the same artist) groupings with different keys resolve to the same display label (case-insensitive)
- **THEN** both groupings' response entries SHALL include `label_collision: true`

#### Scenario: No false positives for a clean group
- **WHEN** an artist or album grouping's files all agree on name and its label doesn't collide with any other grouping's
- **THEN** neither `name_mismatch` nor `label_collision` SHALL be set on that grouping

#### Scenario: Unidentified groupings can only collide, never mismatch
- **WHEN** a grouping has no MBID (name-derived key)
- **THEN** `name_mismatch` SHALL never be set on it, though `label_collision` still may be

### Requirement: MusicBrainz completeness check for an artist
Given an artist grouping with a non-empty `artist_mbid`, the system SHALL expose an endpoint that compares the artist's release-groups already present in the library against that artist's full MusicBrainz discography (official Album/EP release-groups only), returning the counts of albums owned vs. total, and the list of albums (title, year) not present in the library. The check SHALL be performed only on request for a single artist, never eagerly across a list of artists.

#### Scenario: Artist with some albums missing
- **WHEN** a client requests a completeness check for an artist grouping with a valid `artist_mbid`
- **THEN** the response SHALL include the count of albums present locally, the total official Album/EP release-groups for that artist on MusicBrainz, and the titles/years of the ones not present locally

#### Scenario: Artist grouping has no MBID
- **WHEN** a client requests a completeness check for a name-derived (unidentified) artist grouping
- **THEN** the system SHALL return an error or empty result indicating the check is unavailable, rather than attempting a MusicBrainz lookup

#### Scenario: MusicBrainz request failure
- **WHEN** the underlying MusicBrainz request fails
- **THEN** the system SHALL return an error distinct from "artist fully complete," so the caller can distinguish "checked, nothing missing" from "check failed"

### Requirement: MusicBrainz completeness check for an album
Given an album grouping with a non-empty `release_group_mbid`, the system SHALL expose an endpoint that compares the album's tracks already present in the library (matched by `recording_mbid`) against that release's full MusicBrainz tracklist, returning the counts of tracks owned vs. total, and the list of tracks (title, track number) not present in the library. The check SHALL be performed only on request for a single album, never eagerly across a list of albums.

#### Scenario: Album with some tracks missing
- **WHEN** a client requests a completeness check for an album grouping with a valid `release_group_mbid`
- **THEN** the response SHALL include the count of tracks present locally, the total tracks on the resolved release, and the titles/track numbers of the ones not present locally

#### Scenario: Album grouping has no MBID
- **WHEN** a client requests a completeness check for a name-derived (unidentified) album grouping
- **THEN** the system SHALL return an error or empty result indicating the check is unavailable, rather than attempting a MusicBrainz lookup

#### Scenario: MusicBrainz request failure
- **WHEN** the underlying MusicBrainz request fails
- **THEN** the system SHALL return an error distinct from "album fully complete," so the caller can distinguish "checked, nothing missing" from "check failed"
