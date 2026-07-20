## MODIFIED Requirements

### Requirement: Change detection on refresh
The system SHALL classify each file discovered during a refresh into exactly one of: newly discovered, changed since last seen, or unchanged since last seen, using size and modification time as the change signal. A refresh SHALL NOT compute a Chromaprint fingerprint for any file — fingerprinting happens lazily during identification instead (see "Fingerprint computed lazily during identification").

#### Scenario: New file discovered
- **WHEN** a refresh finds a file with no existing tracking record
- **THEN** the system SHALL insert a new record with status `new`, its duration read from the file's own audio properties, and no fingerprint

#### Scenario: Changed file's duration is re-read and its stale fingerprint is cleared
- **WHEN** a refresh finds a tracked file whose size or modification time differs from its stored record
- **THEN** the system SHALL re-read its duration, clear any previously stored fingerprint and fingerprint error (since a fingerprint computed against the file's old content must never be reused against its new content), update the stored record, and reset its status to `new`

#### Scenario: Unchanged file's duration is not re-read
- **WHEN** a refresh finds a tracked file whose size and modification time match its stored record
- **THEN** the system SHALL skip reading its duration again and leave its stored status, fingerprint, and duration unchanged

#### Scenario: A successfully tagged file is not seen as changed by a later scan
- **WHEN** a refresh runs after a file has been successfully tagged (which changes the file's own size and modification time on disk)
- **THEN** the system SHALL treat that file as unchanged, since tagging already updated the stored size and modification time to match — the file's identification status, resolved metadata, and fingerprint SHALL NOT be reset

### Requirement: Identification results are recorded per file
The system SHALL update a tracked file's record with the outcome of an identification attempt, without altering its size or modification time. A tracked file's fingerprint is set separately, as part of identification when one isn't already stored (see "Fingerprint computed lazily during identification") — identification's own outcome recording never alters a fingerprint that identification itself just set or reused.

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

## ADDED Requirements

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
