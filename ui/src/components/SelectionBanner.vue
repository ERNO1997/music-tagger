<script setup>
import { computed } from 'vue';
import { store } from '../store.js';

// Distinguishes three states: nothing selected (hidden), an explicit set
// of paths selected (possibly spanning past pages), and "every file
// matching the current filter" selected — the latter always reads its
// count from store.total, which is re-fetched with every filter change, so
// the displayed count and what a bulk action would actually process never
// drift apart.
const showBanner = computed(() => store.selectionMode === 'filter' || store.selectedPaths.size > 0);

const bannerText = computed(() => {
  if (store.selectionMode === 'filter') {
    return `All ${store.total} matching file(s) selected.`;
  }
  return `${store.selectedPaths.size} selected.`;
});

// "all currently-visible-page entries are selected" — based on whichever
// view most recently populated store.lastEntries (table/grid repopulate it
// on every load).
const allPageSelected = computed(() => {
  const paths = store.lastEntries.map((e) => e.path);
  return paths.length > 0 && paths.every((p) => store.selectedPaths.has(p));
});

const showSelectAllMatching = computed(
  () => store.selectionMode !== 'filter' && allPageSelected.value && store.total > store.selectedPaths.size,
);

function selectAllMatching() {
  store.selectionMode = 'filter';
}

function clearSelection() {
  store.selectionMode = 'explicit';
  store.selectedPaths.clear();
}
</script>

<template>
  <div
    v-if="showBanner"
    id="selection-banner"
    class="text-sm text-neutral-300 mb-3 bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 flex items-center justify-between gap-4"
  >
    <span>{{ bannerText }}</span>
    <div class="flex gap-3 shrink-0">
      <button v-if="showSelectAllMatching" @click="selectAllMatching" class="text-blue-400 hover:underline">
        Select all {{ store.total }} matching
      </button>
      <button @click="clearSelection" class="text-neutral-400 hover:underline">Clear selection</button>
    </div>
  </div>
</template>
