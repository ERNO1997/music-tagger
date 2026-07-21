import { state, buildFilterParams } from '../state.js';
import { formatDuration, escapeHtml, STATUS_LABELS, STATUS_CLASSES } from '../format.js';
import { fetchTree } from '../api.js';
import { renderCoverCell, renderMetadataCell } from '../table.js';
import { playTrack } from '../player.js';

const breadcrumbEl = document.getElementById('tree-breadcrumb');
const directoriesEl = document.getElementById('tree-directories');
const filesEl = document.getElementById('tree-files');
const paginationInfo = document.getElementById('tree-pagination-info');
const prevPageButton = document.getElementById('tree-prev-page');
const nextPageButton = document.getElementById('tree-next-page');

// Set by initTree(); invoked when a file row is clicked to open its details view.
let onFileOpen = null;

// rootPath is captured from the very first response (whatever prefix the
// server resolved an omitted "path" to, i.e. the music root) — used only to
// render a shorter "Home" breadcrumb segment. currentPath is the prefix
// currently being browsed; offset paginates its direct-files list.
let rootPath = null;
let currentPath = null;
let offset = 0;
let currentFiles = [];

// Navigates to path (undefined/null means "wherever we last were, or the
// music root on first load") and re-fetches. Exported so main.js can call
// it when the user switches to this view, and details.js can call it after
// an action that should refresh the currently-visible listing.
export async function loadTree(path) {
  if (path !== undefined && path !== currentPath) {
    currentPath = path;
    offset = 0;
  }
  await fetchAndRender();
}

async function fetchAndRender() {
  const params = buildFilterParams();
  if (currentPath) {
    params.set('path', currentPath);
  }
  params.set('limit', String(state.pageState.limit));
  params.set('offset', String(offset));

  try {
    const data = await fetchTree(params);
    currentPath = data.path;
    if (rootPath === null) {
      rootPath = data.path;
    }
    renderBreadcrumb();
    renderDirectories(data.directories || []);
    renderFiles(data.files?.entries || [], data.files?.total || 0);
  } catch (err) {
    filesEl.innerHTML = `<tr><td class="px-4 py-3 text-red-400" colspan="7">Failed to load folder: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function renderBreadcrumb() {
  breadcrumbEl.innerHTML = '';
  if (rootPath === null || currentPath === null) {
    return;
  }
  const rest = currentPath === rootPath ? '' : currentPath.slice(rootPath.length).replace(/^\/+/, '');
  const segments = rest ? rest.split('/') : [];

  const crumbs = [{ label: 'Home', path: rootPath }];
  let acc = rootPath;
  for (const segment of segments) {
    acc = `${acc}/${segment}`;
    crumbs.push({ label: segment, path: acc });
  }

  crumbs.forEach((crumb, i) => {
    if (i > 0) {
      breadcrumbEl.appendChild(document.createTextNode(' / '));
    }
    const link = document.createElement('button');
    link.textContent = crumb.label;
    link.className = i === crumbs.length - 1 ? 'text-neutral-200 font-medium' : 'text-blue-400 hover:underline';
    link.addEventListener('click', () => loadTree(crumb.path));
    breadcrumbEl.appendChild(link);
  });
}

function renderDirectories(directories) {
  directoriesEl.innerHTML = '';
  for (const dir of directories) {
    const card = document.createElement('button');
    card.className = 'text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600';
    card.innerHTML = `
      <div class="text-sm truncate">&#128193; ${escapeHtml(dir.name)}</div>
      <div class="text-xs text-neutral-500">${dir.identified_count}/${dir.total_count} identified</div>
    `;
    card.addEventListener('click', () => loadTree(`${currentPath}/${dir.name}`));
    directoriesEl.appendChild(card);
  }
}

function renderFiles(entries, total) {
  currentFiles = entries;
  // openDetails() (details.js) looks entries up via state.lastEntries
  // regardless of which view is active — table/grid repopulate it via
  // loadLibrary() whenever they become active again, so overwriting it here
  // is safe.
  state.lastEntries = entries;
  filesEl.innerHTML = '';
  for (const entry of entries) {
    filesEl.appendChild(renderFileRow(entry));
  }

  if (total === 0) {
    paginationInfo.textContent = entries.length === 0 && directoriesEl.children.length === 0
      ? 'No tracked files under this folder.'
      : 'No files directly in this folder.';
  } else {
    const start = offset + 1;
    const end = Math.min(offset + state.pageState.limit, total);
    paginationInfo.textContent = `${start}–${end} of ${total} file(s) in this folder`;
  }
  prevPageButton.disabled = offset === 0;
  nextPageButton.disabled = offset + state.pageState.limit >= total;
}

function renderFileRow(entry) {
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

filesEl.addEventListener('click', (e) => {
  if (e.target.closest('.play-button')) {
    e.stopPropagation();
    const path = e.target.closest('.play-button').dataset.path;
    const entry = currentFiles.find((en) => en.path === path);
    if (entry) {
      playTrack(entry);
    }
    return;
  }
  const row = e.target.closest('tr');
  if (!row || !row.dataset.path) {
    return;
  }
  if (onFileOpen) {
    onFileOpen(row.dataset.path);
  }
});

prevPageButton.addEventListener('click', () => {
  offset = Math.max(0, offset - state.pageState.limit);
  fetchAndRender();
});

nextPageButton.addEventListener('click', () => {
  offset += state.pageState.limit;
  fetchAndRender();
});

// Re-fetches whatever folder is currently displayed, keeping the current
// page position — for main.js's refreshCurrentView after a bulk-action job
// updates the currently-visible listing.
export function reloadTree() {
  return fetchAndRender();
}

// Re-fetches whatever folder is currently displayed, resetting back to its
// first page — for main.js's filter/search change handlers, since a
// narrower filter can leave a previously-valid offset past the end.
export function resetTreePage() {
  offset = 0;
  return fetchAndRender();
}

export function initTree(onFileOpenCallback) {
  onFileOpen = onFileOpenCallback;
}
