## ADDED Requirements

### Requirement: Audio streaming via API
The system SHALL expose a `GET /api/v1/library/audio` endpoint that, given a tracked file's path, streams that file's own audio bytes with the correct `Content-Type` for its tracked format (`audio/mpeg` for MP3, `audio/flac` for FLAC, `audio/mp4` for M4A), supporting HTTP Range requests so playback can seek without downloading the entire file first. The served path SHALL always be looked up from the tracking store, never taken directly from client input beyond the lookup key, consistent with how cover art is already served.

#### Scenario: Streaming a tracked file
- **WHEN** a client requests audio for a tracked, non-missing file
- **THEN** the response SHALL be `200 OK` (or `206 Partial Content` for a ranged request) with the file's audio bytes and the correct `Content-Type` for its format

#### Scenario: Seeking via a Range request
- **WHEN** a client requests a specific byte range of a tracked file's audio
- **THEN** the response SHALL be `206 Partial Content` containing only the requested range

#### Scenario: Requesting audio for a missing or untracked file
- **WHEN** a client requests audio for a path that is not tracked, or is tracked but currently `missing`
- **THEN** the response SHALL be `404 Not Found`

### Requirement: In-browser playback in the web UI
The system SHALL provide a persistent playback control in the web UI, available across the table, grid, folder tree, and Artist-Album views, that plays a selected track's audio via the streaming endpoint without navigating away from or interrupting the current view.

#### Scenario: Playing a track from any view
- **WHEN** a user triggers playback for a track from the table, grid, folder tree, or Artist-Album view
- **THEN** the UI SHALL load and begin playing that track via the audio streaming endpoint, showing its title/artist (resolved, or raw tag snapshot when not yet identified)

#### Scenario: Playback continues across view switches and pagination
- **WHEN** a user switches views or navigates to a different page while a track is playing
- **THEN** playback SHALL continue uninterrupted

#### Scenario: Playing a different track replaces the current one
- **WHEN** a user triggers playback for a track while another is already playing
- **THEN** the current track SHALL stop and the newly selected track SHALL begin playing
