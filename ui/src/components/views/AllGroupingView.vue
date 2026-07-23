<script setup>
import { computed } from 'vue';
import { store } from '../../store.js';
import { loadLibrary } from '../../composables/useLibraryList.js';
import EntryTable from '../EntryTable.vue';
import EntryGrid from '../EntryGrid.vue';

// The flat All grouping: today's whole-library fetch (useLibraryList.js),
// plus its own pagination controls, rendering whichever presentation is
// active — EntryTable/EntryGrid own no pagination of their own.
const paginationInfo = computed(() => {
  if (store.total === 0) {
    return 'No matching files.';
  }
  const start = store.pageState.offset + 1;
  const end = Math.min(store.pageState.offset + store.pageState.limit, store.total);
  return `${start}–${end} of ${store.total}`;
});
const prevDisabled = computed(() => store.pageState.offset === 0);
const nextDisabled = computed(() => store.pageState.offset + store.pageState.limit >= store.total);

function onPrevPage() {
  store.pageState.offset = Math.max(0, store.pageState.offset - store.pageState.limit);
  loadLibrary();
}

function onNextPage() {
  store.pageState.offset += store.pageState.limit;
  loadLibrary();
}

function onPageSizeChange(event) {
  store.pageState.limit = Number(event.target.value);
  store.pageState.offset = 0;
  loadLibrary();
}
</script>

<template>
  <div>
    <EntryTable v-if="store.presentation === 'table'" :entries="store.lastEntries" @sort="loadLibrary" @refresh="loadLibrary" />
    <EntryGrid v-else :entries="store.lastEntries" />

    <div class="flex items-center justify-between mt-3 text-sm text-neutral-400">
      <div>{{ paginationInfo }}</div>
      <div class="flex items-center gap-2">
        <select
          :value="store.pageState.limit"
          @change="onPageSizeChange"
          class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
        >
          <option value="25">25 / page</option>
          <option value="50">50 / page</option>
          <option value="100">100 / page</option>
          <option value="200">200 / page</option>
        </select>
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" :disabled="prevDisabled" @click="onPrevPage">Prev</button>
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" :disabled="nextDisabled" @click="onNextPage">Next</button>
      </div>
    </div>
  </div>
</template>
