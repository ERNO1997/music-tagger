import { getSelectionCount, getSelectionBody } from './state.js';
import { formatEta, IDENTIFY_ETA_THRESHOLD } from './format.js';
import {
  postScanTrigger,
  postIdentifyTrigger,
  postEnrichTrigger,
  postTagTrigger,
  postRelocateTrigger,
} from './api.js';

const statusEl = document.getElementById('status');
const refreshButton = document.getElementById('refresh-button');
const identifyButton = document.getElementById('identify-button');
const enrichButton = document.getElementById('enrich-button');
const tagButton = document.getElementById('tag-button');
const relocateButton = document.getElementById('relocate-button');

let identifyRunning = false;
let enrichRunning = false;
let tagRunning = false;
let relocateRunning = false;
let scanRunning = false;

// Set by initActions(); each starts the poll loop for the matching job.
let startScanPolling = () => {};
let startIdentifyPolling = () => {};
let startEnrichPolling = () => {};
let startTagPolling = () => {};
let startRelocatePolling = () => {};

export function updateAllActionButtons() {
  updateIdentifyButton();
  updateEnrichButton();
  updateTagButton();
  updateRelocateButton();
}

export function updateIdentifyButton() {
  identifyButton.disabled = identifyRunning || getSelectionCount() === 0;
  if (!identifyRunning) {
    identifyButton.textContent = 'Identify Selected';
  }
}

export function updateEnrichButton() {
  enrichButton.disabled = enrichRunning || getSelectionCount() === 0;
  if (!enrichRunning) {
    enrichButton.textContent = 'Enrich Selected';
  }
}

export function updateTagButton() {
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
export function updateRelocateButton() {
  relocateButton.disabled = relocateRunning || scanRunning || getSelectionCount() === 0;
  if (!relocateRunning) {
    relocateButton.textContent = 'Relocate Selected';
  }
}

export function updateRefreshButton() {
  refreshButton.disabled = scanRunning || relocateRunning;
  if (!scanRunning) {
    refreshButton.textContent = 'Refresh';
  }
}

export function setScanningUI(running, processed, total) {
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

export function setIdentifyingUI(running, processed, total) {
  identifyRunning = running;
  updateIdentifyButton();
  if (running) {
    identifyButton.textContent = total > 0 ? `Identifying ${processed}/${total}…` : 'Identifying…';
  }
}

export function setEnrichingUI(running, processed, total) {
  enrichRunning = running;
  updateEnrichButton();
  if (running) {
    enrichButton.textContent = total > 0 ? `Enriching ${processed}/${total}…` : 'Enriching…';
  }
}

export function setTaggingUI(running, processed, total) {
  tagRunning = running;
  updateTagButton();
  if (running) {
    tagButton.textContent = total > 0 ? `Tagging ${processed}/${total}…` : 'Tagging…';
  }
}

export function setRelocatingUI(running, processed, total) {
  relocateRunning = running;
  updateRelocateButton();
  updateRefreshButton();
  if (running) {
    relocateButton.textContent = total > 0 ? `Relocating ${processed}/${total}…` : 'Relocating…';
  }
}

async function triggerRefresh() {
  try {
    await postScanTrigger();
    // Either we started it, or one was already running — either way, a
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
    await postIdentifyTrigger(getSelectionBody());
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
    await postEnrichTrigger(getSelectionBody());
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
    await postTagTrigger(getSelectionBody());
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
    await postRelocateTrigger(getSelectionBody());
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

export function initActions(pollers) {
  startScanPolling = pollers.startScanPolling;
  startIdentifyPolling = pollers.startIdentifyPolling;
  startEnrichPolling = pollers.startEnrichPolling;
  startTagPolling = pollers.startTagPolling;
  startRelocatePolling = pollers.startRelocatePolling;
}
