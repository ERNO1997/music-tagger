## Purpose

Automatically keep a tracked library's derived state (fingerprints, embedded cover art/lyrics detection, and already-relocated files) up to date after every refresh, without requiring the user to manually trigger identification, enrichment, or relocation for work that can be inferred passively from files already on disk.

## Requirements

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

### Requirement: Automatic identification from embedded MusicBrainz IDs
The system SHALL, as part of the analysis pass, read each tracked file's own embedded MusicBrainz recording ID and, when present alongside a non-empty embedded artist and title, treat the file's own tags as authoritative over the tracking store: when that embedded recording ID differs from the file's currently-stored recording MBID (including when the file has no stored recording MBID at all), the system SHALL record the file as `identified` using its embedded artist, album, title, track number, disc number, total discs, total tracks, year, and MusicBrainz recording/release/release-group/artist IDs, and SHALL mark it `tagged`, without calling AcoustID or MusicBrainz. This check runs on every analysis pass for every tracked file, unlike this capability's other detections (embedded cover art/lyrics, already-relocated), which only fill in fields the tracking record doesn't already have — this one can overwrite an already-`identified` record when its embedded recording ID disagrees.

#### Scenario: An unidentified file with an embedded recording ID is identified from it
- **WHEN** the analysis pass reaches a tracked file whose status is not `identified`, and the file's own tags include a MusicBrainz recording ID alongside a non-empty artist and title
- **THEN** the system SHALL record that file as `identified` using its embedded metadata and MusicBrainz IDs, mark it `tagged`, and SHALL NOT call AcoustID or MusicBrainz

#### Scenario: An already-identified file is overwritten when its embedded recording ID disagrees
- **WHEN** the analysis pass reaches a tracked file whose status is already `identified`, and the file's own embedded MusicBrainz recording ID is non-empty and differs from the file's currently-stored recording MBID
- **THEN** the system SHALL overwrite that file's resolved metadata with its embedded tags' values and mark it `tagged`, invalidating any previously stored cover art, lyrics, relocated outcome, and candidate list exactly as any other re-identification does

#### Scenario: An already-identified file whose embedded recording ID agrees is left unchanged
- **WHEN** the analysis pass reaches a tracked file whose status is already `identified`, and the file's own embedded MusicBrainz recording ID matches the file's currently-stored recording MBID
- **THEN** the system SHALL leave that file's resolved metadata, tagged outcome, and relocated outcome unchanged

#### Scenario: A file with no embedded recording ID, or missing artist/title, is left alone
- **WHEN** the analysis pass reaches a tracked file whose own tags include no MusicBrainz recording ID, or include one but no artist or no title
- **THEN** the system SHALL leave that file's tracked status and resolved metadata unchanged, regardless of any other embedded tag data present

#### Scenario: A file identified from its own tags is immediately eligible for this pass's relocation check
- **WHEN** a file is identified (or re-identified) from its own embedded tags earlier in the same analysis pass
- **THEN** the system SHALL use its updated status and tagged outcome for that same pass's canonical-location check, rather than waiting for the next pass to notice it
