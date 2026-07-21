import { state } from './state.js';
import { escapeHtml, DETAILS_FIELD_LABELS, RAW_TAG_FIELD_LABELS, EMBEDDED_TAG_FIELD_LABELS } from './format.js';
import {
  fetchCandidates,
  searchIdentify,
  postIdentifyResolve,
  fetchCoverCandidates,
  postCoverChoose,
  fetchFingerprint,
  fetchEmbeddedTags,
  fetchLyrics,
} from './api.js';
// Set by initDetails(); main.js's refreshCurrentView, which refreshes
// whichever of table/grid/tree/artist-album is presently active — not
// hardcoded to table.js's loadLibrary, since a candidate resolve or cover
// choice can happen while any of the four views is showing this file.
let refreshCurrentView = async () => {};

const detailsOverlay = document.getElementById('details-overlay');
const detailsFields = document.getElementById('details-fields');
const detailsRawTagsSection = document.getElementById('details-raw-tags-section');
const detailsRawTags = document.getElementById('details-raw-tags');
const detailsClose = document.getElementById('details-close');
const detailsCover = document.getElementById('details-cover');
const detailsLyricsSection = document.getElementById('details-lyrics-section');
const detailsLyrics = document.getElementById('details-lyrics');
const detailsEmbeddedTagsSection = document.getElementById('details-embedded-tags-section');
const detailsEmbeddedTags = document.getElementById('details-embedded-tags');
const detailsCandidatesSection = document.getElementById('details-candidates-section');
const detailsCandidatesHeading = document.getElementById('details-candidates-heading');
const detailsCandidates = document.getElementById('details-candidates');
const manualSearchArtistInput = document.getElementById('manual-search-artist');
const manualSearchTitleInput = document.getElementById('manual-search-title');
const manualSearchAlbumInput = document.getElementById('manual-search-album');
const manualSearchButton = document.getElementById('manual-search-button');
const manualSearchStatus = document.getElementById('manual-search-status');
const detailsBrowseCoversToggleWrap = document.getElementById('details-browse-covers-toggle-wrap');
const detailsBrowseCoversToggle = document.getElementById('details-browse-covers-toggle');
const detailsCoverCandidatesSection = document.getElementById('details-cover-candidates-section');
const detailsCoverCandidates = document.getElementById('details-cover-candidates');

export async function openDetails(path) {
  const entry = state.lastEntries.find((e) => e.path === path);
  if (!entry) {
    return;
  }

  if (entry.has_cover_art) {
    detailsCover.src = `/api/v1/library/cover?path=${encodeURIComponent(entry.path)}`;
    detailsCover.classList.remove('hidden');
  } else {
    detailsCover.removeAttribute('src');
    detailsCover.classList.add('hidden');
  }

  detailsFields.innerHTML = '';
  for (const [key, label, formatter] of DETAILS_FIELD_LABELS) {
    const value = entry[key];
    if (value === undefined || value === null || value === '') {
      continue;
    }
    const displayValue = formatter ? formatter(value) : value;
    const row = document.createElement('div');
    row.className = 'flex justify-between gap-4';
    row.innerHTML = `
      <dt class="text-neutral-400">${escapeHtml(label)}</dt>
      <dd class="font-mono text-xs text-right break-all">${escapeHtml(String(displayValue))}</dd>
    `;
    detailsFields.appendChild(row);
  }

  detailsRawTagsSection.classList.add('hidden');
  detailsRawTags.innerHTML = '';
  if (entry.status !== 'identified') {
    const rawFields = RAW_TAG_FIELD_LABELS.filter(([key]) => entry[key]);
    if (rawFields.length > 0) {
      for (const [key, label] of rawFields) {
        const row = document.createElement('div');
        row.className = 'flex justify-between gap-4';
        row.innerHTML = `
          <dt class="text-neutral-400">${escapeHtml(label)}</dt>
          <dd class="font-mono text-xs text-right break-all">${escapeHtml(entry[key])}</dd>
        `;
        detailsRawTags.appendChild(row);
      }
      detailsRawTagsSection.classList.remove('hidden');
    }
  }

  detailsLyricsSection.classList.add('hidden');
  detailsLyrics.textContent = '';
  if (entry.has_lyrics) {
    await loadLyrics(entry.path);
  }

  detailsEmbeddedTagsSection.classList.add('hidden');
  detailsEmbeddedTags.innerHTML = '';
  if (entry.tagged) {
    await loadEmbeddedTags(entry.path);
  }

  detailsCandidatesSection.classList.add('hidden');
  detailsCandidates.innerHTML = '';
  detailsCandidatesHeading.textContent = 'Choose the correct recording';
  if (entry.status === 'ambiguous') {
    await loadCandidates(entry.path);
  }

  manualSearchButton.dataset.path = entry.path;
  manualSearchButton.dataset.status = entry.status;
  manualSearchArtistInput.value = '';
  manualSearchTitleInput.value = '';
  manualSearchAlbumInput.value = '';
  manualSearchStatus.textContent = '';

  detailsCoverCandidatesSection.classList.add('hidden');
  detailsCoverCandidates.innerHTML = '';
  detailsBrowseCoversToggle.dataset.path = entry.path;
  detailsBrowseCoversToggle.textContent = 'Browse other covers…';
  if (entry.status === 'identified') {
    detailsBrowseCoversToggleWrap.classList.remove('hidden');
  } else {
    detailsBrowseCoversToggleWrap.classList.add('hidden');
  }

  await loadFingerprint(entry.path);

  detailsOverlay.classList.remove('hidden');
}

function renderCandidates(path, candidates) {
  detailsCandidates.innerHTML = '';
  for (const candidate of candidates) {
    const track = candidate.track_number ? `Track ${candidate.track_number}` : '';
    const summary = [candidate.artist, candidate.album, candidate.title, track].filter(Boolean).join(' – ');
    const row = document.createElement('div');
    row.className = 'flex items-center justify-between gap-3 bg-neutral-800 rounded-md px-3 py-2';
    row.innerHTML = `
      <span class="text-neutral-200">${escapeHtml(summary)}</span>
      <button class="use-candidate-button shrink-0 rounded-md bg-blue-600 text-white text-xs font-medium px-3 py-1.5 hover:bg-blue-500" data-recording-mbid="${escapeHtml(candidate.recording_mbid)}">Use this</button>
    `;
    row.querySelector('.use-candidate-button').addEventListener('click', () => resolveCandidate(path, candidate.recording_mbid));
    detailsCandidates.appendChild(row);
  }
  detailsCandidatesSection.classList.remove('hidden');
}

async function loadCandidates(path) {
  try {
    const candidates = await fetchCandidates(path);
    renderCandidates(path, candidates);
  } catch (err) {
    detailsCandidates.textContent = `Failed to load candidates: ${err.message}`;
    detailsCandidatesSection.classList.remove('hidden');
  }
}

function luceneEscape(value) {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

function buildManualSearchQuery(artist, title, album) {
  const parts = [];
  if (artist) parts.push(`artist:"${luceneEscape(artist)}"`);
  if (title) parts.push(`recording:"${luceneEscape(title)}"`);
  if (album) parts.push(`release:"${luceneEscape(album)}"`);
  return parts.join(' AND ');
}

manualSearchButton.addEventListener('click', async () => {
  const path = manualSearchButton.dataset.path;
  const status = manualSearchButton.dataset.status;
  const artist = manualSearchArtistInput.value.trim();
  const title = manualSearchTitleInput.value.trim();
  const album = manualSearchAlbumInput.value.trim();

  if (!artist && !title && !album) {
    manualSearchStatus.textContent = 'Enter at least one of artist, title, or album.';
    return;
  }

  if (status === 'identified' && !confirm('This file is already identified. Searching will immediately discard its current resolved metadata, even if you don\'t pick a result. Continue?')) {
    return;
  }

  const query = buildManualSearchQuery(artist, title, album);
  manualSearchButton.disabled = true;
  manualSearchStatus.textContent = 'Searching…';
  try {
    const candidates = await searchIdentify(path, query);
    if (candidates.length === 0) {
      manualSearchStatus.textContent = 'No matches found.';
      return;
    }
    manualSearchStatus.textContent = '';
    manualSearchButton.dataset.status = 'ambiguous';
    detailsCandidatesHeading.textContent = 'Search results — choose the correct recording';
    renderCandidates(path, candidates);
    await refreshCurrentView();
  } catch (err) {
    manualSearchStatus.textContent = `Search failed: ${err.message}`;
  } finally {
    manualSearchButton.disabled = false;
  }
});

async function resolveCandidate(path, recordingMbid) {
  try {
    await postIdentifyResolve(path, recordingMbid);
    await refreshCurrentView();
    closeDetails();
  } catch (err) {
    detailsCandidates.textContent = `Failed to resolve candidate: ${err.message}`;
  }
}

detailsBrowseCoversToggle.addEventListener('click', () => {
  const path = detailsBrowseCoversToggle.dataset.path;
  if (!detailsCoverCandidatesSection.classList.contains('hidden')) {
    detailsCoverCandidatesSection.classList.add('hidden');
    detailsBrowseCoversToggle.textContent = 'Browse other covers…';
    return;
  }
  loadCoverCandidates(path);
});

async function loadCoverCandidates(path) {
  detailsBrowseCoversToggle.textContent = 'Loading covers…';
  detailsBrowseCoversToggle.disabled = true;
  try {
    const candidates = await fetchCoverCandidates(path);

    detailsCoverCandidates.innerHTML = '';
    if (candidates.length === 0) {
      detailsCoverCandidates.innerHTML = '<p class="col-span-4 text-neutral-500 text-xs">No alternate covers found across this release group.</p>';
    }
    for (const candidate of candidates) {
      const cell = document.createElement('button');
      cell.className = 'cover-candidate-button rounded-md overflow-hidden border border-neutral-700 hover:border-blue-400';
      cell.title = candidate.release_title;
      cell.innerHTML = `<img src="${candidate.thumbnail_url}" class="w-full h-20 object-cover" alt="${escapeHtml(candidate.release_title)}" />`;
      cell.addEventListener('click', () => chooseCover(path, candidate.release_mbid, candidate.image_url));
      detailsCoverCandidates.appendChild(cell);
    }
    detailsCoverCandidatesSection.classList.remove('hidden');
    detailsBrowseCoversToggle.textContent = 'Hide alternate covers';
  } catch (err) {
    detailsCoverCandidates.textContent = `Failed to load cover candidates: ${err.message}`;
    detailsCoverCandidatesSection.classList.remove('hidden');
    detailsBrowseCoversToggle.textContent = 'Browse other covers…';
  } finally {
    detailsBrowseCoversToggle.disabled = false;
  }
}

async function chooseCover(path, releaseMbid, imageUrl) {
  try {
    await postCoverChoose(path, releaseMbid, imageUrl);
    await refreshCurrentView();
    detailsCover.src = `/api/v1/library/cover?path=${encodeURIComponent(path)}&_=${Date.now()}`;
    detailsCover.classList.remove('hidden');
  } catch (err) {
    detailsCoverCandidates.textContent = `Failed to choose cover: ${err.message}`;
  }
}

async function loadFingerprint(path) {
  try {
    const data = await fetchFingerprint(path);
    if (!data.fingerprint) {
      return;
    }
    const row = document.createElement('div');
    row.className = 'flex justify-between gap-4';
    row.innerHTML = `
      <dt class="text-neutral-400">Fingerprint</dt>
      <dd class="font-mono text-xs text-right break-all">${escapeHtml(data.fingerprint)}</dd>
    `;
    detailsFields.appendChild(row);
  } catch (err) {
    // Best-effort — the details view is still useful without it.
  }
}

async function loadEmbeddedTags(path) {
  try {
    const data = await fetchEmbeddedTags(path);

    detailsEmbeddedTags.innerHTML = '';
    for (const [key, label] of EMBEDDED_TAG_FIELD_LABELS) {
      const value = data[key];
      if (value === undefined || value === null || value === '') {
        continue;
      }
      const row = document.createElement('div');
      row.className = 'flex justify-between gap-4';
      row.innerHTML = `
        <dt class="text-neutral-400">${escapeHtml(label)}</dt>
        <dd class="font-mono text-xs text-right break-all">${escapeHtml(String(value))}</dd>
      `;
      detailsEmbeddedTags.appendChild(row);
    }
    const extras = [
      data.has_lyrics ? 'Lyrics embedded' : null,
      data.has_cover_art ? 'Cover art embedded' : null,
    ].filter(Boolean);
    if (extras.length > 0) {
      const row = document.createElement('div');
      row.className = 'text-neutral-400 text-xs';
      row.textContent = extras.join(' · ');
      detailsEmbeddedTags.appendChild(row);
    }
    detailsEmbeddedTagsSection.classList.remove('hidden');
  } catch (err) {
    detailsEmbeddedTags.textContent = `Failed to load embedded tags: ${err.message}`;
    detailsEmbeddedTagsSection.classList.remove('hidden');
  }
}

async function loadLyrics(path) {
  try {
    const data = await fetchLyrics(path);
    detailsLyrics.textContent = data.plain_lyrics || data.synced_lyrics || '';
    detailsLyricsSection.classList.remove('hidden');
  } catch (err) {
    detailsLyrics.textContent = `Failed to load lyrics: ${err.message}`;
    detailsLyricsSection.classList.remove('hidden');
  }
}

function closeDetails() {
  detailsOverlay.classList.add('hidden');
}

detailsClose.addEventListener('click', closeDetails);
detailsOverlay.addEventListener('click', (e) => {
  if (e.target === detailsOverlay) {
    closeDetails();
  }
});

export function initDetails(refreshCurrentViewCallback) {
  refreshCurrentView = refreshCurrentViewCallback;
}
