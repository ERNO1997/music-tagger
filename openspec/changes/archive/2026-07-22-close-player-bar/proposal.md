## Why

The persistent player bar (`ui/src/components/PlayerBar.vue`) has no way to dismiss it once a track has played — `playerState.visible` is only ever set to `true`, never back to `false`. Once the user has played anything, the bar (and its embedded `<audio>` element, controls, and currently-loaded track) stays pinned to the bottom of every view for the rest of the session, with no way to get rid of it short of reloading the page.

## What Changes

- Add a close control to the player bar that stops playback and hides the bar.
- Playing any track afterward brings the bar back, showing that track, exactly as it does today.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `audio-playback`: adds a requirement that the persistent playback control can be dismissed, stopping playback and hiding the bar, and reappears the next time a track is played.

## Impact

- Changed code: `ui/src/composables/usePlayer.js` (a `closePlayer()` export alongside the existing `playTrack()`), `ui/src/components/PlayerBar.vue` (a close button, wired to pause the `<audio>` element and hide the bar).
- No backend, API, or database changes.
- No dependency on any other in-progress change — this can land independently.
