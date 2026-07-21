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
import { openDetails } from './details.js';
import {
  initActions,
  setScanningUI,
  setIdentifyingUI,
  setEnrichingUI,
  setTaggingUI,
  setRelocatingUI,
} from './actions.js';

const filterStatusSelect = document.getElementById('filter-status');
const filterTaggedSelect = document.getElementById('filter-tagged');
const filterRelocatedSelect = document.getElementById('filter-relocated');
const filterHasLyricsSelect = document.getElementById('filter-has-lyrics');
const filterHasCoverArtSelect = document.getElementById('filter-has-cover-art');
const filterSearchInput = document.getElementById('filter-search');
const pageSizeSelect = document.getElementById('page-size');

let searchDebounceTimer = null;

filterStatusSelect.addEventListener('change', () => {
  state.filterState.status = filterStatusSelect.value;
  state.pageState.offset = 0;
  loadLibrary();
});

filterTaggedSelect.addEventListener('change', () => {
  state.filterState.tagged = filterTaggedSelect.value;
  state.pageState.offset = 0;
  loadLibrary();
});

filterRelocatedSelect.addEventListener('change', () => {
  state.filterState.relocated = filterRelocatedSelect.value;
  state.pageState.offset = 0;
  loadLibrary();
});

filterHasLyricsSelect.addEventListener('change', () => {
  state.filterState.hasLyrics = filterHasLyricsSelect.value;
  state.pageState.offset = 0;
  loadLibrary();
});

filterHasCoverArtSelect.addEventListener('change', () => {
  state.filterState.hasCoverArt = filterHasCoverArtSelect.value;
  state.pageState.offset = 0;
  loadLibrary();
});

filterSearchInput.addEventListener('input', () => {
  clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    state.filterState.q = filterSearchInput.value.trim();
    state.pageState.offset = 0;
    loadLibrary();
  }, 300);
});

pageSizeSelect.addEventListener('change', () => {
  state.pageState.limit = Number(pageSizeSelect.value);
  state.pageState.offset = 0;
  loadLibrary();
});

// The single render-dispatch point for add-library-views-and-playback to
// extend with grid/tree/artist-album cases — today there's only one.
function renderCurrentView(entries) {
  switch (state.currentView) {
    case 'table':
    default:
      renderTable(entries);
      break;
  }
}

initTable(openDetails, renderCurrentView);

const scanPoll = pollJob({
  fetchStatus: fetchScanStatus,
  onUpdate: async (status) => {
    setScanningUI(status.running, status.processed, status.total);
    await loadLibrary();
  },
});
const identifyPoll = pollJob({
  fetchStatus: fetchIdentifyStatus,
  onUpdate: async (status) => {
    setIdentifyingUI(status.running, status.processed, status.total);
    await loadLibrary();
  },
});
const enrichPoll = pollJob({
  fetchStatus: fetchEnrichStatus,
  onUpdate: async (status) => {
    setEnrichingUI(status.running, status.processed, status.total);
    await loadLibrary();
  },
});
const tagPoll = pollJob({
  fetchStatus: fetchTagStatus,
  onUpdate: async (status) => {
    setTaggingUI(status.running, status.processed, status.total);
    await loadLibrary();
  },
});
const relocatePoll = pollJob({
  fetchStatus: fetchRelocateStatus,
  onUpdate: async (status) => {
    setRelocatingUI(status.running, status.processed, status.total);
    await loadLibrary();
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
