## Purpose

Persistent, per-file tracking of the mounted `/music` volume's discovery and identification state — surviving restarts and distinguishing new, changed, unchanged, missing, identified, and enriched files — so that scanning, identification, and enrichment don't need to re-derive this from scratch on every request.

## Requirements

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
The system SHALL classify each file discovered during a refresh into exactly one of: newly discovered, changed since last seen, or unchanged since last seen, using size and modification time as the change signal. A refresh SHALL NOT compute a Chromaprint fingerprint for any file — fingerprinting happens lazily during identification instead (see "Fingerprint computed lazily during identification"). A new or changed file's raw tag snapshot SHALL be (re-)captured from the file's own embedded tags during the same refresh, independent of and without altering resolved metadata.

#### Scenario: New file discovered
- **WHEN** a refresh finds a file with no existing tracking record
- **THEN** the system SHALL insert a new record with status `new`, its duration read from the file's own audio properties, its raw tag snapshot read from the file's own embedded tags, and no fingerprint

#### Scenario: Changed file's duration and raw tags are re-read and its stale fingerprint is cleared
- **WHEN** a refresh finds a tracked file whose size or modification time differs from its stored record
- **THEN** the system SHALL re-read its duration and raw tag snapshot, clear any previously stored fingerprint and fingerprint error (since a fingerprint computed against the file's old content must never be reused against its new content), update the stored record, and reset its status to `new`

#### Scenario: Unchanged file's duration and raw tags are not re-read
- **WHEN** a refresh finds a tracked file whose size and modification time match its stored record
- **THEN** the system SHALL skip reading its duration and raw tags again and leave its stored status, fingerprint, duration, and raw tag snapshot unchanged

#### Scenario: A successfully tagged file is not seen as changed by a later scan
- **WHEN** a refresh runs after a file has been successfully tagged (which changes the file's own size and modification time on disk)
- **THEN** the system SHALL treat that file as unchanged, since tagging already updated the stored size and modification time to match — the file's identification status, resolved metadata, fingerprint, and raw tag snapshot SHALL NOT be reset

#### Scenario: Raw tag read failure does not abort the refresh or block the duration read
- **WHEN** a new or changed file's raw tag snapshot fails to be read during a refresh, independent of whether its duration read succeeds
- **THEN** the system SHALL leave that file's raw tag fields blank, SHALL still record whatever duration was successfully read, and the refresh SHALL continue processing the remaining files

### Requirement: Missing files are preserved, not deleted
The system SHALL mark a previously tracked file as `missing` when it is no longer found on disk during a refresh, without deleting its tracking record.

#### Scenario: Tracked file no longer on disk
- **WHEN** a refresh does not find a file on disk that has an existing tracking record
- **THEN** the system SHALL set that record's status to `missing` and SHALL preserve its last-known fingerprint, size, and modification time

#### Scenario: Missing file reappears unchanged
- **WHEN** a refresh finds a file at a path previously marked `missing`, with size and modification time matching the preserved record
- **THEN** the system SHALL treat it as unchanged and restore it to its prior (pre-`missing`) status rather than treating it as a new file

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

### Requirement: Low-confidence AcoustID matches are not accepted
The system SHALL treat an AcoustID lookup whose best-scoring match falls below a minimum confidence threshold the same as no match at all, recording the file as `not_found` rather than `identified` and writing no resolved metadata — preferring no metadata over metadata resolved from an unreliable match.

#### Scenario: Best match below the confidence threshold is treated as not found
- **WHEN** identification's AcoustID lookup returns one or more matches, but the best-scoring match is below the minimum confidence threshold
- **THEN** the system SHALL set that file's status to `not_found` and SHALL NOT call MusicBrainz or write any resolved metadata fields

#### Scenario: Best match at or above the confidence threshold is accepted
- **WHEN** identification's AcoustID lookup returns a best-scoring match at or above the minimum confidence threshold
- **THEN** the system SHALL proceed to resolve and record that match's metadata as it does today

### Requirement: Fingerprint computed lazily during identification
The system SHALL compute a tracked file's Chromaprint fingerprint the first time that file is submitted for identification with no fingerprint already stored, persist the result before proceeding to the AcoustID lookup, and reuse the stored fingerprint on any subsequent identification attempt rather than recomputing it, until the file's content changes.

#### Scenario: Fingerprint computed on first identify
- **WHEN** identification is requested for a tracked file that has no stored fingerprint
- **THEN** the system SHALL compute its fingerprint and duration, persist both, and proceed to look it up via AcoustID using the newly computed fingerprint

#### Scenario: Stored fingerprint is reused, not recomputed
- **WHEN** identification is requested for a tracked file that already has a stored fingerprint
- **THEN** the system SHALL use the stored fingerprint directly without recomputing it

#### Scenario: Fingerprint computation failure does not abort the identify job
- **WHEN** fingerprint computation fails for one file during an identify job (e.g. a corrupt or unreadable audio file)
- **THEN** the system SHALL record the failure reason on that file's tracked record, skip that file without treating its identification status as `not_found`, and continue processing the rest of the job

### Requirement: Enrichment results are recorded per file
The system SHALL update a tracked file's record with the outcome of a cover art and lyrics enrichment attempt, without altering its fingerprint, identification status, or resolved metadata. A tracked file's cover art path and/or lyrics MAY also be recorded by the `background-library-analysis` capability's automatic detection of a file's own embedded cover art/lyrics, using the same fields and subject to the same "leave existing data alone" rule described below — recording an outcome via either path SHALL NOT be overwritten by the other once set.

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

#### Scenario: Enrichment does not overwrite a cover art or lyrics value the background analysis pass already recorded
- **WHEN** enrichment resolves a cover image or lyrics for a file whose tracking record already has a cover art path or lyrics stored (whether from a prior enrichment or from automatic embedded-content detection)
- **THEN** the system SHALL leave the already-stored value unchanged rather than replacing it

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
The system SHALL update a tracked file's record with the outcome of a relocation attempt, without altering its fingerprint, identification status, resolved metadata, cover art path, lyrics, or tagged outcome. On a successful relocation, the record's path SHALL be updated to the file's new location as part of the same update. A tracked file MAY also be marked `relocated` by the `background-library-analysis` capability's automatic detection that its current path already equals its computed canonical destination, without any file having been moved.

#### Scenario: Relocation succeeds
- **WHEN** a file is successfully moved to its computed destination path
- **THEN** the system SHALL update that file's tracking record's path to the new location, mark it as relocated, and clear any previously stored relocation error

#### Scenario: Relocation fails
- **WHEN** relocation fails for a tracked file (e.g. a destination collision or filesystem error)
- **THEN** the system SHALL leave that file's tracked path unchanged, leave its relocated flag as not-relocated, and store the failure reason, without aborting relocation of the rest of the batch

#### Scenario: Relocation attempted on a file that is not both identified and tagged
- **WHEN** relocation is requested for a file that is not yet `identified`, or is identified but not yet tagged
- **THEN** the system SHALL skip that file without recording a relocation outcome for it and without aborting relocation of the rest of the batch

#### Scenario: A file passively detected as already relocated does not have its path changed
- **WHEN** the background analysis pass marks a file `relocated` because it found the file already at its canonical destination
- **THEN** the system SHALL mark it relocated without altering its tracked path, since the file was never moved

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

### Requirement: Ambiguous identification is recorded with candidate metadata
The system SHALL treat an AcoustID lookup whose accepted (at-or-above-confidence-threshold) top result ties two or more recordings that resolve to distinct artist/title identities as needing human disambiguation rather than picking one automatically: it SHALL resolve each tied recording's canonical metadata, store the full set as that file's candidates, and set the file's status to `ambiguous` without writing any single resolved-metadata field to the file's own record. A file's stored candidates MAY also originate from a manual search rather than AcoustID tied-recordings — both are stored and resolved through the same mechanism, since the resulting "several candidates, pick one" state is identical regardless of source.

#### Scenario: Tied recordings resolving to distinct identities are recorded as ambiguous
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that resolve to two or more distinct (artist, title) identities
- **THEN** the system SHALL set that file's status to `ambiguous`, store every distinct resolved candidate, and SHALL NOT write resolved metadata to the file's own record

#### Scenario: Tied recordings resolving to the same identity are recorded as a normal success
- **WHEN** identification's AcoustID lookup returns an accepted top result tied to recordings that all resolve to the same (artist, title) identity
- **THEN** the system SHALL set that file's status to `identified` and record that shared identity's resolved metadata, exactly as if AcoustID had returned only one recording

#### Scenario: An ambiguous file's candidates are retrievable
- **WHEN** a file has been recorded `ambiguous`
- **THEN** the system SHALL make its full stored candidate list (each candidate's resolved artist, album, title, track number, and other metadata) available for retrieval

#### Scenario: A manual search's results are recorded the same way, for a file in any prior status
- **WHEN** a manual search for a tracked file returns one or more candidates, regardless of whether that file's prior status was `new`, `not_found`, `identified`, or `ambiguous`
- **THEN** the system SHALL discard the file's prior resolved metadata and any previously stored candidates, store the search's results as its new candidates, and set its status to `ambiguous`

#### Scenario: A manual search with no results does not alter the file's prior state
- **WHEN** a manual search for a tracked file returns zero candidates
- **THEN** the system SHALL leave that file's status, resolved metadata, and any previously stored candidates unchanged

### Requirement: A stored candidate can be chosen to resolve an ambiguous file
The system SHALL allow a stored candidate to be selected for a tracked file whose status is `ambiguous`, recording that choice exactly as a normal successful identification and discarding the file's other stored candidates. This applies uniformly regardless of whether the file's candidates originated from AcoustID tied-recordings or a manual search.

#### Scenario: Choosing a valid candidate resolves the file
- **WHEN** a candidate matching one of an `ambiguous` file's stored recording IDs is chosen
- **THEN** the system SHALL set that file's status to `identified`, store the chosen candidate's resolved metadata exactly as a normal successful identification would, and discard its other stored candidates

#### Scenario: Choosing an unrecognized candidate is rejected
- **WHEN** a candidate recording ID is submitted for a file that does not have a stored candidate with that ID
- **THEN** the system SHALL leave that file's status and stored candidates unchanged and SHALL report that the requested candidate was not found

#### Scenario: Choosing a candidate that originated from a manual search
- **WHEN** a candidate that was stored via a manual search (rather than AcoustID tied-recordings) is chosen
- **THEN** the system SHALL resolve it identically to choosing an AcoustID-sourced candidate — same recorded fields, same downstream tagging/relocation eligibility
