## MODIFIED Requirements

### Requirement: Persistent per-file tracking record
The system SHALL persist one record per discovered audio file — path, format, fingerprint, size, modification time, identification status (`new`, `identified`, `not_found`, `missing`), and, once identified, resolved artist, album, title, track number, and MusicBrainz Recording ID — in an embedded SQLite database that survives process restarts.

#### Scenario: Tracking data survives a restart
- **WHEN** the server process is restarted after files have been tracked
- **THEN** previously tracked files and their status SHALL still be retrievable without re-scanning `/music`

#### Scenario: Resolved metadata survives a restart
- **WHEN** the server process is restarted after a file has been identified
- **THEN** that file's resolved artist, album, title, and track number SHALL still be retrievable without re-running identification

### Requirement: Change detection on refresh
The system SHALL classify each file discovered during a refresh into exactly one of: newly discovered, changed since last seen, or unchanged since last seen, using size and modification time as the change signal.

#### Scenario: New file discovered
- **WHEN** a refresh finds a file with no existing tracking record
- **THEN** the system SHALL insert a new record with status `new` and a freshly computed fingerprint

#### Scenario: Changed file is re-fingerprinted
- **WHEN** a refresh finds a tracked file whose size or modification time differs from its stored record
- **THEN** the system SHALL recompute its fingerprint, update the stored record, and reset its status to `new`

#### Scenario: Unchanged file is not re-fingerprinted
- **WHEN** a refresh finds a tracked file whose size and modification time match its stored record
- **THEN** the system SHALL skip fingerprinting for that file and leave its stored status unchanged

### Requirement: Missing files are preserved, not deleted
The system SHALL mark a previously tracked file as `missing` when it is no longer found on disk during a refresh, without deleting its tracking record.

#### Scenario: Tracked file no longer on disk
- **WHEN** a refresh does not find a file on disk that has an existing tracking record
- **THEN** the system SHALL set that record's status to `missing` and SHALL preserve its last-known fingerprint, size, and modification time

#### Scenario: Missing file reappears unchanged
- **WHEN** a refresh finds a file at a path previously marked `missing`, with size and modification time matching the preserved record
- **THEN** the system SHALL treat it as unchanged and restore it to its prior (pre-`missing`) status rather than treating it as a new file

## ADDED Requirements

### Requirement: Identification results are recorded per file
The system SHALL update a tracked file's record with the outcome of an identification attempt, without altering its fingerprint, size, or modification time.

#### Scenario: Identification succeeds
- **WHEN** identification resolves a match for a tracked file
- **THEN** the system SHALL set that file's status to `identified` and store its resolved artist, album, title, track number, and MusicBrainz Recording ID

#### Scenario: Identification finds no match
- **WHEN** identification finds no AcoustID match for a tracked file
- **THEN** the system SHALL set that file's status to `not_found` and SHALL NOT write any resolved metadata fields

#### Scenario: Identification fails due to a gateway error
- **WHEN** an AcoustID or MusicBrainz request fails during identification of a tracked file
- **THEN** the system SHALL leave that file's status and metadata unchanged and SHALL surface the error separately from the file's tracked state
