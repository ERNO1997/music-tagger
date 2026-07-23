## ADDED Requirements

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
