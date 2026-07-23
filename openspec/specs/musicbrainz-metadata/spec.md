## Purpose

Resolving a MusicBrainz Recording ID (found via the `acoustid-lookup` capability) to canonical artist/release/track/track-number data — plus extended metadata (album artist, release year, disc/track counts, and MBIDs) derived from the same lookup — with the 1 request/second MusicBrainz rate limit enforced centrally regardless of caller.

## Requirements

### Requirement: Recording metadata resolution
Given a MusicBrainz Recording ID, the system SHALL resolve the canonical artist, release (album) title, track title, track number, album artist, release year, disc number, total discs, total tracks, release MBID, release-group MBID, and artist MBID by querying the MusicBrainz recording endpoint with associated releases and artist credits.

#### Scenario: Successful resolution
- **WHEN** a Recording ID resolves to at least one release
- **THEN** the system SHALL return the artist, release title, track title, track number, album artist, release year, disc number, total discs, total tracks, release MBID, release-group MBID, and artist MBID derived from that release

#### Scenario: Recording has multiple associated releases
- **WHEN** a Recording ID is associated with more than one release
- **THEN** the system SHALL prefer a release whose release-group primary type is "Album" and status is "Official", falling back to the first release returned if none qualifies

#### Scenario: Release date has partial precision
- **WHEN** the selected release's date is year-only, year-and-month, or a full date
- **THEN** the system SHALL derive the release year from any of these formats, and SHALL leave the year unset rather than erroring if no date is present

#### Scenario: Album artist differs from track artist
- **WHEN** the selected release's artist-credit differs from the recording's own artist-credit (e.g. a various-artists compilation)
- **THEN** the system SHALL report the release's artist-credit as the album artist without altering the track-level artist

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no releases found"

### Requirement: Centralized MusicBrainz rate limiting
The system SHALL enforce a minimum 1-second interval between requests to MusicBrainz, applied once inside the client itself, regardless of which caller issues the request.

#### Scenario: Two requests issued within the same second
- **WHEN** two calls to the MusicBrainz client are made less than 1 second apart, from the same or different callers
- **THEN** the second request SHALL be delayed until at least 1 second has elapsed since the first

#### Scenario: No concurrent fan-out to MusicBrainz
- **WHEN** multiple callers issue MusicBrainz requests concurrently
- **THEN** the requests SHALL be serialized through the shared rate gate rather than issued in parallel

### Requirement: No additional MusicBrainz requests for extended metadata
The system SHALL derive album artist, release year, disc number, total discs, total tracks, and the release/release-group/artist MBIDs from the same recording lookup response used for existing metadata resolution, without issuing any additional MusicBrainz request.

#### Scenario: Extended metadata resolved without extra requests
- **WHEN** a recording is resolved successfully
- **THEN** the system SHALL NOT issue any MusicBrainz request beyond the single recording lookup already required for artist/album/title/track number

### Requirement: Release-group sibling release resolution
Given a MusicBrainz Release-Group ID, the system SHALL resolve its sibling releases (release ID, title, status, and date for each), for use in browsing alternate cover art across a release-group's editions. This request SHALL be subject to the same centralized MusicBrainz rate limit as recording metadata resolution.

#### Scenario: Successful resolution
- **WHEN** a Release-Group ID resolves to one or more releases
- **THEN** the system SHALL return each release's ID, title, status, and date

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no releases found"

### Requirement: Free-text recording search
Given a free-text query, the system SHALL resolve it directly to candidate MusicBrainz recordings — each with the same canonical artist/release/track/extended metadata as a recording-ID lookup — without requiring an AcoustID fingerprint match. This request SHALL be subject to the same centralized MusicBrainz rate limit as recording and release-group lookups.

#### Scenario: Successful search
- **WHEN** a free-text query matches one or more MusicBrainz recordings
- **THEN** the system SHALL return each matching recording's resolved artist, release (album) title, track title, track number, album artist, release year, disc number, total discs, total tracks, release MBID, release-group MBID, and artist MBID, ranked by MusicBrainz's own relevance order

#### Scenario: No matches found
- **WHEN** a free-text query matches no MusicBrainz recordings
- **THEN** the system SHALL return an empty result, distinct from an error

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz search request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no matches found"

### Requirement: Artist discography resolution
Given a MusicBrainz Artist ID, the system SHALL resolve that artist's release-groups (album title, release-group MBID, and first-release year), filtered to official primary-type Album or EP release-groups excluding Compilation/Live/Remix/Soundtrack/DJ-mix/Mixtape secondary types. This request SHALL be subject to the same centralized MusicBrainz rate limit as recording and release-group lookups, and SHALL transparently follow pagination if the artist's discography spans more than one page, up to a bounded page limit.

#### Scenario: Successful resolution
- **WHEN** an Artist ID resolves to one or more qualifying release-groups
- **THEN** the system SHALL return each release-group's title, MBID, and first-release year

#### Scenario: Non-qualifying release-groups are excluded
- **WHEN** an artist's discography includes release-groups that are not official Album/EP primary type, or carry an excluded secondary type
- **THEN** those release-groups SHALL NOT be included in the result

#### Scenario: Discography spans multiple pages
- **WHEN** an artist's qualifying release-group count exceeds a single MusicBrainz response page
- **THEN** the system SHALL fetch subsequent pages (each subject to the rate limit) until exhausted or a bounded page limit is reached, and SHALL indicate if the result may be incomplete due to that limit

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no release-groups found"

### Requirement: Release tracklist resolution
Given a MusicBrainz Release ID, the system SHALL resolve that release's full tracklist (each track's recording MBID, title, and track number). This request SHALL be subject to the same centralized MusicBrainz rate limit as recording, release-group, and artist discography lookups.

#### Scenario: Successful resolution
- **WHEN** a Release ID resolves successfully
- **THEN** the system SHALL return every track's recording MBID, title, and track number across all media on that release

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no tracks found"
