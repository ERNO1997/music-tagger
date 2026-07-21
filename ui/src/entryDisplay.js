import { STATUS_LABELS, STATUS_CLASSES } from './format.js';

// Shared per-entry display helpers — originally table.js's
// renderMetadataCell/renderCoverCell, imported by tree.js/grid.js/
// artist-album.js. Kept as plain functions (not string-building HTML)
// since Vue templates render them directly.
export function statusLabel(entry) {
  return STATUS_LABELS[entry.status] || entry.status;
}

export function statusClass(entry) {
  return STATUS_CLASSES[entry.status] || '';
}

export function metadataText(entry) {
  if (entry.status === 'identified') {
    const track = entry.track_number ? `Track ${entry.track_number}` : '';
    return [entry.artist, entry.album, entry.title, track].filter(Boolean).join(' – ');
  }
  return [entry.raw_artist, entry.raw_album, entry.raw_title].filter(Boolean).join(' – ');
}

export function hasRawMetadata(entry) {
  return entry.status !== 'identified' && !!(entry.raw_artist || entry.raw_album || entry.raw_title);
}

export function coverSrc(entry) {
  return entry.has_cover_art ? `/api/v1/library/cover?path=${encodeURIComponent(entry.path)}` : null;
}
