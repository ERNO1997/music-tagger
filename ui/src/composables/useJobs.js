import { reactive } from 'vue';
import { store, getSelectionCount, getSelectionBody } from '../store.js';
import { formatEta, IDENTIFY_ETA_THRESHOLD } from '../format.js';
import {
  fetchScanStatus,
  fetchIdentifyStatus,
  fetchEnrichStatus,
  fetchTagStatus,
  fetchRelocateStatus,
  postScanTrigger,
  postIdentifyTrigger,
  postEnrichTrigger,
  postTagTrigger,
  postRelocateTrigger,
} from '../api.js';
import { pollJob } from '../polling.js';
import { libraryStatus } from './useLibraryStatus.js';

// Vue port of actions.js + main.js's five poll-loop wiring: one reactive
// job object per background job (scan/identify/enrich/tag/relocate), each
// tracking running/processed/total and exposing a trigger() call and a
// computed-ish label via a getter — App.vue binds these directly instead
// of the original's manual DOM/button mutation.
export function useJobs({ refreshCurrentView }) {
  const scan = reactive({ running: false, processed: 0, total: 0 });
  const identify = reactive({ running: false, processed: 0, total: 0 });
  const enrich = reactive({ running: false, processed: 0, total: 0 });
  const tag = reactive({ running: false, processed: 0, total: 0 });
  const relocate = reactive({ running: false, processed: 0, total: 0 });

  const scanPoll = pollJob({
    fetchStatus: fetchScanStatus,
    onUpdate: async (status) => {
      scan.running = status.running;
      scan.processed = status.processed;
      scan.total = status.total;
      if (status.running) {
        libraryStatus.text = status.total > 0
          ? `Scanning… ${status.processed}/${status.total} fingerprinted`
          : 'Scanning…';
        libraryStatus.isError = false;
      }
      await refreshCurrentView();
    },
  });
  const identifyPoll = pollJob({
    fetchStatus: fetchIdentifyStatus,
    onUpdate: async (status) => {
      identify.running = status.running;
      identify.processed = status.processed;
      identify.total = status.total;
      await refreshCurrentView();
    },
  });
  const enrichPoll = pollJob({
    fetchStatus: fetchEnrichStatus,
    onUpdate: async (status) => {
      enrich.running = status.running;
      enrich.processed = status.processed;
      enrich.total = status.total;
      await refreshCurrentView();
    },
  });
  const tagPoll = pollJob({
    fetchStatus: fetchTagStatus,
    onUpdate: async (status) => {
      tag.running = status.running;
      tag.processed = status.processed;
      tag.total = status.total;
      await refreshCurrentView();
    },
  });
  const relocatePoll = pollJob({
    fetchStatus: fetchRelocateStatus,
    onUpdate: async (status) => {
      relocate.running = status.running;
      relocate.processed = status.processed;
      relocate.total = status.total;
      reconcileRelocatedSelection(status.relocations);
      await refreshCurrentView();
    },
  });

  // A relocate job changes a file's tracked path server-side; without this,
  // a selected file would silently fall out of store.selectedPaths (it's
  // still keyed by the now-stale old path) once the view refreshes under
  // its new one. Safe to run on every poll tick: status.relocations is a
  // full accumulation since the job started (not just the delta since the
  // last poll), and once a path's swap is applied, its old path is no
  // longer selected, so re-applying the same list is a no-op.
  function reconcileRelocatedSelection(relocations) {
    if (!relocations) {
      return;
    }
    for (const { old_path: oldPath, new_path: newPath } of relocations) {
      if (store.selectedPaths.has(oldPath)) {
        store.selectedPaths.delete(oldPath);
        store.selectedPaths.add(newPath);
      }
    }
  }

  function reportError(message) {
    libraryStatus.text = message;
    libraryStatus.isError = true;
  }

  async function triggerRefresh() {
    try {
      await postScanTrigger();
      scan.running = true;
      scan.processed = 0;
      scan.total = 0;
      libraryStatus.text = 'Scanning…';
      libraryStatus.isError = false;
      scanPoll.start();
    } catch (err) {
      reportError(`Failed to start refresh: ${err.message}`);
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
      identify.running = true;
      identify.processed = 0;
      identify.total = count;
      identifyPoll.start();
    } catch (err) {
      reportError(`Failed to start identification: ${err.message}`);
    }
  }

  async function triggerEnrich() {
    const count = getSelectionCount();
    if (count === 0) {
      return;
    }
    try {
      await postEnrichTrigger(getSelectionBody());
      enrich.running = true;
      enrich.processed = 0;
      enrich.total = count;
      enrichPoll.start();
    } catch (err) {
      reportError(`Failed to start enrichment: ${err.message}`);
    }
  }

  async function triggerTag() {
    const count = getSelectionCount();
    if (count === 0) {
      return;
    }
    try {
      await postTagTrigger(getSelectionBody());
      tag.running = true;
      tag.processed = 0;
      tag.total = count;
      tagPoll.start();
    } catch (err) {
      reportError(`Failed to start tagging: ${err.message}`);
    }
  }

  async function triggerRelocate() {
    const count = getSelectionCount();
    if (count === 0) {
      return;
    }
    try {
      await postRelocateTrigger(getSelectionBody());
      relocate.running = true;
      relocate.processed = 0;
      relocate.total = count;
      relocatePoll.start();
    } catch (err) {
      reportError(`Failed to start relocation: ${err.message}`);
    }
  }

  // Mirrors main.js's init(): checks each job's status once at startup and
  // resumes polling if one was already running (e.g. started by another
  // tab, or before a page reload).
  async function initStatuses() {
    try {
      const status = await fetchScanStatus();
      scan.running = status.running;
      scan.processed = status.processed;
      scan.total = status.total;
      if (status.running) scanPoll.start();
    } catch (err) {
      // Status endpoint unreachable — leave the button enabled; the user can
      // still try to trigger a refresh manually.
    }
    try {
      const status = await fetchIdentifyStatus();
      identify.running = status.running;
      identify.processed = status.processed;
      identify.total = status.total;
      if (status.running) identifyPoll.start();
    } catch (err) {
      // Identify button stays disabled until a selection is made anyway.
    }
    try {
      const status = await fetchEnrichStatus();
      enrich.running = status.running;
      enrich.processed = status.processed;
      enrich.total = status.total;
      if (status.running) enrichPoll.start();
    } catch (err) {
      // Enrich button stays disabled until a selection is made anyway.
    }
    try {
      const status = await fetchTagStatus();
      tag.running = status.running;
      tag.processed = status.processed;
      tag.total = status.total;
      if (status.running) tagPoll.start();
    } catch (err) {
      // Tag button stays disabled until a selection is made anyway.
    }
    try {
      const status = await fetchRelocateStatus();
      relocate.running = status.running;
      relocate.processed = status.processed;
      relocate.total = status.total;
      if (status.running) relocatePoll.start();
    } catch (err) {
      // Relocate button stays disabled until a selection is made anyway.
    }
  }

  return {
    scan,
    identify,
    enrich,
    tag,
    relocate,
    triggerRefresh,
    triggerIdentify,
    triggerEnrich,
    triggerTag,
    triggerRelocate,
    initStatuses,
  };
}
