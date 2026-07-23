<script setup>
import { computed } from 'vue';
import { store } from '../store.js';
import { formatDuration } from '../format.js';
import { statusLabel, statusClass, metadataText, hasRawMetadata, coverSrc, displayPath, missingMetadataFields } from '../entryDisplay.js';
import { deleteLibraryEntry } from '../api.js';
import { playTrack } from '../composables/usePlayer.js';
import { openDetails } from '../composables/useDetails.js';
import { libraryStatus } from '../composables/useLibraryStatus.js';

// Shared row rendering + selection, used by every grouping (All, Folder,
// Artist-Album's tracks level). Takes its rows via `entries` rather than
// reading store.lastEntries directly, and never fetches itself — sorting
// and post-delete reloads are left to the parent grouping view via emits,
// since which fetch to re-run differs per grouping.
//
// `sortable` is only true for the All grouping: the Folder/Artist-Album
// endpoints don't accept sort/order params, so their headers render as
// today's TreeView/ArtistAlbumView did — plain, non-clickable.
const props = defineProps({
  entries: { type: Array, required: true },
  sortable: { type: Boolean, default: true },
});

const emit = defineEmits(['sort', 'refresh']);

const selectAllChecked = computed(
  () => store.selectionMode === 'filter' || (props.entries.length > 0 && props.entries.every((e) => store.selectedPaths.has(e.path))),
);
const selectAllDisabled = computed(() => store.selectionMode === 'filter');

function isRowChecked(entry) {
  return store.selectionMode === 'filter' || store.selectedPaths.has(entry.path);
}

function onRowCheckboxChange(entry, event) {
  if (store.selectionMode === 'filter') {
    // The user is making an explicit choice — drop out of "all matching"
    // mode and seed explicit selection with every row currently checked
    // (which, in filter mode, was every row on this page).
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

function onSelectAllChange(event) {
  store.selectionMode = 'explicit';
  for (const entry of props.entries) {
    if (event.target.checked) {
      store.selectedPaths.add(entry.path);
    } else {
      store.selectedPaths.delete(entry.path);
    }
  }
}

function onSort(column) {
  if (!props.sortable) {
    return;
  }
  if (store.sortState.by === column) {
    store.sortState.desc = !store.sortState.desc;
  } else {
    store.sortState.by = column;
    store.sortState.desc = false;
  }
  store.pageState.offset = 0;
  emit('sort', column);
}

function sortIndicator(column) {
  if (!props.sortable || store.sortState.by !== column) {
    return '';
  }
  return store.sortState.desc ? ' ▼' : ' ▲';
}

function onPlay(entry) {
  playTrack(entry);
}

async function onDelete(entry) {
  if (!confirm(`Delete the tracked entry for:\n${entry.path}\n\nThis only removes it from tracking — it does not affect any file on disk (the file is already missing).`)) {
    return;
  }
  try {
    await deleteLibraryEntry(entry.path);
    store.selectedPaths.delete(entry.path);
    emit('refresh');
  } catch (err) {
    libraryStatus.text = `Failed to delete entry: ${err.message}`;
    libraryStatus.isError = true;
  }
}

function onRowClick(entry) {
  openDetails(entry.path);
}
</script>

<template>
  <div class="overflow-x-auto rounded-lg border border-neutral-800">
    <table class="w-full text-sm text-left">
      <thead class="bg-neutral-900 text-neutral-400 uppercase text-xs">
        <tr>
          <th class="px-4 py-3">
            <input type="checkbox" :checked="selectAllChecked" :disabled="selectAllDisabled" @change="onSelectAllChange" />
          </th>
          <th class="px-4 py-3">Cover</th>
          <th v-if="sortable" class="px-4 py-3 cursor-pointer select-none" @click="onSort('path')">Path<span>{{ sortIndicator('path') }}</span></th>
          <th v-else class="px-4 py-3">Path</th>
          <th class="px-4 py-3">Format</th>
          <th v-if="sortable" class="px-4 py-3 cursor-pointer select-none" @click="onSort('duration')">Duration<span>{{ sortIndicator('duration') }}</span></th>
          <th v-else class="px-4 py-3">Duration</th>
          <th v-if="sortable" class="px-4 py-3 cursor-pointer select-none" @click="onSort('status')">Status<span>{{ sortIndicator('status') }}</span></th>
          <th v-else class="px-4 py-3">Status</th>
          <th v-if="sortable" class="px-4 py-3">
            <span class="cursor-pointer select-none" @click="onSort('artist')">Artist<span>{{ sortIndicator('artist') }}</span></span>
            / <span class="cursor-pointer select-none" @click="onSort('album')">Album<span>{{ sortIndicator('album') }}</span></span>
            / Title / Track
          </th>
          <th v-else class="px-4 py-3">Artist / Album / Title / Track</th>
          <th class="px-4 py-3">Lyrics</th>
          <th class="px-4 py-3">Actions</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-neutral-800">
        <tr v-if="entries.length === 0">
          <td colspan="9" class="px-4 py-8 text-center text-neutral-500">No items match the current filters.</td>
        </tr>
        <tr
          v-for="entry in entries"
          :key="entry.path"
          class="cursor-pointer hover:bg-neutral-900"
          :class="{ 'text-red-400': entry.error }"
          @click="onRowClick(entry)"
        >
          <td class="px-4 py-3" @click.stop>
            <input type="checkbox" class="row-checkbox" :checked="isRowChecked(entry)" :disabled="store.selectionMode === 'filter'" @change="onRowCheckboxChange(entry, $event)" />
          </td>
          <td class="px-4 py-3">
            <img v-if="coverSrc(entry)" :src="coverSrc(entry)" class="w-10 h-10 rounded object-cover" alt="" />
            <div v-else class="w-10 h-10 rounded bg-neutral-800"></div>
          </td>
          <td class="px-4 py-3 font-mono text-xs" :title="entry.path">
            <span v-if="entry.relocated" class="text-green-400 mr-1" title="At its canonical relocated path">&#10003;</span>
            <span v-else-if="entry.relocate_error" class="mr-1" :title="`Relocation failed: ${entry.relocate_error}`">&#9888;</span>
            {{ displayPath(entry.path) }}
          </td>
          <td class="px-4 py-3 uppercase">{{ entry.format }}</td>
          <td class="px-4 py-3">
            <template v-if="entry.error">—</template>
            <template v-else>{{ formatDuration(entry.duration_seconds) }}</template>
          </td>
          <td class="px-4 py-3" :class="!entry.error && statusClass(entry)">
            <template v-if="entry.error">Error: {{ entry.error }}</template>
            <template v-else>{{ statusLabel(entry) }}</template>
          </td>
          <td class="px-4 py-3">
            <template v-if="entry.status === 'identified'">
              <span v-if="missingMetadataFields(entry).length === 0" class="text-green-400 mr-1" title="Artist, album, title, and track number are all present">&#10003;</span>
              <span v-else class="text-yellow-400 mr-1" :title="`Missing: ${missingMetadataFields(entry).join(', ')}`">&#9888;</span>
              <span>{{ metadataText(entry) }}</span>
            </template>
            <span v-else-if="hasRawMetadata(entry)" class="italic text-neutral-500" title="From the file's own tags, not yet identified">{{ metadataText(entry) }}</span>
            <span v-else>—</span>
          </td>
          <td class="px-4 py-3">
            <template v-if="entry.error">—</template>
            <span v-else-if="entry.has_lyrics" class="text-green-400" title="Lyrics available">&#9834;</span>
            <template v-else>—</template>
          </td>
          <td class="px-4 py-3" @click.stop>
            <button v-if="entry.status !== 'missing'" class="play-button text-neutral-300 hover:text-white mr-2" title="Play" @click="onPlay(entry)">&#9654;</button>
            <button v-else class="delete-entry-button text-red-400 hover:text-red-300" title="Delete this tracked entry (the file is already missing from disk)" @click="onDelete(entry)">&#128465;</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
