## ADDED Requirements

### Requirement: Release-group sibling release resolution
Given a MusicBrainz Release-Group ID, the system SHALL resolve its sibling releases (release ID, title, status, and date for each), for use in browsing alternate cover art across a release-group's editions. This request SHALL be subject to the same centralized MusicBrainz rate limit as recording metadata resolution.

#### Scenario: Successful resolution
- **WHEN** a Release-Group ID resolves to one or more releases
- **THEN** the system SHALL return each release's ID, title, status, and date

#### Scenario: MusicBrainz request failure
- **WHEN** the MusicBrainz request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error, and SHALL NOT treat the failure as "no releases found"
