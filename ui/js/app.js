const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('library-rows');
const refreshButton = document.getElementById('refresh-button');
const identifyButton = document.getElementById('identify-button');
const selectAllCheckbox = document.getElementById('select-all');

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
    return;
  }

  statusEl.textContent = `${entries.length} file(s) tracked.`;
  statusEl.className = 'text-neutral-400 mb-4';

  for (const entry of entries) {
    rowsEl.appendChild(renderRow(entry));
  }
  updateIdentifyButton();
}

function renderRow(entry) {
  const row = document.createElement('tr');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const checked = selectedPaths.has(entry.path) ? 'checked' : '';
  const checkboxCell = `<td class="px-4 py-3"><input type="checkbox" class="row-checkbox" data-path="${escapeHtml(entry.path)}" ${checked} /></td>`;
  const metadataCell = renderMetadataCell(entry);

  if (entry.error) {
    row.className = 'text-red-400';
    row.innerHTML = `
      ${checkboxCell}
      <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
      <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">Error: ${escapeHtml(entry.error)}</td>
      <td class="px-4 py-3">${escapeHtml(statusLabel)}</td>
      <td class="px-4 py-3">${metadataCell}</td>
    `;
    return row;
  }

  row.innerHTML = `
    ${checkboxCell}
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 font-mono text-xs truncate max-w-xs" title="${escapeHtml(entry.fingerprint)}">${escapeHtml(entry.fingerprint)}</td>
    <td class="px-4 py-3 ${statusClass}">${escapeHtml(statusLabel)}</td>
    <td class="px-4 py-3">${metadataCell}</td>
  `;
  return row;
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
});

function updateIdentifyButton() {
  identifyButton.disabled = identifyRunning || selectedPaths.size === 0;
  if (!identifyRunning) {
    identifyButton.textContent = 'Identify Selected';
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

refreshButton.addEventListener('click', triggerRefresh);
identifyButton.addEventListener('click', triggerIdentify);

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
})();
