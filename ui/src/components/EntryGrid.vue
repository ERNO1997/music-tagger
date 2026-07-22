<script setup>
import { store } from '../store.js';
import { formatDuration } from '../format.js';
import { statusLabel, statusClass, metadataText, hasRawMetadata, coverSrc } from '../entryDisplay.js';
import { playTrack } from '../composables/usePlayer.js';
import { openDetails } from '../composables/useDetails.js';

// Shared card rendering + selection, used by every grouping (All, Folder,
// Artist-Album's tracks level). Takes its cards via `entries` rather than
// reading store.lastEntries directly, and never fetches itself.
const props = defineProps({
  entries: { type: Array, required: true },
});

function isChecked(entry) {
  return store.selectionMode === 'filter' || store.selectedPaths.has(entry.path);
}

function onCheckboxChange(entry, event) {
  if (store.selectionMode === 'filter') {
    store.selectionMode = 'explicit';
    for (const e of props.entries) {
      store.selectedPaths.add(e.path);
    }
  }
  if (event.target.checked) {
    store.selectedPaths.add(entry.path);
  } else {
    store.selectedPaths.delete(entry.path);
  }
}

function onPlay(entry) {
  playTrack(entry);
}

function onCardClick(entry) {
  openDetails(entry.path);
}
</script>

<template>
  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
    <div
      v-for="entry in entries"
      :key="entry.path"
      class="card bg-neutral-900 border border-neutral-800 rounded-lg overflow-hidden cursor-pointer hover:border-neutral-600"
      @click="onCardClick(entry)"
    >
      <div class="relative">
        <img v-if="coverSrc(entry)" :src="coverSrc(entry)" class="w-full aspect-square object-cover" alt="" />
        <div v-else class="w-full aspect-square bg-neutral-800"></div>
        <input
          type="checkbox"
          class="row-checkbox absolute top-2 left-2 w-4 h-4"
          :checked="isChecked(entry)"
          :disabled="store.selectionMode === 'filter'"
          @click.stop
          @change="onCheckboxChange(entry, $event)"
        />
        <button
          v-if="entry.status !== 'missing'"
          class="play-button absolute bottom-2 right-2 w-7 h-7 rounded-full bg-black/60 text-white hover:bg-black/80"
          title="Play"
          @click.stop="onPlay(entry)"
        >&#9654;</button>
      </div>
      <div class="p-2 text-xs space-y-0.5">
        <div class="truncate">
          <span v-if="entry.status === 'identified'">{{ metadataText(entry) }}</span>
          <span v-else-if="hasRawMetadata(entry)" class="italic text-neutral-500" title="From the file's own tags, not yet identified">{{ metadataText(entry) }}</span>
          <span v-else>—</span>
        </div>
        <div class="text-neutral-500 flex justify-between">
          <span :class="statusClass(entry)">{{ statusLabel(entry) }}</span>
          <span>{{ formatDuration(entry.duration_seconds) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
