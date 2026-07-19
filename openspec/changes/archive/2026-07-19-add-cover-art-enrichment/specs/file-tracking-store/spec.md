## MODIFIED Requirements

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

## ADDED Requirements

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
