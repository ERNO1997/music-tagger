## Purpose

On-demand physical relocation of an already-identified and already-tagged tracked file from its current path into a canonical directory hierarchy rooted at the configured music root — `{music root}/{sanitized artist}/{release year - }{sanitized album}/{zero-padded track number} - {sanitized title}{original extension}` — derived from the file's own resolved metadata, so that a tagged library also ends up organized on disk without requiring a separate manual reorganization step. This includes cleaning up now-empty source directories left behind by a move, sanitizing path segments before any filesystem call, and treating an occupied destination as a failure rather than silently overwriting or renaming around it.

## Requirements

### Requirement: On-demand relocation for identified and tagged files
The system SHALL, on demand and only for tracked files with status `identified` and a tagged outcome of true, physically move the file from its current path to `{music root}/{sanitized artist}/{release year - }{sanitized album}/{zero-padded track number} - {sanitized title}{original extension}`, using the file's own resolved metadata. The release year prefix on the album directory is included when the release has a known year, and omitted when it does not.

#### Scenario: Successful relocation moves the file
- **WHEN** relocation is triggered for an identified, tagged file
- **THEN** the system SHALL create any missing destination directories and move the file to the computed destination path, leaving no copy at the original path

#### Scenario: Track number is zero-padded
- **WHEN** relocation computes a destination filename for a file with a resolved track number
- **THEN** the track number SHALL be zero-padded (e.g. `07`) so files sort correctly within their album directory by filename

#### Scenario: Album directory is prefixed with the release year when known
- **WHEN** relocation computes a destination for a file whose resolved release has a known year
- **THEN** the album directory name SHALL be `{year} - {sanitized album}` (e.g. `2004 - Dead Letters`)

#### Scenario: Album directory omits the year prefix when unknown
- **WHEN** relocation computes a destination for a file whose resolved release has no usable year
- **THEN** the album directory name SHALL be just the sanitized album name, with no year prefix or placeholder

#### Scenario: Relocation skips files that are not both identified and tagged
- **WHEN** relocation is requested for a tracked file whose status is not `identified`, or whose tagged outcome is not true
- **THEN** the system SHALL skip that file, log the reason, and continue processing the rest of the requested paths without aborting the batch

#### Scenario: Relocation failure leaves the source file untouched
- **WHEN** a relocation fails partway (e.g. the destination directory cannot be created, or the move itself fails)
- **THEN** the system SHALL leave the source file at its original path, unmodified, and SHALL NOT leave a partially-written or duplicate file at the destination

#### Scenario: Recording the outcome fails after a successful physical move
- **WHEN** a file is successfully moved to its destination but recording that outcome in the tracking store subsequently fails
- **THEN** the system SHALL move the file back to its original path before reporting the failure, so the file's on-disk location matches its tracking record either way

#### Scenario: The best-effort move-back itself fails
- **WHEN** recording the outcome fails and the subsequent attempt to move the file back to its original path also fails
- **THEN** the system SHALL report a failure that plainly states the file's actual current path and its tracked path, and SHALL log this distinctly rather than treating it as an ordinary relocation failure

### Requirement: Empty source directories are removed after relocation
The system SHALL, after successfully moving a file, remove its original directory and any now-empty ancestor directories, stopping at the first ancestor that still contains a real file or subdirectory, or at the configured music root itself — never above it. A directory containing only OS-generated junk files (e.g. `.DS_Store`, `Thumbs.db`, `desktop.ini`, AppleDouble `._*` sidecar files) SHALL be treated as empty, and that junk deleted along with the directory. This cleanup SHALL NOT cause the relocation itself to be reported as failed if the cleanup encounters an error.

#### Scenario: A now-empty original directory is removed
- **WHEN** relocating a file leaves its original directory with no remaining files or subdirectories
- **THEN** the system SHALL remove that directory

#### Scenario: Empty ancestor directories are removed in a chain
- **WHEN** removing a now-empty directory leaves its parent directory also empty, and so on
- **THEN** the system SHALL remove each consecutively-emptied ancestor directory, stopping at the first ancestor that is not empty or at the music root

#### Scenario: The music root itself is never removed
- **WHEN** the upward empty-directory cleanup reaches the configured music root
- **THEN** the system SHALL stop without removing the music root, regardless of whether it is now empty

#### Scenario: A directory containing only OS junk files is treated as empty
- **WHEN** a file's original directory, after relocation, contains only files like `.DS_Store` and no other files or subdirectories
- **THEN** the system SHALL delete those junk files and remove the directory

#### Scenario: A directory containing a real file or subdirectory is left alone
- **WHEN** a file's original directory still contains another real file or a subdirectory after relocation
- **THEN** the system SHALL NOT remove that directory or any of its ancestors

#### Scenario: Directory cleanup failure does not fail the relocation
- **WHEN** removing an empty directory fails (e.g. a permissions error)
- **THEN** the system SHALL leave the relocation itself reported as successful, since the file was still correctly moved and tracked

### Requirement: Path segments are sanitized before any filesystem call
The system SHALL strip the characters `\ / : * ? " < > |` from the artist, album, and title segments used to build a relocation destination, and SHALL perform this sanitization before constructing the destination path used in any directory-creation or move call — never after.

#### Scenario: Sanitization removes filesystem-prohibited characters
- **WHEN** a file's resolved artist, album, or title contains any of `\ / : * ? " < > |`
- **THEN** those characters SHALL be stripped from the corresponding destination path segment before it is used in any filesystem call

#### Scenario: Sanitized segments are used consistently
- **WHEN** the destination directory is created and the file is subsequently moved
- **THEN** both operations SHALL use the same already-sanitized path segments — sanitization SHALL NOT be applied only to one of the two calls

### Requirement: Destination collisions are treated as a relocation failure
The system SHALL treat an already-existing file at a computed destination path as a relocation failure for that file, and SHALL NOT overwrite the existing file, rename around it, or move the source file to an alternate location.

#### Scenario: Destination path already occupied
- **WHEN** relocation computes a destination path and a file already exists there
- **THEN** the system SHALL fail that file's relocation, leave both the source and the pre-existing destination file unchanged, and record the collision as the failure reason

#### Scenario: A file already at its own computed destination is not a collision with itself
- **WHEN** relocation computes a destination path for a file that is already located at exactly that path
- **THEN** the system SHALL treat this as a successful no-op rather than a destination collision, without moving the file or reporting a failure
