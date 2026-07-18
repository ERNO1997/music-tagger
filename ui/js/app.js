const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('library-rows');
const refreshButton = document.getElementById('refresh-button');

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

let pollTimer = null;

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
  rowsEl.innerHTML = '';

  if (entries.length === 0) {
    statusEl.textContent = 'No tracked files yet.';
    statusEl.className = 'text-neutral-400 mb-4';
    return;
  }

  statusEl.textContent = `${entries.length} file(s) tracked.`;
  statusEl.className = 'text-neutral-400 mb-4';

  for (const entry of entries) {
    rowsEl.appendChild(renderRow(entry));
  }
}

function renderRow(entry) {
  const row = document.createElement('tr');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';

  if (entry.error) {
    row.className = 'text-red-400';
    row.innerHTML = `
      <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
      <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">Error: ${escapeHtml(entry.error)}</td>
      <td class="px-4 py-3">${escapeHtml(statusLabel)}</td>
    `;
    return row;
  }

  row.innerHTML = `
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 font-mono text-xs truncate max-w-xs" title="${escapeHtml(entry.fingerprint)}">${escapeHtml(entry.fingerprint)}</td>
    <td class="px-4 py-3 ${statusClass}">${escapeHtml(statusLabel)}</td>
  `;
  return row;
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

async function fetchScanStatus() {
  const res = await fetch('/api/v1/library/scan/status');
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

function startPolling() {
  if (pollTimer) {
    return;
  }
  pollTimer = setInterval(async () => {
    try {
      const status = await fetchScanStatus();
      setScanningUI(status.running, status.processed, status.total);
      await loadLibrary();
      if (!status.running) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    } catch (err) {
      clearInterval(pollTimer);
      pollTimer = null;
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
    startPolling();
  } catch (err) {
    statusEl.textContent = `Failed to start refresh: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

refreshButton.addEventListener('click', triggerRefresh);

(async function init() {
  await loadLibrary();
  try {
    const status = await fetchScanStatus();
    setScanningUI(status.running, status.processed, status.total);
    if (status.running) {
      startPolling();
    }
  } catch (err) {
    // Status endpoint unreachable — leave the button enabled; the user can
    // still try to trigger a refresh manually.
  }
})();
