## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: No additional MusicBrainz requests for extended metadata
The system SHALL derive album artist, release year, disc number, total discs, total tracks, and the release/release-group/artist MBIDs from the same recording lookup response used for existing metadata resolution, without issuing any additional MusicBrainz request.

#### Scenario: Extended metadata resolved without extra requests
- **WHEN** a recording is resolved successfully
- **THEN** the system SHALL NOT issue any MusicBrainz request beyond the single recording lookup already required for artist/album/title/track number
