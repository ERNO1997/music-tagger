## ADDED Requirements

### Requirement: Low-confidence AcoustID matches are not accepted
The system SHALL treat an AcoustID lookup whose best-scoring match falls below a minimum confidence threshold the same as no match at all, recording the file as `not_found` rather than `identified` and writing no resolved metadata — preferring no metadata over metadata resolved from an unreliable match.

#### Scenario: Best match below the confidence threshold is treated as not found
- **WHEN** identification's AcoustID lookup returns one or more matches, but the best-scoring match is below the minimum confidence threshold
- **THEN** the system SHALL set that file's status to `not_found` and SHALL NOT call MusicBrainz or write any resolved metadata fields

#### Scenario: Best match at or above the confidence threshold is accepted
- **WHEN** identification's AcoustID lookup returns a best-scoring match at or above the minimum confidence threshold
- **THEN** the system SHALL proceed to resolve and record that match's metadata as it does today
