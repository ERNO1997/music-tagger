export function formatDuration(seconds) {
  const total = Math.round(seconds || 0);
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
}

export function formatEta(totalSeconds) {
  const totalMinutes = Math.ceil(totalSeconds / 60);
  if (totalMinutes < 60) {
    return `~${totalMinutes} minute${totalMinutes === 1 ? '' : 's'}`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes === 0 ? `~${hours} hour${hours === 1 ? '' : 's'}` : `~${hours}h ${minutes}m`;
}

export function escapeHtml(value) {
  const div = document.createElement('div');
  div.textContent = value ?? '';
  return div.innerHTML;
}

export const STATUS_LABELS = {
  new: 'New',
  identified: 'Identified',
  not_found: 'Not Found',
  ambiguous: 'Ambiguous',
  missing: 'Missing',
};

export const STATUS_CLASSES = {
  new: 'text-blue-400',
  identified: 'text-green-400',
  not_found: 'text-yellow-400',
  ambiguous: 'text-orange-400',
  missing: 'text-neutral-500',
};

export const DETAILS_FIELD_LABELS = [
  ['path', 'Path'],
  ['format', 'Format'],
  ['duration_seconds', 'Duration', formatDuration],
  ['status', 'Status', (v) => STATUS_LABELS[v] || v],
  ['error', 'Error'],
  ['artist', 'Artist'],
  ['album_artist', 'Album Artist'],
  ['title', 'Title'],
  ['track_number', 'Track Number'],
  ['disc_number', 'Disc Number'],
  ['total_discs', 'Total Discs'],
  ['total_tracks', 'Total Tracks'],
  ['year', 'Year'],
  ['recording_mbid', 'Recording MBID'],
  ['release_mbid', 'Release MBID'],
  ['release_group_mbid', 'Release-Group MBID'],
  ['artist_mbid', 'Artist MBID'],
];

export const RAW_TAG_FIELD_LABELS = [
  ['raw_title', 'Title'],
  ['raw_artist', 'Artist'],
  ['raw_album', 'Album'],
  ['raw_album_artist', 'Album Artist'],
];

export const EMBEDDED_TAG_FIELD_LABELS = [
  ['title', 'Title'],
  ['artist', 'Artist'],
  ['album', 'Album'],
  ['album_artist', 'Album Artist'],
  ['track_number', 'Track Number'],
  ['disc_number', 'Disc Number'],
  ['year', 'Year'],
];

// MusicBrainz's hard 1 req/sec rate limit (charter §4.2) means an identify
// job over N files takes roughly N seconds — above this selection size the
// user is warned with an ETA before the job starts.
export const IDENTIFY_ETA_THRESHOLD = 20;
