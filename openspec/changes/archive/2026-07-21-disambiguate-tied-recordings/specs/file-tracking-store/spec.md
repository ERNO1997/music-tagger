## MODIFIED Requirements

### Requirement: Persistent per-file tracking record
The system SHALL persist one record per discovered audio file — path, format, fingerprint, size, modification time, identification status (`new`, `identified`, `not_found`, `ambiguous`, `missing`), once identified, resolved artist, album artist, title, track number, release year, disc number, total discs, total tracks, and MusicBrainz recording/release/release-group/artist IDs, and, once enriched, a cover art file path and plain/synced lyrics, once tagging has been attempted, a tagged outcome (whether the on-disk file was successfully tagged, and any tagging error), and, once relocation has been attempted, a relocated outcome (whether the file was successfully moved, and any relocation error) — in an embedded SQLite database that survives process restarts. A file's path is the record's identifying key but is not immutable: relocation updates it to the file's new physical location while preserving every other field on that same record.

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

#### Scenario: Stored candidates survive a restart
- **WHEN** the server process is restarted after a file has been recorded `ambiguous`
- **THEN** that file's stored candidate list SHALL still be retrievable without re-running identification

### Requirement: Identification results are recorded per file
The system SHALL update a tracked file's record with the outcome of an identification attempt, without altering its size or modification time. A tracked file's fingerprint is set separately, as part of identification when one isn't already stored (see "Fingerprint computed lazily during identification") — identification's own outcome recording never alters a fingerprint that identification itself just set or reused.

#### Scenario: Identification succeeds
- **WHEN** identification resolves a single, unambiguous match for a tracked file
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
- **WHEN** a previously enriched tracked file (with stored cover art and/or lyrics) is identified again, whether it resolves to `identified`, `not_found`, or `ambiguous`
- **THEN** the system SHALL clear its previously stored cover art path and lyrics, since they were resolved against a possibly-different prior identity and are not guaranteed to still apply

#### Scenario: Re-identification invalidates prior tagged outcome
- **WHEN** a previously tagged tracked file is identified again, whether it resolves to `identified`, `not_found`, or `ambiguous`
- **THEN** the system SHALL clear its stored tagged outcome, since the on-disk file's tags were written against a possibly-different prior identity and must be re-tagged to reflect the new one

#### Scenario: Re-identification invalidates prior relocated outcome
- **WHEN** a previously relocated tracked file is identified again, whether it resolves to `identified`, `not_found`, or `ambiguous`
- **THEN** the system SHALL clear its stored relocated outcome, since the file's location was resolved against a possibly-different prior identity, without moving the file back or altering its currently-tracked path

#### Scenario: Re-identification clears a stale candidate list
- **WHEN** a tracked file that previously had a stored candidate list (from a prior `ambiguous` outcome) is identified again, whether it resolves to `identified`, `not_found`, or `ambiguous` again
- **THEN** the system SHALL discard its previous candidate list, since candidates resolved against the file's old content or a prior tied result must never be shown or resolved against under its new identification attempt

### Requirement: Filtered, sorted, and paginated reads
The system SHALL support reading tracked records filtered by effective status, tagged outcome, relocated outcome, and/or lyrics outcome; searched by a case-insensitive substring match against path, artist, album, and title; sorted by an allow-listed set of columns (path, status, artist, album, duration, year) in ascending or descending order with a deterministic tie-break so repeated reads against unchanged data return the same order; and paginated by a result limit and offset — reporting the total number of matching records independent of the page size. This is distinct from the full, unfiltered table load used internally for scan change-detection, which is unaffected by this requirement.

#### Scenario: Filtering narrows the result set
- **WHEN** a read is requested with a status, tagged, relocated, or has-lyrics filter
- **THEN** only records matching that filter SHALL be included, and the reported total SHALL reflect only the matching count

#### Scenario: Filtering by the ambiguous status
- **WHEN** a read is requested with a status filter of `ambiguous`
- **THEN** only records whose effective status is `ambiguous` SHALL be included

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

## ADDED Requirements

### Requirement: Ambiguous identification is recorded with candidate metadata
The system SHALL treat an AcoustID lookup whose accepted (at-or-above-confidence-threshold) top result ties two or more recordings that resolve to distinct artist/title identities as needing human disambiguation rather than picking one automatically: it SHALL resolve each tied recording's canonical metadata, store the full set as that file's candidates, and set the file's status to `ambiguous` without writing any single resolved-metadata field to the file's own record.

#### Scenario: Tied recordings resolving to distinct identities are recorded as ambiguous
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that resolve to two or more distinct (artist, title) identities
- **THEN** the system SHALL set that file's status to `ambiguous`, store every distinct resolved candidate, and SHALL NOT write resolved metadata to the file's own record

#### Scenario: Tied recordings resolving to the same identity are recorded as a normal success
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that all resolve to the same (artist, title) identity
- **THEN** the system SHALL set that file's status to `identified` and record that shared identity's resolved metadata, exactly as if AcoustID had returned only one recording

#### Scenario: An ambiguous file's candidates are retrievable
- **WHEN** a file has been recorded `ambiguous`
- **THEN** the system SHALL make its full stored candidate list (each candidate's resolved artist, album, title, track number, and other metadata) available for retrieval

### Requirement: A stored candidate can be chosen to resolve an ambiguous file
The system SHALL allow a stored candidate to be selected for a tracked file whose status is `ambiguous`, recording that choice exactly as a normal successful identification and discarding the file's other stored candidates.

#### Scenario: Choosing a valid candidate resolves the file
- **WHEN** a candidate matching one of an `ambiguous` file's stored recording IDs is chosen
- **THEN** the system SHALL set that file's status to `identified`, store the chosen candidate's resolved metadata exactly as a normal successful identification would, and discard its other stored candidates

#### Scenario: Choosing an unrecognized candidate is rejected
- **WHEN** a candidate recording ID is submitted for a file that does not have a stored candidate with that ID
- **THEN** the system SHALL leave that file's status and stored candidates unchanged and SHALL report that the requested candidate was not found
