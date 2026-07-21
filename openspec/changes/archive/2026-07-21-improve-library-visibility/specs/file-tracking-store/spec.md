## MODIFIED Requirements

### Requirement: Persistent per-file tracking record
The system SHALL persist one record per discovered audio file — path, format, fingerprint, size, modification time, identification status (`new`, `identified`, `not_found`, `ambiguous`, `missing`), a raw tag snapshot (title, artist, album, album artist as embedded in the file itself, independent of resolved metadata), once identified, resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and, once enriched, a cover art file path and plain/synced lyrics, once tagging has been attempted, a tagged outcome (whether the on-disk file was successfully tagged, and any tagging error), and, once relocation has been attempted, a relocated outcome (whether the file was successfully moved, and any relocation error) — in an embedded SQLite database that survives process restarts. A file's path is the record's identifying key but is not immutable: relocation updates it to the file's new physical location while preserving every other field on that same record.

#### Scenario: Tracking data survives a restart
- **WHEN** the server process is restarted after files have been tracked
- **THEN** previously tracked files and their status SHALL still be retrievable without re-scanning `/music`

#### Scenario: Resolved metadata survives a restart
- **WHEN** the server process is restarted after a file has been identified
- **THEN** that file's resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz IDs SHALL still be retrievable without re-running identification

#### Scenario: Raw tag snapshot survives a restart
- **WHEN** the server process is restarted after a file has been scanned
- **THEN** that file's raw tag snapshot (title, artist, album, album artist as embedded in the file) SHALL still be retrievable without re-scanning, if it was captured

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

#### Scenario: Stored candidates survive a restart
- **WHEN** the server process is restarted after a file has been recorded `ambiguous`
- **THEN** that file's stored candidate list SHALL still be retrievable without re-running identification

### Requirement: Change detection on refresh
The system SHALL classify each file discovered during a refresh into exactly one of: newly discovered, changed since last seen, or unchanged since last seen, using size and modification time as the change signal. A refresh SHALL NOT compute a Chromaprint fingerprint for any file — fingerprinting happens lazily during identification instead. A new or changed file's raw tag snapshot SHALL be (re-)captured from the file's own embedded tags during the same refresh, independent of and without altering resolved metadata.

#### Scenario: New file discovered
- **WHEN** a refresh finds a file with no existing tracking record
- **THEN** the system SHALL insert a new record with status `new`, its duration read from the file's own audio properties, its raw tag snapshot read from the file's own embedded tags, and no fingerprint

#### Scenario: Changed file's duration and raw tags are re-read and its stale fingerprint is cleared
- **WHEN** a refresh finds a tracked file whose size or modification time differs from its stored record
- **THEN** the system SHALL re-read its duration and raw tag snapshot, clear any previously stored fingerprint and fingerprint error, update the stored record, and reset its status to `new`

#### Scenario: Unchanged file's duration and raw tags are not re-read
- **WHEN** a refresh finds a tracked file whose size and modification time match its stored record
- **THEN** the system SHALL skip reading its duration and raw tags again and leave its stored status, fingerprint, duration, and raw tag snapshot unchanged

#### Scenario: A successfully tagged file is not seen as changed by a later scan
- **WHEN** a refresh runs after a file has been successfully tagged (which changes the file's own size and modification time on disk)
- **THEN** the system SHALL treat that file as unchanged, since tagging already updated the stored size and modification time to match — the file's identification status, resolved metadata, fingerprint, and raw tag snapshot SHALL NOT be reset

#### Scenario: Raw tag read failure does not abort the refresh or block the duration read
- **WHEN** a new or changed file's raw tag snapshot fails to be read during a refresh, independent of whether its duration read succeeds
- **THEN** the system SHALL leave that file's raw tag fields blank, SHALL still record whatever duration was successfully read, and the refresh SHALL continue processing the remaining files

### Requirement: Filtered, sorted, and paginated reads
The system SHALL support reading tracked records filtered by effective status, tagged outcome, relocated outcome, lyrics outcome, and/or cover art outcome; searched by a case-insensitive substring match against path, artist, album, title, and raw title/artist/album; sorted by an allow-listed set of columns (path, status, artist, album, duration, year) in ascending or descending order with a deterministic tie-break so repeated reads against unchanged data return the same order; and paginated by a result limit and offset — reporting the total number of matching records independent of the page size. This is distinct from the full, unfiltered table load used internally for scan change-detection, which is unaffected by this requirement.

#### Scenario: Filtering narrows the result set
- **WHEN** a read is requested with a status, tagged, relocated, has-lyrics, or has-cover-art filter
- **THEN** only records matching that filter SHALL be included, and the reported total SHALL reflect only the matching count

#### Scenario: Filtering by the ambiguous status
- **WHEN** a read is requested with a status filter of `ambiguous`
- **THEN** only records whose effective status is `ambiguous` SHALL be included

#### Scenario: Search matches raw tags for unidentified files
- **WHEN** a read is requested with a search term matching a tracked file's raw title, artist, or album, but that file has no resolved metadata yet
- **THEN** that file SHALL be included in the result set

#### Scenario: Search matches across multiple fields
- **WHEN** a read is requested with a search term
- **THEN** records whose path, artist, album, title, raw title, raw artist, or raw album contains that term, case-insensitively, SHALL be included, and records matching none of those fields SHALL be excluded

#### Scenario: Sorting is stable under concurrent writes
- **WHEN** a read is requested with a sort column and a background job is concurrently modifying tracked records
- **THEN** the returned order SHALL be deterministic for any given snapshot of the data, using a stable tie-break so records are not silently duplicated or skipped across repeated reads purely due to sort-key ties

#### Scenario: Pagination reports the total independent of page size
- **WHEN** a read is requested with a limit and offset
- **THEN** the number of records returned SHALL be at most the limit, and the reported total SHALL reflect the full count of matching records, not the count on the current page

#### Scenario: Resolving a filter to a bare path list
- **WHEN** the full set of paths matching a filter is requested, without pagination
- **THEN** the system SHALL return every currently-matching path, ignoring any limit or offset, for use in resolving a bulk action's filter-based selection at the moment it executes
