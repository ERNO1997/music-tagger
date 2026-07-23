<script setup>
import { ref } from 'vue';
import { store, buildFilterParams } from '../../store.js';
import { fetchArtists, fetchAlbums, fetchTracks, fetchArtistCompleteness, fetchAlbumCompleteness } from '../../api.js';
import EntryTable from '../EntryTable.vue';
import EntryGrid from '../EntryGrid.vue';

// level is one of 'artists' | 'albums' | 'tracks' — the current drill-down
// depth. selectedArtist/selectedAlbum hold the display label shown in the
// breadcrumb; selectedArtistKey/selectedAlbumKey hold the grouping key
// (MusicBrainz ID when identified, else a name-derived key) used for every
// API call from here on, since two groupings can share a display label
// (see label_collision) and only the key disambiguates them. Exposed (auto-
// unwrapped by defineExpose) so App.vue can tell whether a file listing —
// and therefore the presentation toggle — is showing.
const level = ref('artists');
const selectedArtist = ref(null);
const selectedArtistKey = ref(null);
const selectedAlbum = ref(null);
const selectedAlbumKey = ref(null);
const artists = ref([]);
const albums = ref([]);
const tracks = ref([]);
const errorMessage = ref('');

// A grouping's completeness check is only offered when its key is an actual
// MusicBrainz ID, not a "name:"-derived fallback for an unidentified file.
function isIdentifiedKey(key) {
  return !!key && !key.startsWith('name:');
}

function mismatchTitle(entry) {
  const parts = [];
  if (entry.name_mismatch) {
    parts.push(`Inconsistent name across files: ${(entry.distinct_names || []).join(', ')}`);
  }
  if (entry.label_collision) {
    parts.push('Another distinct entry has the same display name');
  }
  return parts.join(' — ');
}

const artistCompleteness = ref(null);
const artistCompletenessLoading = ref(false);
const artistCompletenessUnavailable = ref(false);
const artistCompletenessError = ref('');

async function loadArtistCompleteness(refresh = false) {
  artistCompleteness.value = null;
  artistCompletenessError.value = '';
  artistCompletenessUnavailable.value = false;

  if (!isIdentifiedKey(selectedArtistKey.value)) {
    artistCompletenessUnavailable.value = true;
    return;
  }

  artistCompletenessLoading.value = true;
  try {
    const params = new URLSearchParams({ artist_key: selectedArtistKey.value });
    if (refresh) params.set('refresh', 'true');
    artistCompleteness.value = await fetchArtistCompleteness(params);
  } catch (err) {
    if (err.unavailable) {
      artistCompletenessUnavailable.value = true;
    } else {
      artistCompletenessError.value = err.message;
    }
  } finally {
    artistCompletenessLoading.value = false;
  }
}

const albumCompleteness = ref(null);
const albumCompletenessLoading = ref(false);
const albumCompletenessUnavailable = ref(false);
const albumCompletenessError = ref('');

async function loadAlbumCompleteness(refresh = false) {
  albumCompleteness.value = null;
  albumCompletenessError.value = '';
  albumCompletenessUnavailable.value = false;

  if (!isIdentifiedKey(selectedAlbumKey.value)) {
    albumCompletenessUnavailable.value = true;
    return;
  }

  albumCompletenessLoading.value = true;
  try {
    const params = new URLSearchParams({ artist_key: selectedArtistKey.value, album_key: selectedAlbumKey.value });
    if (refresh) params.set('refresh', 'true');
    albumCompleteness.value = await fetchAlbumCompleteness(params);
  } catch (err) {
    if (err.unavailable) {
      albumCompletenessUnavailable.value = true;
    } else {
      albumCompletenessError.value = err.message;
    }
  } finally {
    albumCompletenessLoading.value = false;
  }
}

async function showArtists() {
  level.value = 'artists';
  selectedArtist.value = null;
  selectedArtistKey.value = null;
  selectedAlbum.value = null;
  selectedAlbumKey.value = null;
  errorMessage.value = '';

  try {
    const data = await fetchArtists(buildFilterParams());
    artists.value = data.artists || [];
  } catch (err) {
    errorMessage.value = `Failed to load artists: ${err.message}`;
  }
}

async function showAlbums(artistKey, artistLabel) {
  level.value = 'albums';
  selectedArtist.value = artistLabel;
  selectedArtistKey.value = artistKey;
  selectedAlbum.value = null;
  selectedAlbumKey.value = null;
  errorMessage.value = '';

  const params = buildFilterParams();
  params.set('artist_key', artistKey);
  try {
    const data = await fetchAlbums(params);
    albums.value = data.albums || [];
  } catch (err) {
    errorMessage.value = `Failed to load albums: ${err.message}`;
  }

  // Fired without awaiting: the completeness panel fills in
  // asynchronously so the rate-gated MusicBrainz round trip never blocks
  // navigation.
  loadArtistCompleteness();
}

async function showTracks(albumKey, albumLabel) {
  level.value = 'tracks';
  selectedAlbum.value = albumLabel;
  selectedAlbumKey.value = albumKey;
  errorMessage.value = '';

  const params = buildFilterParams();
  params.set('artist_key', selectedArtistKey.value);
  params.set('album_key', albumKey);
  try {
    const data = await fetchTracks(params);
    tracks.value = data.entries || [];
    store.lastEntries = tracks.value;
  } catch (err) {
    errorMessage.value = `Failed to load tracks: ${err.message}`;
  }

  loadAlbumCompleteness();
}

function onArtistClick(artist) {
  showAlbums(artist.artist_key, artist.artist);
}

function onAlbumClick(album) {
  showTracks(album.album_key, album.album);
}

function reloadTracks() {
  return showTracks(selectedAlbumKey.value, selectedAlbum.value);
}

// Re-fetches whatever level is currently displayed — for App.vue's
// refreshCurrentView when artist-album is the active grouping.
function reloadArtistAlbum() {
  if (level.value === 'albums') {
    return showAlbums(selectedArtistKey.value, selectedArtist.value);
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
          @click="showAlbums(selectedArtistKey, selectedArtist)"
        >{{ selectedArtist }}</button>
      </template>
      <template v-if="level === 'tracks'">
        <span> / </span>
        <button class="text-neutral-200 font-medium" @click="reloadTracks">{{ selectedAlbum }}</button>
      </template>
    </div>

    <div v-if="errorMessage" class="text-red-400 text-xs mb-4">{{ errorMessage }}</div>

    <div v-if="level === 'albums'" class="mb-3 text-xs">
      <span v-if="artistCompletenessLoading" class="text-neutral-500">Checking MusicBrainz…</span>
      <span v-else-if="artistCompletenessUnavailable" class="text-neutral-600">Completeness check unavailable — this artist isn't identified.</span>
      <span v-else-if="artistCompletenessError" class="text-red-400">
        Completeness check failed: {{ artistCompletenessError }}
        <button class="text-blue-400 hover:underline ml-1" @click="loadArtistCompleteness(true)">Retry</button>
      </span>
      <div v-else-if="artistCompleteness" class="flex flex-wrap items-center gap-2 text-neutral-400">
        <span>{{ artistCompleteness.owned_albums }}/{{ artistCompleteness.total_albums }} albums in your library</span>
        <button class="text-blue-400 hover:underline" @click="loadArtistCompleteness(true)">Recheck</button>
        <span v-if="artistCompleteness.missing && artistCompleteness.missing.length" class="text-neutral-500">
          Missing: {{ artistCompleteness.missing.map(m => m.year ? `${m.title} (${m.year})` : m.title).join(', ') }}
        </span>
      </div>
    </div>

    <div v-if="level === 'tracks'" class="mb-3 text-xs">
      <span v-if="albumCompletenessLoading" class="text-neutral-500">Checking MusicBrainz…</span>
      <span v-else-if="albumCompletenessUnavailable" class="text-neutral-600">Completeness check unavailable — this album isn't identified.</span>
      <span v-else-if="albumCompletenessError" class="text-red-400">
        Completeness check failed: {{ albumCompletenessError }}
        <button class="text-blue-400 hover:underline ml-1" @click="loadAlbumCompleteness(true)">Retry</button>
      </span>
      <div v-else-if="albumCompleteness" class="flex flex-wrap items-center gap-2 text-neutral-400">
        <span>{{ albumCompleteness.owned_tracks }}/{{ albumCompleteness.total_tracks }} tracks in your library</span>
        <button class="text-blue-400 hover:underline" @click="loadAlbumCompleteness(true)">Recheck</button>
        <span v-if="albumCompleteness.release_mismatch" class="text-amber-400">
          ⚠ tracks span more than one release edition — counts may be approximate
        </span>
        <span v-if="albumCompleteness.missing && albumCompleteness.missing.length" class="text-neutral-500">
          Missing: {{ albumCompleteness.missing.map(m => m.track_number ? `#${m.track_number} ${m.title}` : m.title).join(', ') }}
        </span>
      </div>
    </div>

    <div v-if="level !== 'tracks'" class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 mb-4">
      <template v-if="level === 'artists'">
        <button
          v-for="a in artists"
          :key="a.artist_key"
          class="text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600"
          @click="onArtistClick(a)"
        >
          <div class="flex items-center gap-1 text-sm truncate">
            <span class="truncate">{{ a.artist }}</span>
            <span v-if="a.name_mismatch || a.label_collision" class="text-amber-400 shrink-0" :title="mismatchTitle(a)">⚠</span>
          </div>
          <div class="text-xs text-neutral-500">{{ a.track_count }} track(s)</div>
        </button>
      </template>
      <template v-else>
        <button
          v-for="a in albums"
          :key="a.album_key"
          class="text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600"
          @click="onAlbumClick(a)"
        >
          <div class="flex items-center gap-1 text-sm truncate">
            <span class="truncate">{{ a.album }}</span>
            <span v-if="a.name_mismatch || a.label_collision" class="text-amber-400 shrink-0" :title="mismatchTitle(a)">⚠</span>
          </div>
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
