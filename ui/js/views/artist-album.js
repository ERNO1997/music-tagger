import { state, buildFilterParams } from '../state.js';
import { formatDuration, escapeHtml, STATUS_LABELS, STATUS_CLASSES } from '../format.js';
import { fetchArtists, fetchAlbums, fetchTracks } from '../api.js';
import { renderCoverCell, renderMetadataCell } from '../table.js';
import { playTrack } from '../player.js';

const breadcrumbEl = document.getElementById('artist-album-breadcrumb');
const listEl = document.getElementById('artist-album-list');
const tracksWrap = document.getElementById('artist-album-tracks-wrap');
const tracksEl = document.getElementById('artist-album-tracks');

// Set by initArtistAlbum(); invoked when a track row is clicked to open its
// details view.
let onTrackOpen = null;

// level is one of 'artists' | 'albums' | 'tracks' — the current drill-down
// depth. selectedArtist/selectedAlbum are set as the user drills in.
let level = 'artists';
let selectedArtist = null;
let selectedAlbum = null;
let currentTracks = [];

function renderBreadcrumb() {
  breadcrumbEl.innerHTML = '';
  const crumbs = [{ label: 'Artists', onClick: showArtists }];
  if (level === 'albums' || level === 'tracks') {
    crumbs.push({ label: selectedArtist, onClick: () => showAlbums(selectedArtist) });
  }
  if (level === 'tracks') {
    crumbs.push({ label: selectedAlbum, onClick: () => showTracks(selectedArtist, selectedAlbum) });
  }
  crumbs.forEach((crumb, i) => {
    if (i > 0) {
      breadcrumbEl.appendChild(document.createTextNode(' / '));
    }
    const link = document.createElement('button');
    link.textContent = crumb.label;
    link.className = i === crumbs.length - 1 ? 'text-neutral-200 font-medium' : 'text-blue-400 hover:underline';
    link.addEventListener('click', crumb.onClick);
    breadcrumbEl.appendChild(link);
  });
}

export async function showArtists() {
  level = 'artists';
  selectedArtist = null;
  selectedAlbum = null;
  tracksWrap.classList.add('hidden');
  listEl.classList.remove('hidden');
  renderBreadcrumb();

  try {
    const data = await fetchArtists(buildFilterParams());
    listEl.innerHTML = '';
    for (const a of data.artists || []) {
      listEl.appendChild(renderListCard(a.artist, `${a.track_count} track(s)`, () => showAlbums(a.artist)));
    }
  } catch (err) {
    listEl.innerHTML = `<p class="col-span-4 text-red-400 text-xs">Failed to load artists: ${escapeHtml(err.message)}</p>`;
  }
}

export async function showAlbums(artist) {
  level = 'albums';
  selectedArtist = artist;
  selectedAlbum = null;
  tracksWrap.classList.add('hidden');
  listEl.classList.remove('hidden');
  renderBreadcrumb();

  const params = buildFilterParams();
  params.set('artist', artist);
  try {
    const data = await fetchAlbums(params);
    listEl.innerHTML = '';
    for (const a of data.albums || []) {
      listEl.appendChild(renderListCard(a.album, `${a.track_count} track(s)`, () => showTracks(artist, a.album)));
    }
  } catch (err) {
    listEl.innerHTML = `<p class="col-span-4 text-red-400 text-xs">Failed to load albums: ${escapeHtml(err.message)}</p>`;
  }
}

export async function showTracks(artist, album) {
  level = 'tracks';
  selectedArtist = artist;
  selectedAlbum = album;
  listEl.classList.add('hidden');
  tracksWrap.classList.remove('hidden');
  renderBreadcrumb();

  const params = buildFilterParams();
  params.set('artist', artist);
  params.set('album', album);
  try {
    const data = await fetchTracks(params);
    currentTracks = data.entries || [];
    // openDetails() (details.js) looks entries up via state.lastEntries
    // regardless of which view is active.
    state.lastEntries = currentTracks;
    tracksEl.innerHTML = '';
    for (const entry of currentTracks) {
      tracksEl.appendChild(renderTrackRow(entry));
    }
  } catch (err) {
    tracksEl.innerHTML = `<tr><td class="px-4 py-3 text-red-400" colspan="7">Failed to load tracks: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function renderListCard(name, subtitle, onClick) {
  const card = document.createElement('button');
  card.className = 'text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600';
  card.innerHTML = `
    <div class="text-sm truncate">${escapeHtml(name)}</div>
    <div class="text-xs text-neutral-500">${escapeHtml(subtitle)}</div>
  `;
  card.addEventListener('click', onClick);
  return card;
}

function renderTrackRow(entry) {
  const row = document.createElement('tr');
  row.dataset.path = entry.path;
  row.classList.add('cursor-pointer', 'hover:bg-neutral-900');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const playButton = entry.status === 'missing'
    ? '—'
    : `<button class="play-button text-neutral-300 hover:text-white" data-path="${escapeHtml(entry.path)}" title="Play">&#9654;</button>`;

  row.innerHTML = `
    <td class="px-4 py-3">${renderCoverCell(entry)}</td>
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 ${statusClass}">${escapeHtml(statusLabel)}</td>
    <td class="px-4 py-3">${renderMetadataCell(entry)}</td>
    <td class="px-4 py-3">${playButton}</td>
  `;
  return row;
}

tracksEl.addEventListener('click', (e) => {
  if (e.target.closest('.play-button')) {
    e.stopPropagation();
    const path = e.target.closest('.play-button').dataset.path;
    const entry = currentTracks.find((en) => en.path === path);
    if (entry) {
      playTrack(entry);
    }
    return;
  }
  const row = e.target.closest('tr');
  if (!row || !row.dataset.path) {
    return;
  }
  if (onTrackOpen) {
    onTrackOpen(row.dataset.path);
  }
});

// Re-fetches whatever level is currently displayed — for main.js's
// refreshCurrentView when artist-album is the active view.
export function reloadArtistAlbum() {
  if (level === 'albums') {
    return showAlbums(selectedArtist);
  }
  if (level === 'tracks') {
    return showTracks(selectedArtist, selectedAlbum);
  }
  return showArtists();
}

export function initArtistAlbum(onTrackOpenCallback) {
  onTrackOpen = onTrackOpenCallback;
}
