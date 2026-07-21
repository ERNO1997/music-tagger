import { state } from './state.js';
import {
  fetchScanStatus,
  fetchIdentifyStatus,
  fetchEnrichStatus,
  fetchTagStatus,
  fetchRelocateStatus,
} from './api.js';
import { pollJob } from './polling.js';
import { loadLibrary, initTable, renderTable } from './table.js';
import { openDetails, initDetails } from './details.js';
import {
  initActions,
  setScanningUI,
  setIdentifyingUI,
  setEnrichingUI,
  setTaggingUI,
  setRelocatingUI,
} from './actions.js';
import { renderGrid, initGrid } from './views/grid.js';
import { loadTree, resetTreePage, reloadTree, initTree } from './views/tree.js';
import { showArtists, reloadArtistAlbum, initArtistAlbum } from './views/artist-album.js';

const filterStatusSelect = document.getElementById('filter-status');
const filterTaggedSelect = document.getElementById('filter-tagged');
const filterRelocatedSelect = document.getElementById('filter-relocated');
const filterHasLyricsSelect = document.getElementById('filter-has-lyrics');
const filterHasCoverArtSelect = document.getElementById('filter-has-cover-art');
const filterSearchInput = document.getElementById('filter-search');
const pageSizeSelect = document.getElementById('page-size');

const viewTabs = document.querySelectorAll('.view-tab');
const viewContainers = {
  table: document.getElementById('view-table'),
  grid: document.getElementById('view-grid'),
  tree: document.getElementById('view-tree'),
  'artist-album': document.getElementById('view-artist-album'),
};

let searchDebounceTimer = null;

// The single render-dispatch point for table/grid — the only two views that
// render an already-fetched flat entries array. Tree and Artist-Album fetch
// and render through their own grouped-query endpoints instead (see
// refreshCurrentView below), since their data isn't a flat list.
function renderCurrentView(entries) {
  switch (state.currentView) {
    case 'grid':
      renderGrid(entries);
      break;
    case 'table':
    default:
      renderTable(entries);
      break;
  }
}

// Refreshes whichever of the four views is presently active, keeping its
// current page/drill-down position — used after a bulk-action job updates
// the currently-visible listing (details.js's resolve/cover-choose, and
// this file's own polling onUpdate callbacks below).
function refreshCurrentView() {
  switch (state.currentView) {
    case 'tree':
      return reloadTree();
    case 'artist-album':
      return reloadArtistAlbum();
    case 'grid':
    case 'table':
    default:
      return loadLibrary();
  }
}

// Refreshes whichever view is active after a filter/search/sort/page-size
// change, resetting back to its first page — a narrower filter can leave a
// previously-valid offset past the end of the new result set.
function refreshCurrentViewAfterFilterChange() {
  state.pageState.offset = 0;
  switch (state.currentView) {
    case 'tree':
      return resetTreePage();
    case 'artist-album':
      // Artists/albums/tracks levels aren't paginated — re-showing the
      // current level already reflects the new filter.
      return reloadArtistAlbum();
    case 'grid':
    case 'table':
    default:
      return loadLibrary();
  }
}

function setActiveView(view) {
  state.currentView = view;
  for (const [key, el] of Object.entries(viewContainers)) {
    el.classList.toggle('hidden', key !== view);
  }
  for (const tab of viewTabs) {
    const active = tab.dataset.view === view;
    tab.classList.toggle('bg-neutral-100', active);
    tab.classList.toggle('text-neutral-900', active);
    tab.classList.toggle('bg-neutral-900', !active);
    tab.classList.toggle('text-neutral-300', !active);
    tab.classList.toggle('border', !active);
    tab.classList.toggle('border-neutral-800', !active);
  }
}

viewTabs.forEach((tab) => {
  tab.addEventListener('click', () => {
    if (tab.dataset.view === state.currentView) {
      return;
    }
    setActiveView(tab.dataset.view);
    switch (tab.dataset.view) {
      case 'tree':
        loadTree();
        break;
      case 'artist-album':
        showArtists();
        break;
      case 'grid':
      case 'table':
      default:
        loadLibrary();
        break;
    }
  });
});

filterStatusSelect.addEventListener('change', () => {
  state.filterState.status = filterStatusSelect.value;
  refreshCurrentViewAfterFilterChange();
});

filterTaggedSelect.addEventListener('change', () => {
  state.filterState.tagged = filterTaggedSelect.value;
  refreshCurrentViewAfterFilterChange();
});

filterRelocatedSelect.addEventListener('change', () => {
  state.filterState.relocated = filterRelocatedSelect.value;
  refreshCurrentViewAfterFilterChange();
});

filterHasLyricsSelect.addEventListener('change', () => {
  state.filterState.hasLyrics = filterHasLyricsSelect.value;
  refreshCurrentViewAfterFilterChange();
});

filterHasCoverArtSelect.addEventListener('change', () => {
  state.filterState.hasCoverArt = filterHasCoverArtSelect.value;
  refreshCurrentViewAfterFilterChange();
});

filterSearchInput.addEventListener('input', () => {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    state.filterState.q = filterSearchInput.value.trim();
    refreshCurrentViewAfterFilterChange();
  }, 300);
});

pageSizeSelect.addEventListener('change', () => {
  state.pageState.limit = Number(pageSizeSelect.value);
  refreshCurrentViewAfterFilterChange();
});

initTable(openDetails, renderCurrentView);
initGrid(openDetails);
initTree(openDetails);
initArtistAlbum(openDetails);
initDetails(refreshCurrentView);

const scanPoll = pollJob({
  fetchStatus: fetchScanStatus,
  onUpdate: async (status) => {
    setScanningUI(status.running, status.processed, status.total);
    await refreshCurrentView();
  },
});
const identifyPoll = pollJob({
  fetchStatus: fetchIdentifyStatus,
  onUpdate: async (status) => {
    setIdentifyingUI(status.running, status.processed, status.total);
    await refreshCurrentView();
  },
});
const enrichPoll = pollJob({
  fetchStatus: fetchEnrichStatus,
  onUpdate: async (status) => {
    setEnrichingUI(status.running, status.processed, status.total);
    await refreshCurrentView();
  },
});
const tagPoll = pollJob({
  fetchStatus: fetchTagStatus,
  onUpdate: async (status) => {
    setTaggingUI(status.running, status.processed, status.total);
    await refreshCurrentView();
  },
});
const relocatePoll = pollJob({
  fetchStatus: fetchRelocateStatus,
  onUpdate: async (status) => {
    setRelocatingUI(status.running, status.processed, status.total);
    await refreshCurrentView();
  },
});

initActions({
  startScanPolling: scanPoll.start,
  startIdentifyPolling: identifyPoll.start,
  startEnrichPolling: enrichPoll.start,
  startTagPolling: tagPoll.start,
  startRelocatePolling: relocatePoll.start,
});

(async function init() {
  await loadLibrary();
  try {
    const scanStatus = await fetchScanStatus();
    setScanningUI(scanStatus.running, scanStatus.processed, scanStatus.total);
    if (scanStatus.running) {
      scanPoll.start();
    }
  } catch (err) {
    // Status endpoint unreachable — leave the button enabled; the user can
    // still try to trigger a refresh manually.
  }
  try {
    const identifyStatus = await fetchIdentifyStatus();
    setIdentifyingUI(identifyStatus.running, identifyStatus.processed, identifyStatus.total);
    if (identifyStatus.running) {
      identifyPoll.start();
    }
  } catch (err) {
    // Status endpoint unreachable — identify button stays disabled until a
    // selection is made anyway.
  }
  try {
    const enrichStatus = await fetchEnrichStatus();
    setEnrichingUI(enrichStatus.running, enrichStatus.processed, enrichStatus.total);
    if (enrichStatus.running) {
      enrichPoll.start();
    }
  } catch (err) {
    // Status endpoint unreachable — enrich button stays disabled until a
    // selection is made anyway.
  }
  try {
    const tagStatus = await fetchTagStatus();
    setTaggingUI(tagStatus.running, tagStatus.processed, tagStatus.total);
    if (tagStatus.running) {
      tagPoll.start();
    }
  } catch (err) {
    // Status endpoint unreachable — tag button stays disabled until a
    // selection is made anyway.
  }
  try {
    const relocateStatus = await fetchRelocateStatus();
    setRelocatingUI(relocateStatus.running, relocateStatus.processed, relocateStatus.total);
    if (relocateStatus.running) {
      relocatePoll.start();
    }
  } catch (err) {
    // Status endpoint unreachable — relocate button stays disabled until a
    // selection is made anyway.
  }
})();
