async function loadLibrary() {
  const statusEl = document.getElementById('status');
  const rowsEl = document.getElementById('library-rows');

  try {
    const res = await fetch('/api/v1/library');
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    const entries = await res.json();

    if (entries.length === 0) {
      statusEl.textContent = 'No supported audio files found under /music.';
      return;
    }

    statusEl.textContent = `${entries.length} file(s) found.`;

    for (const entry of entries) {
      rowsEl.appendChild(renderRow(entry));
    }
  } catch (err) {
    statusEl.textContent = `Failed to load library: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

function renderRow(entry) {
  const row = document.createElement('tr');

  if (entry.error) {
    row.className = 'text-red-400';
    row.innerHTML = `
      <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
      <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
      <td class="px-4 py-3">—</td>
      <td class="px-4 py-3">Error: ${escapeHtml(entry.error)}</td>
    `;
    return row;
  }

  row.innerHTML = `
    <td class="px-4 py-3 font-mono text-xs">${escapeHtml(entry.path)}</td>
    <td class="px-4 py-3 uppercase">${escapeHtml(entry.format)}</td>
    <td class="px-4 py-3">${formatDuration(entry.duration_seconds)}</td>
    <td class="px-4 py-3 font-mono text-xs truncate max-w-xs" title="${escapeHtml(entry.fingerprint)}">${escapeHtml(entry.fingerprint)}</td>
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

loadLibrary();
