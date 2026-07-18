## MODIFIED Requirements

### Requirement: Fingerprint computation via fpcalc
The system SHALL compute an acoustic fingerprint and duration for a given local audio file by invoking the external `fpcalc` binary via `os/exec` and parsing its output, and SHALL treat this as the sole source of a file's acoustic identity.

#### Scenario: Successful fingerprint computation
- **WHEN** a supported audio file (`.mp3`, `.flac`, or `.m4a`) is passed to the fingerprinting component
- **THEN** it SHALL return a Fingerprint value containing the Chromaprint string and duration in seconds, derived solely from `fpcalc` output

#### Scenario: fpcalc unavailable or produces no usable fingerprint
- **WHEN** the `fpcalc` executable is not found on `PATH`, or `fpcalc` runs but its output contains no parseable, non-empty fingerprint (its exit status alone is not decisive — `fpcalc` may exit non-zero on a benign decode warning while still emitting a valid fingerprint)
- **THEN** the system SHALL return a domain-level fingerprinting error for that file and SHALL NOT fall back to filename or existing-tag-based identification

#### Scenario: Filename and existing tags are never used as identity
- **WHEN** a file has a misleading filename or incorrect/absent embedded tags
- **THEN** the fingerprinting component SHALL still compute identity solely from `fpcalc` output and SHALL NOT read the filename or embedded tags as input to the fingerprint

### Requirement: Unsupported file types are rejected before fingerprinting
The system SHALL only invoke `fpcalc` for files with a `.mp3`, `.flac`, or `.m4a` extension.

#### Scenario: Non-audio or unsupported file skipped
- **WHEN** a file with an extension other than `.mp3`, `.flac`, or `.m4a` is submitted for fingerprinting
- **THEN** the system SHALL skip the file without invoking `fpcalc` and SHALL report it as unsupported rather than as a fingerprinting failure
