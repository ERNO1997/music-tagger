## MODIFIED Requirements

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
