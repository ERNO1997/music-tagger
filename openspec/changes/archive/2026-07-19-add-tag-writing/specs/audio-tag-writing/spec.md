## ADDED Requirements

### Requirement: On-demand tag writing for identified files
The system SHALL, on demand and only for tracked files with status `identified`, write that file's resolved artist, album artist, title, track number, disc number/total discs, total tracks, release year, cover art (when stored), and plain lyrics (when stored) into the physical audio file's own tag format at its current path: ID3v2 for MP3, Vorbis comments plus a `PICTURE` metadata block for FLAC, and MP4/iTunes atoms for M4A — determined by the file's real, content-detected format (see the format-detection requirement below), not assumed from its `.mp3`/`.flac`/`.m4a` extension.

#### Scenario: MP3 tagging writes ID3v2 frames
- **WHEN** tagging is triggered for an identified `.mp3` file
- **THEN** the system SHALL write `TIT2` (title), `TPE1` (artist), `TALB` (album), `TPE2` (album artist), `TRCK` (track number), and `TPOS` (disc number) ID3v2 text frames, an `APIC` frame when cover art is stored, and a `USLT` frame when plain lyrics are stored

#### Scenario: FLAC tagging writes Vorbis comments and a picture block
- **WHEN** tagging is triggered for an identified `.flac` file
- **THEN** the system SHALL write `TITLE`, `ARTIST`, `ALBUM`, `ALBUMARTIST`, `TRACKNUMBER`, and `DISCNUMBER` Vorbis comment fields, a `PICTURE` metadata block when cover art is stored, and a `LYRICS` comment field when plain lyrics are stored

#### Scenario: M4A tagging writes MP4 atoms
- **WHEN** tagging is triggered for an identified `.m4a` file
- **THEN** the system SHALL write `©nam`, `©ART`, `©alb`, `aART`, `trkn`, and `disk` MP4 atoms, a `covr` atom when cover art is stored, and a `©lyr` atom when plain lyrics are stored

#### Scenario: Tagging preserves unrelated existing tag data
- **WHEN** tagging is triggered for a file that already has tag fields or frames not covered by this requirement (e.g. a custom comment, ReplayGain tags, an existing encoder comment)
- **THEN** the system SHALL leave those unrelated fields/frames untouched

#### Scenario: Tagging skips files that are not yet identified
- **WHEN** tagging is requested for a tracked file whose status is not `identified`
- **THEN** the system SHALL skip that file, log the reason, and continue processing the rest of the requested paths without aborting the batch

#### Scenario: Missing cover art or lyrics does not block tagging
- **WHEN** tagging is triggered for an identified file that has no stored cover art and/or no stored lyrics
- **THEN** the system SHALL still write the available metadata fields, omitting only the `APIC`/`PICTURE`/`covr` frame and/or the `USLT`/`LYRICS`/`©lyr` frame for the missing piece

#### Scenario: Tagging failure leaves the original file untouched
- **WHEN** a tag write fails partway (e.g. the file is locked, unwritable, or malformed)
- **THEN** the system SHALL leave the original file's on-disk contents unchanged and SHALL record the failure against that file rather than leaving a partially-written file

#### Scenario: Re-tagging after resolved metadata changes
- **WHEN** tagging is triggered again for a file that was previously tagged, and its resolved metadata, cover art, or lyrics have since changed (e.g. via re-identification)
- **THEN** the system SHALL overwrite the previously written tag fields with the current resolved values

### Requirement: Reading back embedded tags for verification
The system SHALL be able to read a file's actual, currently-embedded tag values (title, artist, album, album artist, track number, disc number, year, whether lyrics are embedded, whether cover art is embedded) directly from the physical file, independent of and without relying on the resolved metadata cached in the tracking store, so that what was actually written can be visually verified against what was resolved.

#### Scenario: Embedded tags reflect a successful write
- **WHEN** the embedded tags of a file are read back immediately after a successful tag write
- **THEN** the returned values SHALL match the metadata that was written (title, artist, album, album artist, track number, disc number, year, and presence of lyrics/cover art)

#### Scenario: Embedded tags read from an untagged file
- **WHEN** the embedded tags of a file that has never been tagged by this system are read back
- **THEN** the system SHALL return whatever tag values (if any) already exist in the file, without error, distinct from an empty tracking-store record

### Requirement: Tag format is determined by content, not file extension
The system SHALL determine which tag format to write to or read from (ID3v2, Vorbis comment, or MP4 atom) by inspecting the physical file's actual leading bytes, and SHALL NOT assume the file's format from its `.mp3`/`.flac`/`.m4a` extension alone — a file's extension is untrusted input, for the same reason the system already treats filenames as untrusted for identification purposes.

#### Scenario: Extension matches real format
- **WHEN** a tracked file's extension matches its actual, content-detected container format
- **THEN** the system SHALL tag or read it normally, with no special handling

#### Scenario: Extension does not match real format
- **WHEN** a tracked file's extension does not match its actual, content-detected container format (e.g. an MP4/M4A container saved with a `.mp3` extension)
- **THEN** the system SHALL write or read tags according to the file's real format rather than the format implied by its extension, and SHALL NOT write tags in the format implied by the mismatched extension

#### Scenario: Filename is unaffected by format correction
- **WHEN** tagging or reading embedded tags for a file whose extension does not match its real format
- **THEN** the file's name and location on disk SHALL be unchanged once the operation completes — format correction affects only which tag representation is written, not the file's path

#### Scenario: Extension-trusting external tools may not see corrected tags (known limitation)
- **WHEN** a file whose extension doesn't match its real format has been successfully tagged in its real format
- **THEN** an external tool that itself determines the file's format from its extension rather than its content (e.g. macOS Finder/Spotlight) MAY show no metadata or cover art for that file, since it will look for tags in the format implied by the extension rather than the file's real format — this is a known limitation of this capability, not a defect; the complete fix (renaming the file to match its real format) is out of scope here

#### Scenario: Unrecognized format falls back to the extension
- **WHEN** a tracked file's real format cannot be determined from its content
- **THEN** the system SHALL fall back to the format implied by its extension rather than failing the operation
