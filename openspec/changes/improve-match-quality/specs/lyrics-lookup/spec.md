## MODIFIED Requirements

### Requirement: Lyrics resolution via LRCLIB
Given a resolved artist, track title, album, and duration, the system SHALL query LRCLIB for that track's lyrics and return the plain-text lyrics and, when available, LRC-timed synced lyrics. If an exact-match lookup finds no result, the system SHALL fall back to a fuzzy, ranked search by artist and title before concluding no lyrics are available.

#### Scenario: Lyrics found
- **WHEN** LRCLIB returns a match for the given artist/title/album/duration
- **THEN** the system SHALL return that match's plain lyrics and, if present, its synced lyrics

#### Scenario: Duration improves match precision
- **WHEN** a duration is provided alongside artist and title
- **THEN** the system SHALL pass it to LRCLIB to prefer the most precisely matching entry over an entry resolved without duration

#### Scenario: Exact match not found falls back to fuzzy search
- **WHEN** an exact-match lookup by artist/title/album/duration finds no result
- **THEN** the system SHALL retry via a fuzzy search by artist and title, and if that search returns one or more candidates, SHALL select the one whose duration is closest to the given duration and return its plain and synced lyrics

#### Scenario: Fuzzy search also finds nothing
- **WHEN** both the exact-match lookup and the fuzzy-search fallback find no result
- **THEN** the system SHALL treat this as "no lyrics available", not an error, and SHALL NOT alter the file's identification status

#### Scenario: Instrumental track
- **WHEN** either the exact-match lookup or the fuzzy-search fallback returns a match marked as instrumental
- **THEN** the system SHALL treat this identically to "no lyrics available"

#### Scenario: LRCLIB request failure
- **WHEN** a LRCLIB request (exact-match or fuzzy-search) fails for a reason other than "not found" (network error, non-404 non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from "no lyrics available"
