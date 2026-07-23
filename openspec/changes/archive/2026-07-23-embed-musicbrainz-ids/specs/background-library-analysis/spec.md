## ADDED Requirements

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
