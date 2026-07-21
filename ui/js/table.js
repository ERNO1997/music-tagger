import { state, buildListParams } from './state.js';
import { formatDuration, escapeHtml, STATUS_LABELS, STATUS_CLASSES } from './format.js';
import { fetchLibrary, deleteLibraryEntry } from './api.js';
import { updateAllActionButtons } from './actions.js';

const statusEl = document.getElementById('status');
const rowsEl = document.getElementById('library-rows');
const selectAllCheckbox = document.getElementById('select-all');
const paginationInfo = document.getElementById('pagination-info');
const prevPageButton = document.getElementById('prev-page');
const nextPageButton = document.getElementById('next-page');
const selectionBanner = document.getElementById('selection-banner');
const selectionBannerText = document.getElementById('selection-banner-text');
const selectAllMatchingButton = document.getElementById('select-all-matching');
const clearSelectionButton = document.getElementById('clear-selection');

// Set by initTable(); invoked when a row is clicked to open its details view.
let onRowOpen = null;

// Set by initTable(); this is main.js's renderCurrentView, the single
// dispatch point add-library-views-and-playback will extend with
// grid/tree/artist-album cases. Defaults to renderTable so this module
// still works standalone (e.g. in tests) if initTable is never called.
let render = renderTable;

export async function loadLibrary() {
  try {
    const params = buildListParams();
    const data = await fetchLibrary(params);
    state.total = data.total || 0;
    render(data.entries || []);
    updatePaginationControls();
  } catch (err) {
    statusEl.textContent = `Failed to load library: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

export function renderTable(entries) {
  state.lastEntries = entries;

  rowsEl.innerHTML = '';

  if (entries.length === 0) {
    statusEl.textContent = state.total === 0 ? 'No files match the current filters.' : 'No tracked files yet.';
    statusEl.className = 'text-neutral-400 mb-4';
    selectAllCheckbox.checked = false;
    selectAllCheckbox.disabled = state.selectionMode === 'filter';
    updateAllActionButtons();
    updateSelectionBanner();
    return;
  }

  statusEl.textContent = `Showing ${entries.length} of ${state.total} tracked file(s).`;
  statusEl.className = 'text-neutral-400 mb-4';

  for (const entry of entries) {
    rowsEl.appendChild(renderRow(entry));
  }

  selectAllCheckbox.checked = state.selectionMode === 'filter' || entries.every((e) => state.selectedPaths.has(e.path));
  selectAllCheckbox.disabled = state.selectionMode === 'filter';

  updateAllActionButtons();
  updateSelectionBanner();
}

function renderRow(entry) {
  const row = document.createElement('tr');
  row.dataset.path = entry.path;
  row.classList.add('cursor-pointer', 'hover:bg-neutral-900');
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const checked = (state.selectionMode === 'filter' || state.selectedPaths.has(entry.path)) ? 'checked' : '';
  const disabledAttr = state.selectionMode === 'filter' ? 'disabled' : '';
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
  if (entry.status === 'identified') {
    const track = entry.track_number ? `Track ${entry.track_number}` : '';
    return escapeHtml([entry.artist, entry.album, entry.title, track].filter(Boolean).join(' – '));
  }
  const raw = [entry.raw_artist, entry.raw_album, entry.raw_title].filter(Boolean).join(' – ');
  if (!raw) {
    return '—';
  }
  return `<span class="italic text-neutral-500" title="From the file's own tags, not yet identified">${escapeHtml(raw)}</span>`;
}

async function deleteEntry(path) {
  if (!confirm(`Delete the tracked entry for:\n${path}\n\nThis only removes it from tracking — it does not affect any file on disk (the file is already missing).`)) {
    return;
  }
  try {
    await deleteLibraryEntry(path);
    state.selectedPaths.delete(path);
    await loadLibrary();
  } catch (err) {
    statusEl.textContent = `Failed to delete entry: ${err.message}`;
    statusEl.className = 'text-red-400 mb-4';
  }
}

// The banner distinguishes three states: nothing selected (hidden), an
// explicit set of paths selected (possibly spanning past pages), and
// "every file matching the current filter" selected — the latter always
// reads its count from `total`, which is re-fetched with every filter
// change, so the displayed count and what a bulk action would actually
// process never drift apart.
function updateSelectionBanner() {
  if (state.selectionMode === 'filter') {
    selectionBanner.classList.remove('hidden');
    selectionBannerText.textContent = `All ${state.total} matching file(s) selected.`;
    selectAllMatchingButton.classList.add('hidden');
    clearSelectionButton.classList.remove('hidden');
    return;
  }

  const pageCheckboxes = rowsEl.querySelectorAll('.row-checkbox');
  const allPageSelected = pageCheckboxes.length > 0 && [...pageCheckboxes].every((cb) => state.selectedPaths.has(cb.dataset.path));

  if (state.selectedPaths.size === 0) {
    selectionBanner.classList.add('hidden');
    return;
  }

  selectionBanner.classList.remove('hidden');
  selectionBannerText.textContent = `${state.selectedPaths.size} selected.`;
  clearSelectionButton.classList.remove('hidden');

  if (allPageSelected && state.total > state.selectedPaths.size) {
    selectAllMatchingButton.textContent = `Select all ${state.total} matching`;
    selectAllMatchingButton.classList.remove('hidden');
  } else {
    selectAllMatchingButton.classList.add('hidden');
  }
}

function updatePaginationControls() {
  if (state.total === 0) {
    paginationInfo.textContent = 'No matching files.';
  } else {
    const start = state.pageState.offset + 1;
    const end = Math.min(state.pageState.offset + state.pageState.limit, state.total);
    paginationInfo.textContent = `${start}–${end} of ${state.total}`;
  }
  prevPageButton.disabled = state.pageState.offset === 0;
  nextPageButton.disabled = state.pageState.offset + state.pageState.limit >= state.total;
}

function updateSortIndicators() {
  document.querySelectorAll('[data-sort-indicator]').forEach((el) => {
    const col = el.dataset.sortIndicator;
    el.textContent = state.sortState.by === col ? (state.sortState.desc ? ' ▼' : ' ▲') : '';
  });
}

rowsEl.addEventListener('change', (e) => {
  if (!e.target.matches('.row-checkbox')) {
    return;
  }
  if (state.selectionMode === 'filter') {
    // The user is making an explicit choice — drop out of "all matching"
    // mode and seed explicit selection with every row currently checked
    // (which, in filter mode, was every row on this page).
    state.selectionMode = 'explicit';
    for (const checkbox of rowsEl.querySelectorAll('.row-checkbox')) {
      checkbox.disabled = false;
      if (checkbox.checked) {
        state.selectedPaths.add(checkbox.dataset.path);
      }
    }
  }
  const path = e.target.dataset.path;
  if (e.target.checked) {
    state.selectedPaths.add(path);
  } else {
    state.selectedPaths.delete(path);
  }
  selectAllCheckbox.checked = state.lastEntries.length > 0 && state.lastEntries.every((entry) => state.selectedPaths.has(entry.path));
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
  if (onRowOpen) {
    onRowOpen(row.dataset.path);
  }
});

selectAllCheckbox.addEventListener('change', () => {
  state.selectionMode = 'explicit';
  const checkboxes = rowsEl.querySelectorAll('.row-checkbox');
  for (const checkbox of checkboxes) {
    checkbox.disabled = false;
    checkbox.checked = selectAllCheckbox.checked;
    if (selectAllCheckbox.checked) {
      state.selectedPaths.add(checkbox.dataset.path);
    } else {
      state.selectedPaths.delete(checkbox.dataset.path);
    }
  }
  updateAllActionButtons();
  updateSelectionBanner();
});

selectAllMatchingButton.addEventListener('click', () => {
  state.selectionMode = 'filter';
  renderTable(state.lastEntries);
});

clearSelectionButton.addEventListener('click', () => {
  state.selectionMode = 'explicit';
  state.selectedPaths.clear();
  renderTable(state.lastEntries);
});

document.querySelector('thead').addEventListener('click', (e) => {
  const target = e.target.closest('[data-sort]');
  if (!target) {
    return;
  }
  const col = target.dataset.sort;
  if (state.sortState.by === col) {
    state.sortState.desc = !state.sortState.desc;
  } else {
    state.sortState.by = col;
    state.sortState.desc = false;
  }
  state.pageState.offset = 0;
  updateSortIndicators();
  loadLibrary();
});
updateSortIndicators();

prevPageButton.addEventListener('click', () => {
  state.pageState.offset = Math.max(0, state.pageState.offset - state.pageState.limit);
  loadLibrary();
});

nextPageButton.addEventListener('click', () => {
  state.pageState.offset += state.pageState.limit;
  loadLibrary();
});

export function initTable(onRowOpenCallback, renderCallback) {
  onRowOpen = onRowOpenCallback;
  render = renderCallback;
}
