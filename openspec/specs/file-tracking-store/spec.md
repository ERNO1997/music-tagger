## Purpose

Persistent, per-file tracking of the mounted `/music` volume's discovery and identification state — surviving restarts and distinguishing new, changed, unchanged, missing, identified, and enriched files — so that scanning, identification, and enrichment don't need to re-derive this from scratch on every request.

## Requirements

### Requirement: Persistent per-file tracking record
The system SHALL persist one record per discovered audio file — path, format, fingerprint, size, modification time, identification status (`new`, `identified`, `not_found`, `missing`), once identified, resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and, once enriched, a cover art file path — in an embedded SQLite database that survives process restarts.

#### Scenario: Tracking data survives a restart
- **WHEN** the server process is restarted after files have been tracked
- **THEN** previously tracked files and their status SHALL still be retrievable without re-scanning `/music`

#### Scenario: Resolved metadata survives a restart
- **WHEN** the server process is restarted after a file has been identified
- **THEN** that file's resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz IDs SHALL still be retrievable without re-running identification

#### Scenario: Cover art path survives a restart
- **WHEN** the server process is restarted after a file has been enriched with cover art
- **THEN** that file's cover art path SHALL still be retrievable without re-running enrichment

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

### Requirement: Identification results are recorded per file
The system SHALL update a tracked file's record with the outcome of an identification attempt, without altering its fingerprint, size, or modification time.

#### Scenario: Identification succeeds
- **WHEN** identification resolves a match for a tracked file
- **THEN** the system SHALL set that file's status to `identified` and store its resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs

#### Scenario: Identification finds no match
- **WHEN** identification finds no AcoustID match for a tracked file
- **THEN** the system SHALL set that file's status to `not_found` and SHALL NOT write any resolved metadata fields

#### Scenario: Identification fails due to a gateway error
- **WHEN** an AcoustID or MusicBrainz request fails during identification of a tracked file
- **THEN** the system SHALL leave that file's status and metadata unchanged and SHALL surface the error separately from the file's tracked state

#### Scenario: Missing release year does not block identification
- **WHEN** identification resolves a match whose release has no usable date
- **THEN** the system SHALL still record the file as `identified` with its other resolved fields, leaving the release year unset

### Requirement: Enrichment results are recorded per file
The system SHALL update a tracked file's record with the outcome of a cover art enrichment attempt, without altering its fingerprint, identification status, or resolved metadata.

#### Scenario: Cover art found and stored
- **WHEN** enrichment resolves a front cover image for a tracked file's release
- **THEN** the system SHALL store the downloaded image's file path on that file's tracking record

#### Scenario: No cover art available
- **WHEN** enrichment finds no cover art for a tracked file's release
- **THEN** the system SHALL leave that file's cover art path empty without treating this as an error

#### Scenario: Enrichment attempted on an unidentified file
- **WHEN** enrichment is requested for a file that is not yet `identified`
- **THEN** the system SHALL skip that file (no Release MBID is available to look up) without aborting enrichment of the rest of the batch

#### Scenario: Shared cover art across tracks on the same release
- **WHEN** two tracked files resolve to the same release during enrichment
- **THEN** the system SHALL reuse the same stored cover art file rather than downloading and storing a duplicate
