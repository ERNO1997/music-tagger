<script setup>
import { store } from '../store.js';

// Emits 'change' whenever a filter/sort-scoped control changes — App.vue
// listens and dispatches to whichever view is currently active
// (refreshCurrentViewAfterFilterChange), since the right refetch differs
// per view (table/grid reload, tree resets its page, artist-album re-shows
// its current level).
const emit = defineEmits(['change']);

let searchDebounceTimer = null;

function onSelectChange(field, event) {
  store.filterState[field] = event.target.value;
  emit('change');
}

function onSearchInput(event) {
  clearTimeout(searchDebounceTimer);
  const value = event.target.value;
  searchDebounceTimer = setTimeout(() => {
    store.filterState.q = value.trim();
    emit('change');
  }, 300);
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2 mb-3">
    <select
      :value="store.filterState.status"
      @change="onSelectChange('status', $event)"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
    >
      <option value="">All statuses</option>
      <option value="new">New</option>
      <option value="identified">Identified</option>
      <option value="not_found">Not Found</option>
      <option value="ambiguous">Ambiguous</option>
      <option value="missing">Missing</option>
    </select>
    <select
      :value="store.filterState.tagged"
      @change="onSelectChange('tagged', $event)"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
    >
      <option value="">Tagged: any</option>
      <option value="true">Tagged: yes</option>
      <option value="false">Tagged: no</option>
    </select>
    <select
      :value="store.filterState.relocated"
      @change="onSelectChange('relocated', $event)"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
    >
      <option value="">Relocated: any</option>
      <option value="true">Relocated: yes</option>
      <option value="false">Relocated: no</option>
    </select>
    <select
      :value="store.filterState.hasLyrics"
      @change="onSelectChange('hasLyrics', $event)"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
    >
      <option value="">Lyrics: any</option>
      <option value="true">Lyrics: yes</option>
      <option value="false">Lyrics: no</option>
    </select>
    <select
      :value="store.filterState.hasCoverArt"
      @change="onSelectChange('hasCoverArt', $event)"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-2 py-1.5"
    >
      <option value="">Cover: any</option>
      <option value="true">Cover: yes</option>
      <option value="false">Cover: no</option>
    </select>
    <input
      type="search"
      :value="store.filterState.q"
      @input="onSearchInput"
      placeholder="Search path / artist / album / title…"
      class="rounded-md bg-neutral-900 border border-neutral-800 text-sm px-3 py-1.5 flex-1 min-w-[200px]"
    />
  </div>
</template>
