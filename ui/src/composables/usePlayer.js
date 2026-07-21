import { reactive } from 'vue';

// Global player state — a single persistent player, driven by whichever
// view's play button was last clicked. Switching views, paging, or
// filtering never tears it down or interrupts playback; playing a new
// track just replaces the source.
export const playerState = reactive({
  visible: false,
  src: '',
  title: '',
  artist: '',
  // Incremented on every playTrack() call, even when replaying the same
  // track — PlayerBar.vue watches this (not `src`) to know when to call
  // .play(), since Vue's watch() doesn't fire when a watched value is set
  // to what's already there, which `src` would be when replaying the same
  // path twice in a row.
  playToken: 0,
});

export function playTrack(entry) {
  const title = entry.title || entry.raw_title || entry.path;
  const artist = entry.artist || entry.raw_artist || '';

  playerState.src = `/api/v1/library/audio?path=${encodeURIComponent(entry.path)}`;
  playerState.title = title;
  playerState.artist = artist;
  playerState.visible = true;
  playerState.playToken += 1;
}

// Hides the player bar. Playing any track afterward (playTrack) brings it
// back exactly as before — src/title/artist are left as-is here since
// playTrack always overwrites them on the next play regardless.
export function closePlayer() {
  playerState.visible = false;
}
