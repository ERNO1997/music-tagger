## ADDED Requirements

### Requirement: Automatic analysis pass after every refresh
The system SHALL start an analysis pass automatically, without requiring any user action, immediately after every completed refresh (whether triggered at server startup or on demand), processing every tracked file. The system SHALL expose a `GET /api/v1/library/analyze/status` endpoint reporting `{running, processed, total}`, in the same shape as the existing scan/identify/enrich/tag/relocate status endpoints.

#### Scenario: Analysis starts automatically after a refresh completes
- **WHEN** a refresh (startup-triggered or on-demand) finishes
- **THEN** the system SHALL start an analysis pass over the tracked library without requiring the user to trigger it

#### Scenario: Analysis progress is observable
- **WHEN** an analysis pass is running
- **THEN** `GET /api/v1/library/analyze/status` SHALL report `running: true` along with the number of files processed so far and the total to process

#### Scenario: Analysis does not run concurrently with relocation
- **WHEN** a relocate job is currently running
- **THEN** the system SHALL NOT start an analysis pass until it completes, mirroring the existing mutual exclusion between refresh and relocate

### Requirement: Automatic fingerprinting during analysis
The system SHALL compute a Chromaprint fingerprint, via the same mechanism identification already uses, for every tracked file that does not yet have one, as part of the analysis pass.

#### Scenario: A file with no stored fingerprint is fingerprinted
- **WHEN** the analysis pass reaches a tracked file with no fingerprint stored
- **THEN** the system SHALL compute and persist its fingerprint and duration, identically to how identification would if triggered manually first

#### Scenario: A file that already has a fingerprint is left alone
- **WHEN** the analysis pass reaches a tracked file that already has a fingerprint stored
- **THEN** the system SHALL NOT recompute it

### Requirement: Automatic detection of embedded cover art and lyrics
The system SHALL, as part of the analysis pass, read each tracked file's own embedded cover art and lyrics directly from the file, and store whichever of these the tracking record does not already have — so that a file's `has_cover_art`/`has_lyrics` outcome reflects content actually embedded in the file itself, independent of whether this app's own enrichment has ever run against it.

#### Scenario: Embedded cover art is stored when the record has none yet
- **WHEN** the analysis pass reaches a tracked file whose tracking record has no stored cover art, and the file itself has an embedded cover image
- **THEN** the system SHALL store that embedded image as the file's cover art, exactly as enrichment storing a downloaded cover would

#### Scenario: Embedded lyrics are stored when the record has none yet
- **WHEN** the analysis pass reaches a tracked file whose tracking record has no stored lyrics, and the file itself has embedded lyrics
- **THEN** the system SHALL store those embedded lyrics as the file's lyrics, exactly as enrichment storing looked-up lyrics would

#### Scenario: Existing enrichment or a prior pass's result is never overwritten
- **WHEN** the analysis pass reaches a tracked file that already has a stored cover art path and/or lyrics (from prior enrichment, a manually chosen cover, or an earlier analysis pass)
- **THEN** the system SHALL leave that already-stored field unchanged, regardless of what the file's own embedded tags currently contain

#### Scenario: A file with neither embedded cover art nor lyrics is left alone
- **WHEN** the analysis pass reaches a tracked file with no stored cover art or lyrics and the file itself has neither embedded
- **THEN** the system SHALL leave both fields empty, without treating this as an error

### Requirement: Automatic detection of files already at their canonical location
The system SHALL, as part of the analysis pass, check every tracked file that is `identified`, has a tagged outcome of true, and is not already marked `relocated`, against its computed canonical destination path (the same computation the on-demand relocation action uses), and mark it `relocated` when its current tracked path already equals that destination — without moving the file.

#### Scenario: A file already at its canonical destination is marked relocated
- **WHEN** the analysis pass reaches an identified, tagged, not-yet-relocated file whose current tracked path already equals its computed canonical destination
- **THEN** the system SHALL mark that file `relocated`, without moving it or altering its tracked path

#### Scenario: A file not at its canonical destination is left unmarked
- **WHEN** the analysis pass reaches an identified, tagged, not-yet-relocated file whose current tracked path does not equal its computed canonical destination
- **THEN** the system SHALL leave its relocated outcome unchanged

#### Scenario: Files that aren't both identified and tagged are skipped
- **WHEN** the analysis pass reaches a tracked file that is not `identified`, or is identified but not yet tagged
- **THEN** the system SHALL skip the relocation check for that file, consistent with the on-demand relocation action's own precondition
