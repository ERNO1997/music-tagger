## Purpose

Resolving a MusicBrainz Recording ID (found via the `acoustid-lookup` capability) to canonical artist/release/track/track-number data, with the 1 request/second MusicBrainz rate limit enforced centrally regardless of caller.

## Requirements

### Requirement: Recording metadata resolution
Given a MusicBrainz Recording ID, the system SHALL resolve the canonical artist, release (album) title, track title, and track number by querying the MusicBrainz recording endpoint with associated releases and artist credits.

#### Scenario: Successful resolution
- **WHEN** a Recording ID resolves to at least one release
- **THEN** the system SHALL return the artist, release title, track title, and track number derived from that release

#### Scenario: Recording has multiple associated releases
- **WHEN** a Recording ID is associated with more than one release
- **THEN** the system SHALL prefer a release whose release-group primary type is "Album" and status is "Official", falling back to the first release returned if none qualifies

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
