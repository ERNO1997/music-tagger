## Purpose

Resolving a local file's acoustic fingerprint to candidate MusicBrainz Recording IDs via the AcoustID API — the first step of on-demand identification, ahead of resolving canonical metadata via the `musicbrainz-metadata` capability.

## Requirements

### Requirement: Fingerprint resolution via AcoustID
The system SHALL submit a file's fingerprint and duration to the AcoustID API and return the resulting MusicBrainz Recording ID(s), ranked by match score.

#### Scenario: Match found
- **WHEN** AcoustID returns one or more scored matches for a fingerprint
- **THEN** the system SHALL return the matched Recording ID(s) ordered by descending score

#### Scenario: No match found
- **WHEN** AcoustID returns zero matches for a fingerprint
- **THEN** the system SHALL return an empty result, distinct from an error, so the caller can record the file as `not_found` rather than failed

#### Scenario: AcoustID request failure
- **WHEN** the AcoustID request fails (network error, non-2xx response, or malformed response)
- **THEN** the system SHALL return an error distinguishable from a no-match result, and SHALL NOT record the file as `not_found`
