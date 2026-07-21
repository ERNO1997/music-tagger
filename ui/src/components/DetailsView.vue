<script setup>
import { ref, computed, watch } from 'vue';
import { store } from '../store.js';
import { DETAILS_FIELD_LABELS, RAW_TAG_FIELD_LABELS, EMBEDDED_TAG_FIELD_LABELS } from '../format.js';
import {
  fetchCandidates,
  searchIdentify,
  postIdentifyResolve,
  fetchCoverCandidates,
  postCoverChoose,
  fetchFingerprint,
  fetchEmbeddedTags,
  fetchLyrics,
} from '../api.js';
import { detailsState, closeDetails } from '../composables/useDetails.js';

const props = defineProps({
  refreshCurrentView: { type: Function, required: true },
});

const visible = computed(() => !!detailsState.path);
const entry = ref(null);

const fields = ref([]); // [{ label, value }]
const rawTagFields = ref([]); // [{ label, value }]
const lyricsText = ref('');
const showLyrics = ref(false);
const embeddedTagFields = ref([]);
const embeddedExtras = ref('');
const showEmbeddedTags = ref(false);
const embeddedTagsError = ref('');
const candidates = ref([]);
const candidatesHeading = ref('Choose the correct recording');
const showCandidates = ref(false);
const candidatesError = ref('');

const manualArtist = ref('');
const manualTitle = ref('');
const manualAlbum = ref('');
const manualStatus = ref('');
const manualSearching = ref(false);
let manualPathStatus = '';

const showBrowseCoversToggle = ref(false);
const browsingCovers = ref(false);
const coverCandidates = ref([]);
const coverCandidatesError = ref('');
const loadingCovers = ref(false);
const coverBustCache = ref(0);

const coverUrl = computed(() => {
  if (!entry.value?.has_cover_art) {
    return null;
  }
  const bust = coverBustCache.value ? `&_=${coverBustCache.value}` : '';
  return `/api/v1/library/cover?path=${encodeURIComponent(entry.value.path)}${bust}`;
});

watch(
  () => detailsState.path,
  async (path) => {
    if (!path) {
      return;
    }
    await loadDetails(path);
  },
);

async function loadDetails(path) {
  const found = store.lastEntries.find((e) => e.path === path);
  if (!found) {
    closeDetails();
    return;
  }
  entry.value = found;
  coverBustCache.value = 0;

  fields.value = [];
  for (const [key, label, formatter] of DETAILS_FIELD_LABELS) {
    const value = found[key];
    if (value === undefined || value === null || value === '') {
      continue;
    }
    fields.value.push({ label, value: formatter ? formatter(value) : value });
  }

  rawTagFields.value = [];
  if (found.status !== 'identified') {
    rawTagFields.value = RAW_TAG_FIELD_LABELS
      .filter(([key]) => found[key])
      .map(([key, label]) => ({ label, value: found[key] }));
  }

  showLyrics.value = false;
  lyricsText.value = '';
  if (found.has_lyrics) {
    await loadLyrics(path);
  }

  showEmbeddedTags.value = false;
  embeddedTagFields.value = [];
  embeddedExtras.value = '';
  embeddedTagsError.value = '';
  if (found.tagged) {
    await loadEmbeddedTags(path);
  }

  showCandidates.value = false;
  candidates.value = [];
  candidatesError.value = '';
  candidatesHeading.value = 'Choose the correct recording';
  if (found.status === 'ambiguous') {
    await loadCandidates(path);
  }

  manualPathStatus = found.status;
  manualArtist.value = '';
  manualTitle.value = '';
  manualAlbum.value = '';
  manualStatus.value = '';

  showBrowseCoversToggle.value = found.status === 'identified';
  browsingCovers.value = false;
  coverCandidates.value = [];
  coverCandidatesError.value = '';

  await loadFingerprint(path);
}

async function loadFingerprint(path) {
  try {
    const data = await fetchFingerprint(path);
    if (!data.fingerprint) {
      return;
    }
    fields.value.push({ label: 'Fingerprint', value: data.fingerprint });
  } catch (err) {
    // Best-effort — the details view is still useful without it.
  }
}

async function loadLyrics(path) {
  try {
    const data = await fetchLyrics(path);
    lyricsText.value = data.plain_lyrics || data.synced_lyrics || '';
    showLyrics.value = true;
  } catch (err) {
    lyricsText.value = `Failed to load lyrics: ${err.message}`;
    showLyrics.value = true;
  }
}

async function loadEmbeddedTags(path) {
  try {
    const data = await fetchEmbeddedTags(path);
    embeddedTagFields.value = EMBEDDED_TAG_FIELD_LABELS
      .filter(([key]) => data[key] !== undefined && data[key] !== null && data[key] !== '')
      .map(([key, label]) => ({ label, value: data[key] }));
    const extras = [
      data.has_lyrics ? 'Lyrics embedded' : null,
      data.has_cover_art ? 'Cover art embedded' : null,
    ].filter(Boolean);
    embeddedExtras.value = extras.join(' · ');
    showEmbeddedTags.value = true;
  } catch (err) {
    embeddedTagsError.value = `Failed to load embedded tags: ${err.message}`;
    showEmbeddedTags.value = true;
  }
}

async function loadCandidates(path) {
  try {
    candidates.value = await fetchCandidates(path);
    showCandidates.value = true;
  } catch (err) {
    candidatesError.value = `Failed to load candidates: ${err.message}`;
    showCandidates.value = true;
  }
}

function candidateSummary(candidate) {
  const track = candidate.track_number ? `Track ${candidate.track_number}` : '';
  return [candidate.artist, candidate.album, candidate.title, track].filter(Boolean).join(' – ');
}

async function resolveCandidate(recordingMbid) {
  try {
    await postIdentifyResolve(entry.value.path, recordingMbid);
    await props.refreshCurrentView();
    closeDetails();
  } catch (err) {
    candidatesError.value = `Failed to resolve candidate: ${err.message}`;
  }
}

function luceneEscape(value) {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

function buildManualSearchQuery(artist, title, album) {
  const parts = [];
  if (artist) parts.push(`artist:"${luceneEscape(artist)}"`);
  if (title) parts.push(`recording:"${luceneEscape(title)}"`);
  if (album) parts.push(`release:"${luceneEscape(album)}"`);
  return parts.join(' AND ');
}

async function onManualSearch() {
  const path = entry.value.path;
  const artist = manualArtist.value.trim();
  const title = manualTitle.value.trim();
  const album = manualAlbum.value.trim();

  if (!artist && !title && !album) {
    manualStatus.value = 'Enter at least one of artist, title, or album.';
    return;
  }

  if (manualPathStatus === 'identified' && !confirm('This file is already identified. Searching will immediately discard its current resolved metadata, even if you don\'t pick a result. Continue?')) {
    return;
  }

  const query = buildManualSearchQuery(artist, title, album);
  manualSearching.value = true;
  manualStatus.value = 'Searching…';
  try {
    const results = await searchIdentify(path, query);
    if (results.length === 0) {
      manualStatus.value = 'No matches found.';
      return;
    }
    manualStatus.value = '';
    manualPathStatus = 'ambiguous';
    candidatesHeading.value = 'Search results — choose the correct recording';
    candidates.value = results;
    candidatesError.value = '';
    showCandidates.value = true;
    await props.refreshCurrentView();
  } catch (err) {
    manualStatus.value = `Search failed: ${err.message}`;
  } finally {
    manualSearching.value = false;
  }
}

function toggleBrowseCovers() {
  if (browsingCovers.value) {
    browsingCovers.value = false;
    return;
  }
  loadCoverCandidates();
}

async function loadCoverCandidates() {
  loadingCovers.value = true;
  try {
    coverCandidates.value = await fetchCoverCandidates(entry.value.path);
    coverCandidatesError.value = '';
    browsingCovers.value = true;
  } catch (err) {
    coverCandidatesError.value = `Failed to load cover candidates: ${err.message}`;
    browsingCovers.value = true;
  } finally {
    loadingCovers.value = false;
  }
}

async function chooseCover(candidate) {
  try {
    await postCoverChoose(entry.value.path, candidate.release_mbid, candidate.image_url);
    await props.refreshCurrentView();
    coverBustCache.value = Date.now();
  } catch (err) {
    coverCandidatesError.value = `Failed to choose cover: ${err.message}`;
  }
}

function onOverlayClick(event) {
  if (event.target === event.currentTarget) {
    closeDetails();
  }
}
</script>

<template>
  <div v-if="visible" class="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-10" @click="onOverlayClick">
    <div class="bg-neutral-900 border border-neutral-800 rounded-lg max-w-lg w-full max-h-[85vh] overflow-y-auto">
      <div class="flex items-center justify-between px-5 py-4 border-b border-neutral-800">
        <h2 class="text-lg font-semibold">File Details</h2>
        <button class="text-neutral-400 hover:text-neutral-100 text-xl leading-none" @click="closeDetails">&times;</button>
      </div>

      <img v-if="coverUrl" :src="coverUrl" class="w-full max-h-64 object-contain bg-black" />

      <div v-if="showBrowseCoversToggle" class="px-5 pt-3">
        <button class="text-xs text-blue-400 hover:underline" @click="toggleBrowseCovers">
          {{ loadingCovers ? 'Loading covers…' : (browsingCovers ? 'Hide alternate covers' : 'Browse other covers…') }}
        </button>
      </div>

      <div v-if="browsingCovers" class="px-5 py-4 border-b border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">Choose a cover from this release group</h3>
        <p v-if="coverCandidatesError" class="text-red-400 text-xs">{{ coverCandidatesError }}</p>
        <p v-else-if="coverCandidates.length === 0" class="col-span-4 text-neutral-500 text-xs">No alternate covers found across this release group.</p>
        <div v-else class="grid grid-cols-4 gap-2">
          <button
            v-for="candidate in coverCandidates"
            :key="candidate.release_mbid"
            class="rounded-md overflow-hidden border border-neutral-700 hover:border-blue-400"
            :title="candidate.release_title"
            @click="chooseCover(candidate)"
          >
            <img :src="candidate.thumbnail_url" class="w-full h-20 object-cover" :alt="candidate.release_title" />
          </button>
        </div>
      </div>

      <dl class="px-5 py-4 space-y-2 text-sm">
        <div v-for="f in fields" :key="f.label" class="flex justify-between gap-4">
          <dt class="text-neutral-400">{{ f.label }}</dt>
          <dd class="font-mono text-xs text-right break-all">{{ f.value }}</dd>
        </div>
      </dl>

      <div v-if="rawTagFields.length > 0" class="px-5 py-4 border-t border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">From the file itself (not yet identified)</h3>
        <dl class="space-y-2 text-sm">
          <div v-for="f in rawTagFields" :key="f.label" class="flex justify-between gap-4">
            <dt class="text-neutral-400">{{ f.label }}</dt>
            <dd class="font-mono text-xs text-right break-all">{{ f.value }}</dd>
          </div>
        </dl>
      </div>

      <div v-if="showLyrics" class="px-5 py-4 border-t border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">Lyrics</h3>
        <pre class="whitespace-pre-wrap font-sans text-sm max-h-64 overflow-y-auto text-neutral-200">{{ lyricsText }}</pre>
      </div>

      <div v-if="showEmbeddedTags" class="px-5 py-4 border-t border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">Embedded Tags (read from the file itself)</h3>
        <p v-if="embeddedTagsError" class="text-red-400 text-xs">{{ embeddedTagsError }}</p>
        <dl v-else class="space-y-2 text-sm">
          <div v-for="f in embeddedTagFields" :key="f.label" class="flex justify-between gap-4">
            <dt class="text-neutral-400">{{ f.label }}</dt>
            <dd class="font-mono text-xs text-right break-all">{{ f.value }}</dd>
          </div>
          <div v-if="embeddedExtras" class="text-neutral-400 text-xs">{{ embeddedExtras }}</div>
        </dl>
      </div>

      <div v-if="showCandidates" class="px-5 py-4 border-t border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">{{ candidatesHeading }}</h3>
        <p v-if="candidatesError" class="text-sm text-red-400">{{ candidatesError }}</p>
        <div v-else class="space-y-2 text-sm">
          <div
            v-for="candidate in candidates"
            :key="candidate.recording_mbid"
            class="flex items-center justify-between gap-3 bg-neutral-800 rounded-md px-3 py-2"
          >
            <span class="text-neutral-200">{{ candidateSummary(candidate) }}</span>
            <button
              class="use-candidate-button shrink-0 rounded-md bg-blue-600 text-white text-xs font-medium px-3 py-1.5 hover:bg-blue-500"
              @click="resolveCandidate(candidate.recording_mbid)"
            >Use this</button>
          </div>
        </div>
      </div>

      <div class="px-5 py-4 border-t border-neutral-800">
        <h3 class="text-neutral-400 text-xs uppercase mb-2">Search manually</h3>
        <p class="text-neutral-500 text-xs mb-2">Searches MusicBrainz directly by text — no audio fingerprint needed. Fill in whichever fields you know.</p>
        <div class="grid grid-cols-3 gap-2 mb-2">
          <input v-model="manualArtist" type="text" placeholder="Artist" class="rounded-md bg-neutral-800 border border-neutral-700 text-sm px-2 py-1.5" />
          <input v-model="manualTitle" type="text" placeholder="Title" class="rounded-md bg-neutral-800 border border-neutral-700 text-sm px-2 py-1.5" />
          <input v-model="manualAlbum" type="text" placeholder="Album" class="rounded-md bg-neutral-800 border border-neutral-700 text-sm px-2 py-1.5" />
        </div>
        <div class="flex items-center gap-2">
          <button
            class="rounded-md bg-blue-600 text-white text-xs font-medium px-3 py-1.5 hover:bg-blue-500 disabled:opacity-50"
            :disabled="manualSearching"
            @click="onManualSearch"
          >Search</button>
          <span class="text-xs text-neutral-400">{{ manualStatus }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
