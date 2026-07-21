<script setup>
import { store } from '../../store.js';
import { formatDuration } from '../../format.js';
import { statusLabel, statusClass, metadataText, hasRawMetadata, coverSrc } from '../../entryDisplay.js';
import { playTrack } from '../../composables/usePlayer.js';
import { openDetails } from '../../composables/useDetails.js';

function isChecked(entry) {
  return store.selectionMode === 'filter' || store.selectedPaths.has(entry.path);
}

function onCheckboxChange(entry, event) {
  if (store.selectionMode === 'filter') {
    store.selectionMode = 'explicit';
    for (const e of store.lastEntries) {
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
  <div>
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
      <div
        v-for="entry in store.lastEntries"
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

    <!--
      These pagination controls are intentionally NOT wired to anything —
      today's grid view has no working pagination (it silently relies on
      the table view's Prev/Next mutating the shared store.pageState.offset
      instead), and this change ports that bug as-is. See the separately
      proposed grid-view-pagination-fix change.
    -->
    <div class="flex items-center justify-between mt-3 text-sm text-neutral-400">
      <div></div>
      <div class="flex items-center gap-2">
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" disabled>Prev</button>
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" disabled>Next</button>
      </div>
    </div>
  </div>
</template>
