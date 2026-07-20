## Purpose

Persistent, per-file tracking of the mounted `/music` volume's discovery and identification state — surviving restarts and distinguishing new, changed, unchanged, missing, identified, and enriched files — so that scanning, identification, and enrichment don't need to re-derive this from scratch on every request.

## Requirements

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

#### Scenario: Re-identification invalidates prior enrichment
- **WHEN** a previously enriched tracked file (with stored cover art and/or lyrics) is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its previously stored cover art path and lyrics, since they were resolved against a possibly-different prior identity and are not guaranteed to still apply

#### Scenario: Re-identification invalidates prior tagged outcome
- **WHEN** a previously tagged tracked file is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its stored tagged outcome, since the on-disk file's tags were written against a possibly-different prior identity and must be re-tagged to reflect the new one

#### Scenario: Re-identification invalidates prior relocated outcome
- **WHEN** a previously relocated tracked file is identified again, whether it resolves to `identified` or `not_found`
- **THEN** the system SHALL clear its stored relocated outcome, since the file's location was resolved against a possibly-different prior identity, without moving the file back or altering its currently-tracked path

### Requirement: Enrichment results are recorded per file
The system SHALL update a tracked file's record with the outcome of a cover art and lyrics enrichment attempt, without altering its fingerprint, identification status, or resolved metadata.

#### Scenario: Cover art found and stored
- **WHEN** enrichment resolves a front cover image for a tracked file's release
- **THEN** the system SHALL store the downloaded image's file path on that file's tracking record

#### Scenario: No cover art available
- **WHEN** enrichment finds no cover art for a tracked file's release
- **THEN** the system SHALL leave that file's cover art path empty without treating this as an error

#### Scenario: Lyrics found and stored
- **WHEN** enrichment resolves lyrics for a tracked file
- **THEN** the system SHALL store the plain lyrics, and synced lyrics when available, on that file's tracking record

#### Scenario: No lyrics available
- **WHEN** enrichment finds no lyrics for a tracked file (not found, or the track is instrumental)
- **THEN** the system SHALL leave that file's lyrics fields empty without treating this as an error

#### Scenario: Cover art and lyrics outcomes are independent
- **WHEN** enrichment succeeds for one of cover art or lyrics but fails or finds nothing for the other
- **THEN** the system SHALL record the successful outcome regardless of the other's result

#### Scenario: Enrichment attempted on an unidentified file
- **WHEN** enrichment is requested for a file that is not yet `identified`
- **THEN** the system SHALL skip that file (no resolved metadata is available to look up) without aborting enrichment of the rest of the batch

#### Scenario: Shared cover art across tracks on the same release
- **WHEN** two tracked files resolve to the same release during enrichment
- **THEN** the system SHALL reuse the same stored cover art file rather than downloading and storing a duplicate

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

### Requirement: Filtered, sorted, and paginated reads
The system SHALL support reading tracked records filtered by effective status, tagged outcome, and/or relocated outcome; searched by a case-insensitive substring match against path, artist, album, and title; sorted by an allow-listed set of columns (path, status, artist, album, duration, year) in ascending or descending order with a deterministic tie-break so repeated reads against unchanged data return the same order; and paginated by a result limit and offset — reporting the total number of matching records independent of the page size. This is distinct from the full, unfiltered table load used internally for scan change-detection, which is unaffected by this requirement.

#### Scenario: Filtering narrows the result set
- **WHEN** a read is requested with a status, tagged, or relocated filter
- **THEN** only records matching that filter SHALL be included, and the reported total SHALL reflect only the matching count

#### Scenario: Search matches across multiple fields
- **WHEN** a read is requested with a search term
- **THEN** records whose path, artist, album, or title contains that term, case-insensitively, SHALL be included, and records matching none of those fields SHALL be excluded

#### Scenario: Sorting is stable under concurrent writes
- **WHEN** a read is requested with a sort column and a background job is concurrently modifying tracked records
- **THEN** the returned order SHALL be deterministic for any given snapshot of the data, using a stable tie-break so records are not silently duplicated or skipped across repeated reads purely due to sort-key ties

#### Scenario: Pagination reports the total independent of page size
- **WHEN** a read is requested with a limit and offset
- **THEN** the number of records returned SHALL be at most the limit, and the reported total SHALL reflect the full count of matching records, not the count on the current page

#### Scenario: Resolving a filter to a bare path list
- **WHEN** the full set of paths matching a filter is requested, without pagination
- **THEN** the system SHALL return every currently-matching path, ignoring any limit or offset, for use in resolving a bulk action's filter-based selection at the moment it executes
