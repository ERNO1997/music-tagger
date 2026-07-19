## MODIFIED Requirements

### Requirement: Persistent per-file tracking record
The system SHALL persist one record per discovered audio file — path, format, fingerprint, size, modification time, identification status (`new`, `identified`, `not_found`, `missing`), once identified, resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and, once enriched, a cover art file path and plain/synced lyrics, once tagging has been attempted, a tagged outcome (whether the on-disk file was successfully tagged, and any tagging error), and, once relocation has been attempted, a relocated outcome (whether the file was successfully moved, and any relocation error) — in an embedded SQLite database that survives process restarts. A file's path is the record's identifying key but is not immutable: relocation updates it to the file's new physical location while preserving every other field on that same record.

#### Scenario: Tracking data survives a restart
- **WHEN** the server process is restarted after files have been tracked
- **THEN** previously tracked files and their status SHALL still be retrievable without re-scanning `/music`

#### Scenario: Resolved metadata survives a restart
- **WHEN** the server process is restarted after a file has been identified
- **THEN** that file's resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz IDs SHALL still be retrievable without re-running identification

#### Scenario: Cover art path survives a restart
- **WHEN** the server process is restarted after a file has been enriched with cover art
- **THEN** that file's cover art path SHALL still be retrievable without re-running enrichment

#### Scenario: Lyrics survive a restart
- **WHEN** the server process is restarted after a file has been enriched with lyrics
- **THEN** that file's plain and synced lyrics SHALL still be retrievable without re-running enrichment

#### Scenario: Tagged outcome survives a restart
- **WHEN** the server process is restarted after a file has been tagged
- **THEN** that file's tagged outcome SHALL still be retrievable without re-running tagging

#### Scenario: Relocated outcome survives a restart
- **WHEN** the server process is restarted after a file has been relocated
- **THEN** that file's relocated outcome and its current (post-relocation) path SHALL still be retrievable without re-running relocation

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

#### Scenario: Re-identification invalidates prior enrichment
- **WHEN** a previously enriched tracked file (with stored cover art and/or lyrics) is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its previously stored cover art path and lyrics, since they were resolved against a possibly-different prior identity and are not guaranteed to still apply

#### Scenario: Re-identification invalidates prior tagged outcome
- **WHEN** a previously tagged tracked file is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its stored tagged outcome, since the on-disk file's tags were written against a possibly-different prior identity and must be re-tagged to reflect the new one

#### Scenario: Re-identification invalidates prior relocated outcome
- **WHEN** a previously relocated tracked file is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its stored relocated outcome, since the file's location was resolved against a possibly-different prior identity, without moving the file back or altering its currently-tracked path

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

#### Scenario: A successfully tagged file is not seen as changed by a later scan
- **WHEN** a refresh runs after a file has been successfully tagged (which changes the file's own size and modification time on disk)
- **THEN** the system SHALL treat that file as unchanged, since tagging already updated the stored size and modification time to match — the file's identification status and resolved metadata SHALL NOT be reset

### Requirement: Tagging results are recorded per file
The system SHALL update a tracked file's record with the outcome of a tag-writing attempt. On success, the system SHALL also update the stored size and modification time to match the file's actual state after writing (since writing tags changes the file itself), without altering its fingerprint, identification status, resolved metadata, cover art path, or lyrics.

#### Scenario: Tagging succeeds
- **WHEN** tag writing completes successfully for a tracked file
- **THEN** the system SHALL mark that file as tagged, clear any previously stored tagging error, and update the stored size and modification time to the file's current, post-write values

#### Scenario: Tagging fails
- **WHEN** tag writing fails for a tracked file (e.g. an unwritable or malformed file)
- **THEN** the system SHALL leave that file's tagged flag as not-tagged and store the failure reason, without aborting tagging of the rest of the batch, and SHALL NOT alter the stored size or modification time

#### Scenario: Tagging attempted on an unidentified file
- **WHEN** tagging is requested for a file that is not yet `identified`
- **THEN** the system SHALL skip that file without recording a tagging outcome for it and without aborting tagging of the rest of the batch

## ADDED Requirements

### Requirement: Relocation results are recorded per file
The system SHALL update a tracked file's record with the outcome of a relocation attempt, without altering its fingerprint, identification status, resolved metadata, cover art path, lyrics, or tagged outcome. On a successful relocation, the record's path SHALL be updated to the file's new location as part of the same update.

#### Scenario: Relocation succeeds
- **WHEN** a file is successfully moved to its computed destination path
- **THEN** the system SHALL update that file's tracking record's path to the new location, mark it as relocated, and clear any previously stored relocation error

#### Scenario: Relocation fails
- **WHEN** relocation fails for a tracked file (e.g. a destination collision or filesystem error)
- **THEN** the system SHALL leave that file's tracked path unchanged, leave its relocated flag as not-relocated, and store the failure reason, without aborting relocation of the rest of the batch

#### Scenario: Relocation attempted on a file that is not both identified and tagged
- **WHEN** relocation is requested for a file that is not yet `identified`, or is identified but not yet tagged
- **THEN** the system SHALL skip that file without recording a relocation outcome for it and without aborting relocation of the rest of the batch
