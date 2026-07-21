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
});

export function playTrack(entry) {
  const title = entry.title || entry.raw_title || entry.path;
  const artist = entry.artist || entry.raw_artist || '';

  playerState.src = `/api/v1/library/audio?path=${encodeURIComponent(entry.path)}`;
  playerState.title = title;
  playerState.artist = artist;
  playerState.visible = true;
}
