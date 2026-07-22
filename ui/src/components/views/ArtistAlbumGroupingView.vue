<script setup>
import { ref } from 'vue';
import { store, buildFilterParams } from '../../store.js';
import { fetchArtists, fetchAlbums, fetchTracks } from '../../api.js';
import EntryTable from '../EntryTable.vue';
import EntryGrid from '../EntryGrid.vue';

// level is one of 'artists' | 'albums' | 'tracks' — the current drill-down
// depth. selectedArtist/selectedAlbum are set as the user drills in.
// Exposed (auto-unwrapped by defineExpose) so App.vue can tell whether a
// file listing — and therefore the presentation toggle — is showing.
const level = ref('artists');
const selectedArtist = ref(null);
const selectedAlbum = ref(null);
const artists = ref([]);
const albums = ref([]);
const tracks = ref([]);
const errorMessage = ref('');

async function showArtists() {
  level.value = 'artists';
  selectedArtist.value = null;
  selectedAlbum.value = null;
  errorMessage.value = '';

  try {
    const data = await fetchArtists(buildFilterParams());
    artists.value = data.artists || [];
  } catch (err) {
    errorMessage.value = `Failed to load artists: ${err.message}`;
  }
}

async function showAlbums(artist) {
  level.value = 'albums';
  selectedArtist.value = artist;
  selectedAlbum.value = null;
  errorMessage.value = '';

  const params = buildFilterParams();
  params.set('artist', artist);
  try {
    const data = await fetchAlbums(params);
    albums.value = data.albums || [];
  } catch (err) {
    errorMessage.value = `Failed to load albums: ${err.message}`;
  }
}

async function showTracks(artist, album) {
  level.value = 'tracks';
  selectedArtist.value = artist;
  selectedAlbum.value = album;
  errorMessage.value = '';

  const params = buildFilterParams();
  params.set('artist', artist);
  params.set('album', album);
  try {
    const data = await fetchTracks(params);
    tracks.value = data.entries || [];
    store.lastEntries = tracks.value;
  } catch (err) {
    errorMessage.value = `Failed to load tracks: ${err.message}`;
  }
}

function onArtistClick(artist) {
  showAlbums(artist.artist);
}

function onAlbumClick(album) {
  showTracks(selectedArtist.value, album.album);
}

function reloadTracks() {
  return showTracks(selectedArtist.value, selectedAlbum.value);
}

// Re-fetches whatever level is currently displayed — for App.vue's
// refreshCurrentView when artist-album is the active grouping.
function reloadArtistAlbum() {
  if (level.value === 'albums') {
    return showAlbums(selectedArtist.value);
  }
  if (level.value === 'tracks') {
    return reloadTracks();
  }
  return showArtists();
}

defineExpose({ level, showArtists, showAlbums, showTracks, reloadArtistAlbum });
</script>

<template>
  <div>
    <div class="flex flex-wrap items-center gap-1 text-sm text-neutral-400 mb-3">
      <button
        :class="level === 'artists' ? 'text-neutral-200 font-medium' : 'text-blue-400 hover:underline'"
        @click="showArtists"
      >Artists</button>
      <template v-if="level === 'albums' || level === 'tracks'">
        <span> / </span>
        <button
          :class="level === 'albums' ? 'text-neutral-200 font-medium' : 'text-blue-400 hover:underline'"
          @click="showAlbums(selectedArtist)"
        >{{ selectedArtist }}</button>
      </template>
      <template v-if="level === 'tracks'">
        <span> / </span>
        <button class="text-neutral-200 font-medium" @click="reloadTracks">{{ selectedAlbum }}</button>
      </template>
    </div>

    <div v-if="errorMessage" class="text-red-400 text-xs mb-4">{{ errorMessage }}</div>

    <div v-if="level !== 'tracks'" class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 mb-4">
      <template v-if="level === 'artists'">
        <button
          v-for="a in artists"
          :key="a.artist"
          class="text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600"
          @click="onArtistClick(a)"
        >
          <div class="text-sm truncate">{{ a.artist }}</div>
          <div class="text-xs text-neutral-500">{{ a.track_count }} track(s)</div>
        </button>
      </template>
      <template v-else>
        <button
          v-for="a in albums"
          :key="a.album"
          class="text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600"
          @click="onAlbumClick(a)"
        >
          <div class="text-sm truncate">{{ a.album }}</div>
          <div class="text-xs text-neutral-500">{{ a.track_count }} track(s)</div>
        </button>
      </template>
    </div>

    <template v-else>
      <EntryTable v-if="store.presentation === 'table'" :entries="tracks" :sortable="false" @refresh="reloadTracks" />
      <EntryGrid v-else :entries="tracks" />
    </template>
  </div>
</template>
