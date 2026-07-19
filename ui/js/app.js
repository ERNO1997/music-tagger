const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('library-rows');
const refreshButton = document.getElementById('refresh-button');
const identifyButton = document.getElementById('identify-button');
const enrichButton = document.getElementById('enrich-button');
const selectAllCheckbox = document.getElementById('select-all');
const detailsOverlay = document.getElementById('details-overlay');
const detailsFields = document.getElementById('details-fields');
const detailsClose = document.getElementById('details-close');
const detailsCover = document.getElementById('details-cover');
const detailsLyricsSection = document.getElementById('details-lyrics-section');
const detailsLyrics = document.getElementById('details-lyrics');

const DETAILS_FIELD_LABELS = [
  ['path', 'Path'],
  ['format', 'Format'],
  ['duration_seconds', 'Duration', formatDuration],
  ['fingerprint', 'Fingerprint'],
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

const selectedPaths = new Set();
let scanPollTimer = null;
let identifyPollTimer = null;
let identifyRunning = false;
let enrichPollTimer = null;
let enrichRunning = false;
let lastEntries = [];

async function loadLibrary() {
  try {
    const res = await fetch('/api/v1/library');
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const entries = await res.json();
    renderTable(entries);
  } catch (err) {
    statusEl.textContent = `Failed to load library: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

function renderTable(entries) {
  lastEntries = entries;

  const knownPaths = new Set(entries.map((e) => e.path));
  for (const path of [...selectedPaths]) {
    if (!knownPaths.has(path)) {
      selectedPaths.delete(path);
    }
  }

  rowsEl.innerHTML = '';

  if (entries.length === 0) {
    statusEl.textContent = 'No tracked files yet.';
    statusEl.className = 'text-neutral-400 mb-4';
    updateIdentifyButton();
    updateEnrichButton();
    return;
  }

  statusEl.textContent = `${entries.length} file(s) tracked.`;
  statusEl.className = 'text-neutral-400 mb-4';

  for (const entry of entries) {
    rowsEl.appendChild(renderRow(entry));
  }
  updateIdentifyButton();
  updateEnrichButton();
}

function renderRow(entry) {
  const row = document.createElement('tr');
  row.dataset.path = entry.path;
  row.classList.add('cursor-pointer', 'hover:bg-neutral-900');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const checked = selectedPaths.has(entry.path) ? 'checked' : '';
  const checkboxCell = `<td class="px-4 py-3"><input type="checkbox" class="row-checkbox" data-path="${escapeHtml(entry.path)}" ${checked} /></td>`;
  const coverCell = renderCoverCell(entry);
  const metadataCell = renderMetadataCell(entry);
  const lyricsCell = entry.has_lyrics ? '<span class="text-green-400" title="Lyrics available">&#9834;</span>' : '—';

  if (entry.error) {
    row.classList.add('text-red-400');
    row.innerHTML = `
      ${checkboxCell}
      <td class="px-4 py-3">${coverCell}</td>
      <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
      <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">Error: ${escapeHtml(entry.error)}</td>
      <td class="px-4 py-3">${escapeHtml(statusLabel)}</td>
      <td class="px-4 py-3">${metadataCell}</td>
      <td class="px-4 py-3">—</td>
    `;
    return row;
  }

  row.innerHTML = `
    ${checkboxCell}
    <td class="px-4 py-3">${coverCell}</td>
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 font-mono text-xs truncate max-w-xs" title="${escapeHtml(entry.fingerprint)}">${escapeHtml(entry.fingerprint)}</td>
    <td class="px-4 py-3 ${statusClass}">${escapeHtml(statusLabel)}</td>
    <td class="px-4 py-3">${metadataCell}</td>
    <td class="px-4 py-3">${lyricsCell}</td>
  `;
  return row;
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

function escapeHtml(value) {
  const div = document.createElement('div');
  div.textContent = value ?? '';
  return div.innerHTML;
}

rowsEl.addEventListener('change', (e) => {
  if (!e.target.matches('.row-checkbox')) {
    return;
  }
  const path = e.target.dataset.path;
  if (e.target.checked) {
    selectedPaths.add(path);
  } else {
    selectedPaths.delete(path);
  }
  updateIdentifyButton();
  updateEnrichButton();
});

rowsEl.addEventListener('click', (e) => {
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

  detailsOverlay.classList.remove('hidden');
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
  const checkboxes = rowsEl.querySelectorAll('.row-checkbox');
  for (const checkbox of checkboxes) {
    checkbox.checked = selectAllCheckbox.checked;
    if (selectAllCheckbox.checked) {
      selectedPaths.add(checkbox.dataset.path);
    } else {
      selectedPaths.delete(checkbox.dataset.path);
    }
  }
  updateIdentifyButton();
  updateEnrichButton();
});

function updateIdentifyButton() {
  identifyButton.disabled = identifyRunning || selectedPaths.size === 0;
  if (!identifyRunning) {
    identifyButton.textContent = 'Identify Selected';
  }
}

function updateEnrichButton() {
  enrichButton.disabled = enrichRunning || selectedPaths.size === 0;
  if (!enrichRunning) {
    enrichButton.textContent = 'Enrich Selected';
  }
}

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

function setScanningUI(running, processed, total) {
  refreshButton.disabled = running;
  if (running) {
    refreshButton.textContent = 'Scanning…';
    statusEl.textContent = total > 0
      ? `Scanning… ${processed}/${total} fingerprinted`
      : 'Scanning…';
    statusEl.className = 'text-neutral-400 mb-4';
  } else {
    refreshButton.textContent = 'Refresh';
  }
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
  const paths = [...selectedPaths];
  if (paths.length === 0) {
    return;
  }
  try {
    const res = await fetch('/api/v1/library/identify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths }),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `identify request failed: ${res.status}`);
    }
    setIdentifyingUI(true, 0, paths.length);
    startIdentifyPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start identification: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

async function triggerEnrich() {
  const paths = [...selectedPaths];
  if (paths.length === 0) {
    return;
  }
  try {
    const res = await fetch('/api/v1/library/enrich', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths }),
    });
    if (res.status !== 202 && res.status !== 409) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `enrich request failed: ${res.status}`);
    }
    setEnrichingUI(true, 0, paths.length);
    startEnrichPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start enrichment: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

refreshButton.addEventListener('click', triggerRefresh);
identifyButton.addEventListener('click', triggerIdentify);
enrichButton.addEventListener('click', triggerEnrich);

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
})();
