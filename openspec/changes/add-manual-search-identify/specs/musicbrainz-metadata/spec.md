## ADDED Requirements

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
