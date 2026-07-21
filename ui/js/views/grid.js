import { state } from '../state.js';
import { escapeHtml, STATUS_LABELS, STATUS_CLASSES, formatDuration } from '../format.js';
import { renderMetadataCell, updateSelectionBanner } from '../table.js';
import { updateAllActionButtons } from '../actions.js';
import { playTrack } from '../player.js';

const gridEl = document.getElementById('library-grid');

// Set by initGrid(); invoked when a card is clicked to open its details view.
let onCardOpen = null;

// Cover-forward rendering of the exact same GET /api/v1/library response
// table.js's loadLibrary already fetches — same filters/search/sort/
// selection/bulk-actions, just cards instead of rows.
export function renderGrid(entries) {
  gridEl.innerHTML = '';
  for (const entry of entries) {
    gridEl.appendChild(renderCard(entry));
  }
  updateAllActionButtons();
  updateSelectionBanner(gridEl);
}

function renderCard(entry) {
  const card = document.createElement('div');
  card.dataset.path = entry.path;
  card.className = 'card bg-neutral-900 border border-neutral-800 rounded-lg overflow-hidden cursor-pointer hover:border-neutral-600';

  const checked = (state.selectionMode === 'filter' || state.selectedPaths.has(entry.path)) ? 'checked' : '';
  const disabledAttr = state.selectionMode === 'filter' ? 'disabled' : '';
  const statusLabel = STATUS_LABELS[entry.status] || entry.status;
  const statusClass = STATUS_CLASSES[entry.status] || '';
  const cover = entry.has_cover_art
    ? `<img src="/api/v1/library/cover?path=${encodeURIComponent(entry.path)}" class="w-full aspect-square object-cover" alt="" />`
    : '<div class="w-full aspect-square bg-neutral-800"></div>';
  const playButton = entry.status === 'missing'
    ? ''
    : `<button class="play-button absolute bottom-2 right-2 w-7 h-7 rounded-full bg-black/60 text-white hover:bg-black/80" data-path="${escapeHtml(entry.path)}" title="Play">&#9654;</button>`;

  card.innerHTML = `
    <div class="relative">
      ${cover}
      <input type="checkbox" class="row-checkbox absolute top-2 left-2 w-4 h-4" data-path="${escapeHtml(entry.path)}" ${checked} ${disabledAttr} />
      ${playButton}
    </div>
    <div class="p-2 text-xs space-y-0.5">
      <div class="truncate">${renderMetadataCell(entry)}</div>
      <div class="text-neutral-500 flex justify-between">
        <span class="${statusClass}">${escapeHtml(statusLabel)}</span>
        <span>${formatDuration(entry.duration_seconds)}</span>
      </div>
    </div>
  `;
  return card;
}

gridEl.addEventListener('change', (e) => {
  if (!e.target.matches('.row-checkbox')) {
    return;
  }
  if (state.selectionMode === 'filter') {
    state.selectionMode = 'explicit';
    for (const checkbox of gridEl.querySelectorAll('.row-checkbox')) {
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
  updateAllActionButtons();
  updateSelectionBanner(gridEl);
});

gridEl.addEventListener('click', (e) => {
  if (e.target.closest('.play-button')) {
    e.stopPropagation();
    const path = e.target.closest('.play-button').dataset.path;
    const entry = state.lastEntries.find((en) => en.path === path);
    if (entry) {
      playTrack(entry);
    }
    return;
  }
  if (e.target.closest('input')) {
    return;
  }
  const card = e.target.closest('.card');
  if (!card) {
    return;
  }
  if (onCardOpen) {
    onCardOpen(card.dataset.path);
  }
});

export function initGrid(onCardOpenCallback) {
  onCardOpen = onCardOpenCallback;
}
