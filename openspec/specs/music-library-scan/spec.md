## Purpose

Read-only discovery and fingerprint reporting for the mounted local `/music` volume: recursively finding supported audio files and surfacing their acoustic fingerprints through an API and a web listing page, with no external network calls, tagging, or file relocation.

## Requirements

### Requirement: Recursive discovery of audio files in the mounted volume
The system SHALL recursively walk the configured `/music` directory and identify all files with `.mp3` or `.flac` extensions as candidate audio files, at any subdirectory depth.

#### Scenario: Nested directories are included
- **WHEN** `/music` contains audio files nested at arbitrary subdirectory depths
- **THEN** all matching files SHALL be included in the scan result regardless of depth

#### Scenario: Non-audio files are ignored
- **WHEN** `/music` contains files with extensions other than `.mp3`/`.flac`
- **THEN** those files SHALL be excluded from the scan result and SHALL NOT be passed to the fingerprinting component

### Requirement: Read-only scan report via API
The system SHALL expose a `GET /api/v1/library` endpoint that synchronously scans `/music`, computes a fingerprint for each discovered file, and returns a JSON array of file path, format, duration, and fingerprint. The scan SHALL NOT write, move, rename, or otherwise modify any file under `/music`.

#### Scenario: Successful scan response
- **WHEN** a client issues `GET /api/v1/library` against a populated `/music` volume
- **THEN** the response SHALL be `200 OK` with a JSON array where each entry includes `path`, `format`, `duration_seconds`, and `fingerprint`

#### Scenario: Per-file fingerprint failure does not abort the scan
- **WHEN** one file in `/music` fails fingerprinting (e.g. a corrupt audio file)
- **THEN** that file SHALL be reported with an error indicator in its entry and the scan SHALL continue processing the remaining files

#### Scenario: Empty or missing volume
- **WHEN** `/music` contains no supported audio files, or the directory does not exist
- **THEN** the endpoint SHALL return `200 OK` with an empty array rather than an error

#### Scenario: No external calls or filesystem writes occur
- **WHEN** the scan runs to completion, successfully or with per-file errors
- **THEN** the system SHALL NOT call AcoustID, MusicBrainz, Cover Art Archive, or Genius, and SHALL NOT call `os.MkdirAll` or `os.Rename` on any file under `/music`

### Requirement: Web UI listing of scan results
The system SHALL serve a dark-mode web page that fetches `GET /api/v1/library` and renders each entry as a row showing path, format, duration, and fingerprint.

#### Scenario: Page loads scan results on open
- **WHEN** a user opens the web UI in a browser
- **THEN** the page SHALL call `GET /api/v1/library` and render one row per returned file, displaying no cover art, tag, or lyrics data
