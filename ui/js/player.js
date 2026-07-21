const playerBar = document.getElementById('player-bar');
const playerAudio = document.getElementById('player-audio');
const playerTitle = document.getElementById('player-title');
const playerArtist = document.getElementById('player-artist');

// A single persistent player, mounted once outside every view container —
// switching views, paging, or filtering never tears it down or interrupts
// playback. Playing a new track just replaces this element's src.
export function playTrack(entry) {
  const title = entry.title || entry.raw_title || entry.path;
  const artist = entry.artist || entry.raw_artist || '';

  playerAudio.src = `/api/v1/library/audio?path=${encodeURIComponent(entry.path)}`;
  playerTitle.textContent = title;
  playerArtist.textContent = artist;
  playerBar.classList.remove('hidden');
  playerAudio.play();
}
