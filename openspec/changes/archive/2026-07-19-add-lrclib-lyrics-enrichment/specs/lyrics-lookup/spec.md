## ADDED Requirements

### Requirement: Lyrics resolution via LRCLIB
Given a resolved artist, track title, album, and duration, the system SHALL query LRCLIB for that track's lyrics and return the plain-text lyrics and, when available, LRC-timed synced lyrics.

#### Scenario: Lyrics found
- **WHEN** LRCLIB returns a match for the given artist/title/album/duration
- **THEN** the system SHALL return that match's plain lyrics and, if present, its synced lyrics

#### Scenario: Duration improves match precision
- **WHEN** a duration is provided alongside artist and title
- **THEN** the system SHALL pass it to LRCLIB to prefer the most precisely matching entry over an entry resolved without duration

#### Scenario: Track not found
- **WHEN** LRCLIB returns a not-found response for the given artist/title/album/duration
- **THEN** the system SHALL treat this as "no lyrics available", not an error, and SHALL NOT alter the file's identification status

#### Scenario: Instrumental track
- **WHEN** LRCLIB returns a match marked as instrumental
- **THEN** the system SHALL treat this identically to "no lyrics available"

#### Scenario: LRCLIB request failure
- **WHEN** the LRCLIB request fails for a reason other than "not found" (network error, non-404 non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from "no lyrics available"
