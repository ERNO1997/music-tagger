<script setup>
import { ref, computed, onMounted } from 'vue';
import { store, getSelectionCount } from './store.js';
import { libraryStatus } from './composables/useLibraryStatus.js';
import { loadLibrary } from './composables/useLibraryList.js';
import { useJobs } from './composables/useJobs.js';
import FilterBar from './components/FilterBar.vue';
import SelectionBanner from './components/SelectionBanner.vue';
import PlayerBar from './components/PlayerBar.vue';
import DetailsView from './components/DetailsView.vue';
import AllGroupingView from './components/views/AllGroupingView.vue';
import FolderGroupingView from './components/views/FolderGroupingView.vue';
import ArtistAlbumGroupingView from './components/views/ArtistAlbumGroupingView.vue';

const folderViewRef = ref(null);
const artistAlbumViewRef = ref(null);

const GROUPINGS = [
  { key: 'all', label: 'All' },
  { key: 'folder', label: 'Folder' },
  { key: 'artist-album', label: 'Artist / Album' },
];

const PRESENTATIONS = [
  { key: 'table', label: 'Table', icon: '☰' },
  { key: 'grid', label: 'Grid', icon: '⊞' },
];

// The presentation toggle only makes sense where the active grouping/level
// currently has a file/track listing to present: always for All and
// Folder; only at Artist-Album's `tracks` level (the artists/albums levels
// stay card grids of groups regardless of the stored presentation).
const presentationVisible = computed(() => {
  if (store.grouping === 'artist-album') {
    return artistAlbumViewRef.value?.level === 'tracks';
  }
  return true;
});

// Refreshes whichever grouping is presently active, keeping its current
// page/drill-down position — used after a bulk-action job updates the
// currently-visible listing (DetailsView's resolve/cover-choose, and the
// job poll onUpdate callbacks below).
async function refreshCurrentView() {
  switch (store.grouping) {
    case 'folder':
      return folderViewRef.value?.reloadTree();
    case 'artist-album':
      return artistAlbumViewRef.value?.reloadArtistAlbum();
    case 'all':
    default:
      return loadLibrary();
  }
}

// Refreshes whichever grouping is active after a filter/search/sort/page-size
// change, resetting back to its first page — a narrower filter can leave a
// previously-valid offset past the end of the new result set.
function refreshCurrentViewAfterFilterChange() {
  store.pageState.offset = 0;
  switch (store.grouping) {
    case 'folder':
      return folderViewRef.value?.resetTreePage();
    case 'artist-album':
      // Artists/albums/tracks levels aren't paginated — re-showing the
      // current level already reflects the new filter.
      return artistAlbumViewRef.value?.reloadArtistAlbum();
    case 'all':
    default:
      return loadLibrary();
  }
}

// Grouping and presentation are independent: switching one never resets or
// affects the other, and switching presentation never triggers a re-fetch —
// it's purely how the already-fetched entries are rendered.
async function selectGrouping(groupingKey) {
  if (groupingKey === store.grouping) {
    return;
  }
  store.grouping = groupingKey;
  switch (groupingKey) {
    case 'folder':
      await folderViewRef.value?.loadTree();
      break;
    case 'artist-album':
      await artistAlbumViewRef.value?.showArtists();
      break;
    case 'all':
    default:
      await loadLibrary();
      break;
  }
}

function selectPresentation(presentationKey) {
  store.presentation = presentationKey;
}

const jobs = useJobs({ refreshCurrentView });

const selectionCount = computed(() => getSelectionCount());

const identifyDisabled = computed(() => jobs.identify.running || selectionCount.value === 0);
const identifyLabel = computed(() => {
  if (!jobs.identify.running) return 'Identify Selected';
  return jobs.identify.total > 0 ? `Identifying ${jobs.identify.processed}/${jobs.identify.total}…` : 'Identifying…';
});

const enrichDisabled = computed(() => jobs.enrich.running || selectionCount.value === 0);
const enrichLabel = computed(() => {
  if (!jobs.enrich.running) return 'Enrich Selected';
  return jobs.enrich.total > 0 ? `Enriching ${jobs.enrich.processed}/${jobs.enrich.total}…` : 'Enriching…';
});

const tagDisabled = computed(() => jobs.tag.running || selectionCount.value === 0);
const tagLabel = computed(() => {
  if (!jobs.tag.running) return 'Tag Selected';
  return jobs.tag.total > 0 ? `Tagging ${jobs.tag.processed}/${jobs.tag.total}…` : 'Tagging…';
});

// Relocate and scan mutually exclude each other (a scan walking /music
// concurrently with a file being moved could see it as both missing at
// its old location and new at its new one) — the relocate action is
// disabled while a scan is running, and the refresh trigger is disabled
// while a relocate job is running, mirroring what the API itself rejects.
const relocateDisabled = computed(() => jobs.relocate.running || jobs.scan.running || selectionCount.value === 0);
const relocateLabel = computed(() => {
  if (!jobs.relocate.running) return 'Relocate Selected';
  return jobs.relocate.total > 0 ? `Relocating ${jobs.relocate.processed}/${jobs.relocate.total}…` : 'Relocating…';
});

const refreshDisabled = computed(() => jobs.scan.running || jobs.relocate.running);
const refreshLabel = computed(() => (jobs.scan.running ? 'Scanning…' : 'Refresh'));

const statusClass = computed(() => (libraryStatus.isError ? 'text-red-400 mb-4' : 'text-neutral-400 mb-4'));

onMounted(async () => {
  await loadLibrary();
  await jobs.initStatuses();
});
</script>

<template>
  <div class="max-w-6xl mx-auto p-6 pb-24">
    <div class="sticky top-0 z-10 bg-neutral-950 flex items-start justify-between gap-4 mb-1 py-3 border-b border-neutral-800">
      <h1 class="text-2xl font-semibold">Music Tagger</h1>
      <div class="flex gap-2 shrink-0">
        <button
          class="rounded-md bg-blue-600 text-white text-sm font-medium px-4 py-2 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="identifyDisabled"
          @click="jobs.triggerIdentify"
        >{{ identifyLabel }}</button>
        <button
          class="rounded-md bg-purple-600 text-white text-sm font-medium px-4 py-2 hover:bg-purple-500 disabled:opacity-50 disabled:cursor-not-allowed"
          title="Resolves cover art and lyrics for the selected files"
          :disabled="enrichDisabled"
          @click="jobs.triggerEnrich"
        >{{ enrichLabel }}</button>
        <button
          class="rounded-md bg-teal-600 text-white text-sm font-medium px-4 py-2 hover:bg-teal-500 disabled:opacity-50 disabled:cursor-not-allowed"
          title="Writes resolved metadata, cover art, and lyrics into the selected files' own tags"
          :disabled="tagDisabled"
          @click="jobs.triggerTag"
        >{{ tagLabel }}</button>
        <button
          class="rounded-md bg-orange-600 text-white text-sm font-medium px-4 py-2 hover:bg-orange-500 disabled:opacity-50 disabled:cursor-not-allowed"
          title="Moves the selected files into Artist/Album/Track - Title folders (requires tagging first)"
          :disabled="relocateDisabled"
          @click="jobs.triggerRelocate"
        >{{ relocateLabel }}</button>
        <button
          class="rounded-md bg-neutral-100 text-neutral-900 text-sm font-medium px-4 py-2 hover:bg-white disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="refreshDisabled"
          @click="jobs.triggerRefresh"
        >{{ refreshLabel }}</button>
      </div>
    </div>
    <p class="text-neutral-400 mb-4">Local library tracking — select rows to identify (AcoustID/MusicBrainz), enrich (cover art &amp; lyrics), tag (write it all into the file itself), or relocate (move into Artist/Album/Track folders). Files confirmed missing from disk can be removed from tracking.</p>

    <FilterBar @change="refreshCurrentViewAfterFilterChange" />

    <div class="flex flex-wrap items-center justify-between gap-3 mb-3">
      <div class="flex gap-1" role="tablist" aria-label="Grouping">
        <button
          v-for="g in GROUPINGS"
          :key="g.key"
          class="rounded-md px-3 py-1.5 text-sm font-medium"
          :class="store.grouping === g.key ? 'bg-neutral-100 text-neutral-900' : 'bg-neutral-900 text-neutral-300 border border-neutral-800'"
          @click="selectGrouping(g.key)"
        >{{ g.label }}</button>
      </div>
      <div v-if="presentationVisible" class="flex gap-1" role="tablist" aria-label="Presentation">
        <button
          v-for="p in PRESENTATIONS"
          :key="p.key"
          class="rounded-md px-3 py-1.5 text-sm font-medium"
          :class="store.presentation === p.key ? 'bg-neutral-100 text-neutral-900' : 'bg-neutral-900 text-neutral-300 border border-neutral-800'"
          :title="p.label"
          :aria-label="p.label"
          @click="selectPresentation(p.key)"
        >{{ p.icon }}</button>
      </div>
    </div>

    <SelectionBanner />

    <div :class="statusClass">{{ libraryStatus.text }}</div>

    <div v-show="store.grouping === 'all'">
      <AllGroupingView />
    </div>
    <div v-show="store.grouping === 'folder'">
      <FolderGroupingView ref="folderViewRef" />
    </div>
    <div v-show="store.grouping === 'artist-album'">
      <ArtistAlbumGroupingView ref="artistAlbumViewRef" />
    </div>
  </div>

  <PlayerBar />
  <DetailsView :refresh-current-view="refreshCurrentView" />
</template>
