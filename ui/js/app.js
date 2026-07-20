const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('library-rows');
const refreshButton = document.getElementById('refresh-button');
const identifyButton = document.getElementById('identify-button');
const enrichButton = document.getElementById('enrich-button');
const tagButton = document.getElementById('tag-button');
const relocateButton = document.getElementById('relocate-button');
const selectAllCheckbox = document.getElementById('select-all');
const detailsOverlay = document.getElementById('details-overlay');
const detailsFields = document.getElementById('details-fields');
const detailsClose = document.getElementById('details-close');
const detailsCover = document.getElementById('details-cover');
const detailsLyricsSection = document.getElementById('details-lyrics-section');
const detailsLyrics = document.getElementById('details-lyrics');
const detailsEmbeddedTagsSection = document.getElementById('details-embedded-tags-section');
const detailsEmbeddedTags = document.getElementById('details-embedded-tags');

const filterStatusSelect = document.getElementById('filter-status');
const filterTaggedSelect = document.getElementById('filter-tagged');
const filterRelocatedSelect = document.getElementById('filter-relocated');
const filterSearchInput = document.getElementById('filter-search');
const pageSizeSelect = document.getElementById('page-size');
const paginationInfo = document.getElementById('pagination-info');
const prevPageButton = document.getElementById('prev-page');
const nextPageButton = document.getElementById('next-page');
const selectionBanner = document.getElementById('selection-banner');
const selectionBannerText = document.getElementById('selection-banner-text');
const selectAllMatchingButton = document.getElementById('select-all-matching');
const clearSelectionButton = document.getElementById('clear-selection');

const DETAILS_FIELD_LABELS = [
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

const STATUS_LABELS = {
  new: 'New',
  identified: 'Identified',
  not_found: 'Not Found',
  missing: 'Missing',
};

const STATUS_CLASSES = {
  new: 'text-blue-400',
  identified: 'text-green-400',
  not_found: 'text-yellow-400',
  missing: 'text-neutral-500',
};

const EMBEDDED_TAG_FIELD_LABELS = [
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
const IDENTIFY_ETA_THRESHOLD = 20;

const selectedPaths = new Set();

// 'explicit': selectedPaths enumerates exactly what's selected, across
// however many pages the user has visited. 'filter': every file currently
// matching filterState is considered selected, however many that is — the
// server re-resolves the matching set at execution time (see
// resolveSelection on the API side), so this never enumerates paths
// client-side.
let selectionMode = 'explicit';

let filterState = { status: '', tagged: '', relocated: '', q: '' };
let sortState = { by: 'path', desc: false };
let pageState = { limit: 50, offset: 0 };
let total = 0;
let searchDebounceTimer = null;

let scanPollTimer = null;
let identifyPollTimer = null;
let identifyRunning = false;
let enrichPollTimer = null;
let enrichRunning = false;
let tagPollTimer = null;
let tagRunning = false;
let relocatePollTimer = null;
let relocateRunning = false;
let scanRunning = false;
let lastEntries = [];

function buildListParams() {
  const params = new URLSearchParams();
  if (filterState.status) params.set('status', filterState.status);
  if (filterState.tagged !== '') params.set('tagged', filterState.tagged);
  if (filterState.relocated !== '') params.set('relocated', filterState.relocated);
  if (filterState.q) params.set('q', filterState.q);
  if (sortState.by) params.set('sort', sortState.by);
  params.set('order', sortState.desc ? 'desc' : 'asc');
  params.set('limit', String(pageState.limit));
  params.set('offset', String(pageState.offset));
  return params;
}

function currentFilterPayload() {
  const filter = { status: filterState.status, q: filterState.q };
  if (filterState.tagged !== '') filter.tagged = filterState.tagged === 'true';
  if (filterState.relocated !== '') filter.relocated = filterState.relocated === 'true';
  return filter;
}

function getSelectionCount() {
  return selectionMode === 'filter' ? total : selectedPaths.size;
}

function getSelectionBody() {
  if (selectionMode === 'filter') {
    return { filter: currentFilterPayload() };
  }
  return { paths: [...selectedPaths] };
}

async function loadLibrary() {
  try {
    const params = buildListParams();
    const res = await fetch(`/api/v1/library?${params.toString()}`);
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const data = await res.json();
    total = data.total || 0;
    renderTable(data.entries || []);
    updatePaginationControls();
  } catch (err) {
    statusEl.textContent = `Failed to load library: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

function renderTable(entries) {
  lastEntries = entries;

  rowsEl.innerHTML = '';

  if (entries.length === 0) {
    statusEl.textContent = total === 0 ? 'No files match the current filters.' : 'No tracked files yet.';
    statusEl.className = 'text-neutral-400 mb-4';
    selectAllCheckbox.checked = false;
    selectAllCheckbox.disabled = selectionMode === 'filter';
    updateAllActionButtons();
    updateSelectionBanner();
    return;
  }

  statusEl.textContent = `Showing ${entries.length} of ${total} tracked file(s).`;
  statusEl.className = 'text-neutral-400 mb-4';

  for (const entry of entries) {
    rowsEl.appendChild(renderRow(entry));
  }

  selectAllCheckbox.checked = selectionMode === 'filter' || entries.every((e) => selectedPaths.has(e.path));
  selectAllCheckbox.disabled = selectionMode === 'filter';

  updateAllActionButtons();
  updateSelectionBanner();
}

function renderRow(entry) {
  const row = document.createElement('tr');
  row.dataset.path = entry.path;
  row.classList.add('cursor-pointer', 'hover:bg-neutral-900');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const checked = (selectionMode === 'filter' || selectedPaths.has(entry.path)) ? 'checked' : '';
  const disabledAttr = selectionMode === 'filter' ? 'disabled' : '';
  const checkboxCell = `<td class="px-4 py-3"><input type="checkbox" class="row-checkbox" data-path="${escapeHtml(entry.path)}" ${checked} ${disabledAttr} /></td>`;
  const coverCell = renderCoverCell(entry);
  const metadataCell = renderMetadataCell(entry);
  const lyricsCell = entry.has_lyrics ? '<span class="text-green-400" title="Lyrics available">&#9834;</span>' : '—';
  const taggedCell = renderTaggedCell(entry);
  const relocatedCell = renderRelocatedCell(entry);
  const actionsCell = renderActionsCell(entry);

  if (entry.error) {
    row.classList.add('text-red-400');
    row.innerHTML = `
      ${checkboxCell}
      <td class="px-4 py-3">${coverCell}</td>
      <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
      <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">Error: ${escapeHtml(entry.error)}</td>
      <td class="px-4 py-3">${metadataCell}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">${taggedCell}</td>
      <td class="px-4 py-3">${relocatedCell}</td>
      <td class="px-4 py-3">${actionsCell}</td>
    `;
    return row;
  }

  row.innerHTML = `
    ${checkboxCell}
    <td class="px-4 py-3">${coverCell}</td>
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 ${statusClass}">${escapeHtml(statusLabel)}</td>
    <td class="px-4 py-3">${metadataCell}</td>
    <td class="px-4 py-3">${lyricsCell}</td>
    <td class="px-4 py-3">${taggedCell}</td>
    <td class="px-4 py-3">${relocatedCell}</td>
    <td class="px-4 py-3">${actionsCell}</td>
  `;
  return row;
}

function renderTaggedCell(entry) {
  if (entry.tagged) {
    return '<span class="text-green-400" title="Tagged">&#10003;</span>';
  }
  if (entry.tag_error) {
    return `<span class="text-red-400" title="Tagging failed: ${escapeHtml(entry.tag_error)}">&#10007;</span>`;
  }
  return '—';
}

function renderRelocatedCell(entry) {
  if (entry.relocated) {
    return '<span class="text-green-400" title="Relocated">&#10003;</span>';
  }
  if (entry.relocate_error) {
    return `<span class="text-red-400" title="Relocation failed: ${escapeHtml(entry.relocate_error)}">&#10007;</span>`;
  }
  return '—';
}

function renderActionsCell(entry) {
  if (entry.status !== 'missing') {
    return '—';
  }
  return `<button class="delete-entry-button text-red-400 hover:text-red-300" data-path="${escapeHtml(entry.path)}" title="Delete this tracked entry (the file is already missing from disk)">&#128465;</button>`;
}

function renderCoverCell(entry) {
  if (!entry.has_cover_art) {
    return '<div class="w-10 h-10 rounded bg-neutral-800"></div>';
  }
  const src = `/api/v1/library/cover?path=${encodeURIComponent(entry.path)}`;
  return `<img src="${src}" class="w-10 h-10 rounded object-cover" alt="" />`;
}

function renderMetadataCell(entry) {
  if (entry.status !== 'identified') {
    return '—';
  }
  const track = entry.track_number ? `Track ${entry.track_number}` : '';
  return escapeHtml([entry.artist, entry.album, entry.title, track].filter(Boolean).join(' – '));
}

function formatDuration(seconds) {
  const total = Math.round(seconds || 0);
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
}

function formatEta(totalSeconds) {
  const totalMinutes = Math.ceil(totalSeconds / 60);
  if (totalMinutes < 60) {
    return `~${totalMinutes} minute${totalMinutes === 1 ? '' : 's'}`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes === 0 ? `~${hours} hour${hours === 1 ? '' : 's'}` : `~${hours}h ${minutes}m`;
}

function escapeHtml(value) {
  const div = document.createElement('div');
  div.textContent = value ?? '';
  return div.innerHTML;
}

rowsEl.addEventListener('change', (e) => {
  if (!e.target.matches('.row-checkbox')) {
    return;
  }
  if (selectionMode === 'filter') {
    // The user is making an explicit choice — drop out of "all matching"
    // mode and seed explicit selection with every row currently checked
    // (which, in filter mode, was every row on this page).
    selectionMode = 'explicit';
    for (const checkbox of rowsEl.querySelectorAll('.row-checkbox')) {
      checkbox.disabled = false;
      if (checkbox.checked) {
        selectedPaths.add(checkbox.dataset.path);
      }
    }
  }
  const path = e.target.dataset.path;
  if (e.target.checked) {
    selectedPaths.add(path);
  } else {
    selectedPaths.delete(path);
  }
  selectAllCheckbox.checked = lastEntries.length > 0 && lastEntries.every((entry) => selectedPaths.has(entry.path));
  updateAllActionButtons();
  updateSelectionBanner();
});

rowsEl.addEventListener('click', (e) => {
  if (e.target.closest('.delete-entry-button')) {
    e.stopPropagation();
    deleteEntry(e.target.closest('.delete-entry-button').dataset.path);
    return;
  }
  // Clicking the selection checkbox (or anything inside it) only toggles
  // selection — it must not also open the details view.
  if (e.target.closest('input')) {
    return;
  }
  const row = e.target.closest('tr');
  if (!row) {
    return;
  }
  openDetails(row.dataset.path);
});

async function deleteEntry(path) {
  if (!confirm(`Delete the tracked entry for:\n${path}\n\nThis only removes it from tracking — it does not affect any file on disk (the file is already missing).`)) {
    return;
  }
  try {
    const res = await fetch(`/api/v1/library/entry?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
    if (res.status !== 204 && res.status !== 200) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `delete request failed: ${res.status}`);
    }
    selectedPaths.delete(path);
    await loadLibrary();
  } catch (err) {
    statusEl.textContent = `Failed to delete entry: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function openDetails(path) {
  const entry = lastEntries.find((e) => e.path === path);
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

  await loadFingerprint(entry.path);

  detailsOverlay.classList.remove('hidden');
}

async function loadFingerprint(path) {
  try {
    const res = await fetch(`/api/v1/library/fingerprint?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const data = await res.json();
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
    const res = await fetch(`/api/v1/library/tags?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const data = await res.json();

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
    const res = await fetch(`/api/v1/library/lyrics?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const data = await res.json();
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

selectAllCheckbox.addEventListener('change', () => {
  selectionMode = 'explicit';
  const checkboxes = rowsEl.querySelectorAll('.row-checkbox');
  for (const checkbox of checkboxes) {
    checkbox.disabled = false;
    checkbox.checked = selectAllCheckbox.checked;
    if (selectAllCheckbox.checked) {
      selectedPaths.add(checkbox.dataset.path);
    } else {
      selectedPaths.delete(checkbox.dataset.path);
    }
  }
  updateAllActionButtons();
  updateSelectionBanner();
});

selectAllMatchingButton.addEventListener('click', () => {
  selectionMode = 'filter';
  renderTable(lastEntries);
});

clearSelectionButton.addEventListener('click', () => {
  selectionMode = 'explicit';
  selectedPaths.clear();
  renderTable(lastEntries);
});

// The banner distinguishes three states: nothing selected (hidden), an
// explicit set of paths selected (possibly spanning past pages), and
// "every file matching the current filter" selected — the latter always
// reads its count from `total`, which is re-fetched with every filter
// change, so the displayed count and what a bulk action would actually
// process never drift apart.
function updateSelectionBanner() {
  if (selectionMode === 'filter') {
    selectionBanner.classList.remove('hidden');
    selectionBannerText.textContent = `All ${total} matching file(s) selected.`;
    selectAllMatchingButton.classList.add('hidden');
    clearSelectionButton.classList.remove('hidden');
    return;
  }

  const pageCheckboxes = rowsEl.querySelectorAll('.row-checkbox');
  const allPageSelected = pageCheckboxes.length > 0 && [...pageCheckboxes].every((cb) => selectedPaths.has(cb.dataset.path));

  if (selectedPaths.size === 0) {
    selectionBanner.classList.add('hidden');
    return;
  }

  selectionBanner.classList.remove('hidden');
  selectionBannerText.textContent = `${selectedPaths.size} selected.`;
  clearSelectionButton.classList.remove('hidden');

  if (allPageSelected && total > selectedPaths.size) {
    selectAllMatchingButton.textContent = `Select all ${total} matching`;
    selectAllMatchingButton.classList.remove('hidden');
  } else {
    selectAllMatchingButton.classList.add('hidden');
  }
}

function updateAllActionButtons() {
  updateIdentifyButton();
  updateEnrichButton();
  updateTagButton();
  updateRelocateButton();
}

function updateIdentifyButton() {
  identifyButton.disabled = identifyRunning || getSelectionCount() === 0;
  if (!identifyRunning) {
    identifyButton.textContent = 'Identify Selected';
  }
}

function updateEnrichButton() {
  enrichButton.disabled = enrichRunning || getSelectionCount() === 0;
  if (!enrichRunning) {
    enrichButton.textContent = 'Enrich Selected';
  }
}

function updateTagButton() {
  tagButton.disabled = tagRunning || getSelectionCount() === 0;
  if (!tagRunning) {
    tagButton.textContent = 'Tag Selected';
  }
}

// Relocate and scan mutually exclude each other (a scan walking /music
// concurrently with a file being moved could see it as both missing at
// its old location and new at its new one) — the relocate action is
// disabled while a scan is running, and the refresh trigger is disabled
// while a relocate job is running, mirroring what the API itself rejects.
function updateRelocateButton() {
  relocateButton.disabled = relocateRunning || scanRunning || getSelectionCount() === 0;
  if (!relocateRunning) {
    relocateButton.textContent = 'Relocate Selected';
  }
}

function updateRefreshButton() {
  refreshButton.disabled = scanRunning || relocateRunning;
  if (!scanRunning) {
    refreshButton.textContent = 'Refresh';
  }
}

function updatePaginationControls() {
  if (total === 0) {
    paginationInfo.textContent = 'No matching files.';
  } else {
    const start = pageState.offset + 1;
    const end = Math.min(pageState.offset + pageState.limit, total);
    paginationInfo.textContent = `${start}–${end} of ${total}`;
  }
  prevPageButton.disabled = pageState.offset === 0;
  nextPageButton.disabled = pageState.offset + pageState.limit >= total;
}

function updateSortIndicators() {
  document.querySelectorAll('[data-sort-indicator]').forEach((el) => {
    const col = el.dataset.sortIndicator;
    el.textContent = sortState.by === col ? (sortState.desc ? ' ▼' : ' ▲') : '';
  });
}

document.querySelector('thead').addEventListener('click', (e) => {
  const target = e.target.closest('[data-sort]');
  if (!target) {
    return;
  }
  const col = target.dataset.sort;
  if (sortState.by === col) {
    sortState.desc = !sortState.desc;
  } else {
    sortState.by = col;
    sortState.desc = false;
  }
  pageState.offset = 0;
  updateSortIndicators();
  loadLibrary();
});
updateSortIndicators();

filterStatusSelect.addEventListener('change', () => {
  filterState.status = filterStatusSelect.value;
  pageState.offset = 0;
  loadLibrary();
});

filterTaggedSelect.addEventListener('change', () => {
  filterState.tagged = filterTaggedSelect.value;
  pageState.offset = 0;
  loadLibrary();
});

filterRelocatedSelect.addEventListener('change', () => {
  filterState.relocated = filterRelocatedSelect.value;
  pageState.offset = 0;
  loadLibrary();
});

filterSearchInput.addEventListener('input', () => {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    filterState.q = filterSearchInput.value.trim();
    pageState.offset = 0;
    loadLibrary();
  }, 300);
});

pageSizeSelect.addEventListener('change', () => {
  pageState.limit = Number(pageSizeSelect.value);
  pageState.offset = 0;
  loadLibrary();
});

prevPageButton.addEventListener('click', () => {
  pageState.offset = Math.max(0, pageState.offset - pageState.limit);
  loadLibrary();
});

nextPageButton.addEventListener('click', () => {
  pageState.offset += pageState.limit;
  loadLibrary();
});

async function fetchScanStatus() {
  const res = await fetch('/api/v1/library/scan/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

async function fetchIdentifyStatus() {
  const res = await fetch('/api/v1/library/identify/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

async function fetchEnrichStatus() {
  const res = await fetch('/api/v1/library/enrich/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

async function fetchTagStatus() {
  const res = await fetch('/api/v1/library/tag/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

async function fetchRelocateStatus() {
  const res = await fetch('/api/v1/library/relocate/status');
  if (!res.ok) {
    throw new Error(`status request failed: ${res.status}`);
  }
  return res.json();
}

function setScanningUI(running, processed, total) {
  scanRunning = running;
  if (running) {
    refreshButton.textContent = 'Scanning…';
    statusEl.textContent = total > 0
      ? `Scanning… ${processed}/${total} fingerprinted`
      : 'Scanning…';
    statusEl.className = 'text-neutral-400 mb-4';
  }
  updateRefreshButton();
  updateRelocateButton();
}

function setIdentifyingUI(running, processed, total) {
  identifyRunning = running;
  updateIdentifyButton();
  if (running) {
    identifyButton.textContent = total > 0 ? `Identifying ${processed}/${total}…` : 'Identifying…';
  }
}

function setEnrichingUI(running, processed, total) {
  enrichRunning = running;
  updateEnrichButton();
  if (running) {
    enrichButton.textContent = total > 0 ? `Enriching ${processed}/${total}…` : 'Enriching…';
  }
}

function setTaggingUI(running, processed, total) {
  tagRunning = running;
  updateTagButton();
  if (running) {
    tagButton.textContent = total > 0 ? `Tagging ${processed}/${total}…` : 'Tagging…';
  }
}

function setRelocatingUI(running, processed, total) {
  relocateRunning = running;
  updateRelocateButton();
  updateRefreshButton();
  if (running) {
    relocateButton.textContent = total > 0 ? `Relocating ${processed}/${total}…` : 'Relocating…';
  }
}

function startScanPolling() {
  if (scanPollTimer) {
    return;
  }
  scanPollTimer = setInterval(async () => {
    try {
      const status = await fetchScanStatus();
      setScanningUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(scanPollTimer);
        scanPollTimer = null;
      }
    } catch (err) {
      clearInterval(scanPollTimer);
      scanPollTimer = null;
    }
  }, 1000);
}

function startIdentifyPolling() {
  if (identifyPollTimer) {
    return;
  }
  identifyPollTimer = setInterval(async () => {
    try {
      const status = await fetchIdentifyStatus();
      setIdentifyingUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(identifyPollTimer);
        identifyPollTimer = null;
      }
    } catch (err) {
      clearInterval(identifyPollTimer);
      identifyPollTimer = null;
    }
  }, 1000);
}

function startEnrichPolling() {
  if (enrichPollTimer) {
    return;
  }
  enrichPollTimer = setInterval(async () => {
    try {
      const status = await fetchEnrichStatus();
      setEnrichingUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(enrichPollTimer);
        enrichPollTimer = null;
      }
    } catch (err) {
      clearInterval(enrichPollTimer);
      enrichPollTimer = null;
    }
  }, 1000);
}

function startTagPolling() {
  if (tagPollTimer) {
    return;
  }
  tagPollTimer = setInterval(async () => {
    try {
      const status = await fetchTagStatus();
      setTaggingUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(tagPollTimer);
        tagPollTimer = null;
      }
    } catch (err) {
      clearInterval(tagPollTimer);
      tagPollTimer = null;
    }
  }, 1000);
}

function startRelocatePolling() {
  if (relocatePollTimer) {
    return;
  }
  relocatePollTimer = setInterval(async () => {
    try {
      const status = await fetchRelocateStatus();
      setRelocatingUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(relocatePollTimer);
        relocatePollTimer = null;
      }
    } catch (err) {
      clearInterval(relocatePollTimer);
      relocatePollTimer = null;
    }
  }, 1000);
}

async function triggerRefresh() {
  try {
    const res = await fetch('/api/v1/library/scan', { method: 'POST' });
    if (res.status !== 202 && res.status !== 409) {
      throw new Error(`refresh request failed: ${res.status}`);
    }
    // 202: we started it. 409: one was already running — either way, a
    // refresh is now in flight, so start observing it.
    setScanningUI(true, 0, 0);
    startScanPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start refresh: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function triggerIdentify() {
  const count = getSelectionCount();
  if (count === 0) {
    return;
  }
  if (count > IDENTIFY_ETA_THRESHOLD) {
    const eta = formatEta(count); // MusicBrainz: ~1 request/second
    if (!confirm(`Identifying ${count} file(s) will take about ${eta} (MusicBrainz allows 1 request/second). Continue?`)) {
      return;
    }
  }
  try {
    const res = await fetch('/api/v1/library/identify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(getSelectionBody()),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `identify request failed: ${res.status}`);
    }
    setIdentifyingUI(true, 0, count);
    startIdentifyPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start identification: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function triggerEnrich() {
  const count = getSelectionCount();
  if (count === 0) {
    return;
  }
  try {
    const res = await fetch('/api/v1/library/enrich', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(getSelectionBody()),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `enrich request failed: ${res.status}`);
    }
    setEnrichingUI(true, 0, count);
    startEnrichPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start enrichment: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function triggerTag() {
  const count = getSelectionCount();
  if (count === 0) {
    return;
  }
  try {
    const res = await fetch('/api/v1/library/tag', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(getSelectionBody()),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `tag request failed: ${res.status}`);
    }
    setTaggingUI(true, 0, count);
    startTagPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start tagging: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function triggerRelocate() {
  const count = getSelectionCount();
  if (count === 0) {
    return;
  }
  try {
    const res = await fetch('/api/v1/library/relocate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(getSelectionBody()),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `relocate request failed: ${res.status}`);
    }
    setRelocatingUI(true, 0, count);
    startRelocatePolling();
  } catch (err) {
    statusEl.textContent = `Failed to start relocation: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

refreshButton.addEventListener('click', triggerRefresh);
identifyButton.addEventListener('click', triggerIdentify);
enrichButton.addEventListener('click', triggerEnrich);
tagButton.addEventListener('click', triggerTag);
relocateButton.addEventListener('click', triggerRelocate);

(async function init() {
  await loadLibrary();
  try {
    const scanStatus = await fetchScanStatus();
    setScanningUI(scanStatus.running, scanStatus.processed, scanStatus.total);
    if (scanStatus.running) {
      startScanPolling();
    }
  } catch (err) {
    // Status endpoint unreachable — leave the button enabled; the user can
    // still try to trigger a refresh manually.
  }
  try {
    const identifyStatus = await fetchIdentifyStatus();
    setIdentifyingUI(identifyStatus.running, identifyStatus.processed, identifyStatus.total);
    if (identifyStatus.running) {
      startIdentifyPolling();
    }
  } catch (err) {
    // Status endpoint unreachable — identify button stays disabled until a
    // selection is made anyway.
  }
  try {
    const enrichStatus = await fetchEnrichStatus();
    setEnrichingUI(enrichStatus.running, enrichStatus.processed, enrichStatus.total);
    if (enrichStatus.running) {
      startEnrichPolling();
    }
  } catch (err) {
    // Status endpoint unreachable — enrich button stays disabled until a
    // selection is made anyway.
  }
  try {
    const tagStatus = await fetchTagStatus();
    setTaggingUI(tagStatus.running, tagStatus.processed, tagStatus.total);
    if (tagStatus.running) {
      startTagPolling();
    }
  } catch (err) {
    // Status endpoint unreachable — tag button stays disabled until a
    // selection is made anyway.
  }
  try {
    const relocateStatus = await fetchRelocateStatus();
    setRelocatingUI(relocateStatus.running, relocateStatus.processed, relocateStatus.total);
    if (relocateStatus.running) {
      startRelocatePolling();
    }
  } catch (err) {
    // Status endpoint unreachable — relocate button stays disabled until a
    // selection is made anyway.
  }
})();
