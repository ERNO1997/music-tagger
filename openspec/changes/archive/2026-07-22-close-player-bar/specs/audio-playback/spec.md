## ADDED Requirements

### Requirement: Dismissing the player
The system SHALL provide a control on the persistent player bar that stops playback and hides the bar, and SHALL show the bar again, playing the newly selected track, the next time playback is triggered from any view.

#### Scenario: Closing the player while a track is playing
- **WHEN** a user activates the close control on the player bar
- **THEN** playback SHALL stop and the player bar SHALL be hidden

#### Scenario: Playing a track brings the player back
- **WHEN** a user triggers playback for any track while the player bar is closed
- **THEN** the player bar SHALL become visible again, showing that track
