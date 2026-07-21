<script setup>
import { ref } from 'vue';
import { store, buildFilterParams } from '../../store.js';
import { formatDuration } from '../../format.js';
import { statusLabel, statusClass, metadataText, hasRawMetadata, coverSrc } from '../../entryDisplay.js';
import { fetchArtists, fetchAlbums, fetchTracks } from '../../api.js';
import { playTrack } from '../../composables/usePlayer.js';
import { openDetails } from '../../composables/useDetails.js';

// level is one of 'artists' | 'albums' | 'tracks' — the current drill-down
// depth. selectedArtist/selectedAlbum are set as the user drills in.
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

function onTrackClick(entry) {
  openDetails(entry.path);
}

function onPlay(entry) {
  playTrack(entry);
}

// Re-fetches whatever level is currently displayed — for App.vue's
// refreshCurrentView when artist-album is the active view.
function reloadArtistAlbum() {
  if (level.value === 'albums') {
    return showAlbums(selectedArtist.value);
  }
  if (level.value === 'tracks') {
    return showTracks(selectedArtist.value, selectedAlbum.value);
  }
  return showArtists();
}

defineExpose({ showArtists, showAlbums, showTracks, reloadArtistAlbum });
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
        <button class="text-neutral-200 font-medium" @click="showTracks(selectedArtist, selectedAlbum)">{{ selectedAlbum }}</button>
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

    <div v-else class="overflow-x-auto rounded-lg border border-neutral-800">
      <table class="w-full text-sm text-left">
        <thead class="bg-neutral-900 text-neutral-400 uppercase text-xs">
          <tr>
            <th class="px-4 py-3">Cover</th>
            <th class="px-4 py-3">Path</th>
            <th class="px-4 py-3">Format</th>
            <th class="px-4 py-3">Duration</th>
            <th class="px-4 py-3">Status</th>
            <th class="px-4 py-3">Artist / Album / Title / Track</th>
            <th class="px-4 py-3">Play</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-neutral-800">
          <tr
            v-for="entry in tracks"
            :key="entry.path"
            class="cursor-pointer hover:bg-neutral-900"
            @click="onTrackClick(entry)"
          >
            <td class="px-4 py-3">
              <img v-if="coverSrc(entry)" :src="coverSrc(entry)" class="w-10 h-10 rounded object-cover" alt="" />
              <div v-else class="w-10 h-10 rounded bg-neutral-800"></div>
            </td>
            <td class="px-4 py-3 font-mono text-xs">{{ entry.path }}</td>
            <td class="px-4 py-3 uppercase">{{ entry.format }}</td>
            <td class="px-4 py-3">{{ formatDuration(entry.duration_seconds) }}</td>
            <td class="px-4 py-3" :class="statusClass(entry)">{{ statusLabel(entry) }}</td>
            <td class="px-4 py-3">
              <span v-if="entry.status === 'identified'">{{ metadataText(entry) }}</span>
              <span v-else-if="hasRawMetadata(entry)" class="italic text-neutral-500" title="From the file's own tags, not yet identified">{{ metadataText(entry) }}</span>
              <span v-else>—</span>
            </td>
            <td class="px-4 py-3" @click.stop>
              <span v-if="entry.status === 'missing'">—</span>
              <button v-else class="play-button text-neutral-300 hover:text-white" title="Play" @click="onPlay(entry)">&#9654;</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
