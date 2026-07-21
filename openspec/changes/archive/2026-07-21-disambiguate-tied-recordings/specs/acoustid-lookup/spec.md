## MODIFIED Requirements

### Requirement: Fingerprint resolution via AcoustID
The system SHALL submit a file's fingerprint and duration to the AcoustID API and return the resulting matches ranked by descending score, preserving which MusicBrainz Recording ID(s) are tied together under the same matching result — a single fingerprint can map to more than one distinct recording (e.g. a reissue or compilation reusing the same master), and callers need to tell that apart from an unambiguous single-recording match.

#### Scenario: Match found with a single recording
- **WHEN** AcoustID returns a scored result whose recordings list contains exactly one Recording ID
- **THEN** the system SHALL return that result with its single Recording ID, ordered among other results by descending score

#### Scenario: Match found with multiple tied recordings
- **WHEN** AcoustID returns a scored result whose recordings list contains more than one distinct Recording ID
- **THEN** the system SHALL return that result with every one of its tied Recording IDs grouped together, not flattened into separate same-scored entries indistinguishable from unrelated results

#### Scenario: No match found
- **WHEN** AcoustID returns zero matches for a fingerprint
- **THEN** the system SHALL return an empty result, distinct from an error, so the caller can record the file as `not_found` rather than failed

#### Scenario: AcoustID request failure
- **WHEN** the AcoustID request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from a no-match result, and SHALL NOT record the file as `not_found`
