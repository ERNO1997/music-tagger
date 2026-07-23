<script setup>
import { ref } from 'vue';
import { store, buildFilterParams } from '../../store.js';
import { fetchTree } from '../../api.js';
import EntryTable from '../EntryTable.vue';
import EntryGrid from '../EntryGrid.vue';

// rootPath is captured from the very first response (whatever prefix the
// server resolved an omitted "path" to, i.e. the music root) — used only to
// render a shorter "Home" breadcrumb segment. currentPath is the prefix
// currently being browsed; offset paginates its direct-files list.
let rootPath = null;
const currentPath = ref(null);
const offset = ref(0);

const directories = ref([]);
const files = ref([]);
const total = ref(0);
const crumbs = ref([]);
const errorMessage = ref('');

// Navigates to path (undefined/null means "wherever we last were, or the
// music root on first load") and re-fetches.
async function loadTree(path) {
  if (path !== undefined && path !== currentPath.value) {
    currentPath.value = path;
    offset.value = 0;
  }
  await fetchAndRender();
}

async function fetchAndRender() {
  const params = buildFilterParams();
  if (currentPath.value) {
    params.set('path', currentPath.value);
  }
  params.set('limit', String(store.pageState.limit));
  params.set('offset', String(offset.value));

  try {
    const data = await fetchTree(params);
    currentPath.value = data.path;
    if (rootPath === null) {
      rootPath = data.path;
    }
    buildBreadcrumb();
    directories.value = data.directories || [];
    files.value = data.files?.entries || [];
    total.value = data.files?.total || 0;
    store.lastEntries = files.value;
    errorMessage.value = '';
  } catch (err) {
    errorMessage.value = `Failed to load folder: ${err.message}`;
  }
}

function buildBreadcrumb() {
  if (rootPath === null || currentPath.value === null) {
    crumbs.value = [];
    return;
  }
  const rest = currentPath.value === rootPath ? '' : currentPath.value.slice(rootPath.length).replace(/^\/+/, '');
  const segments = rest ? rest.split('/') : [];

  const result = [{ label: 'Home', path: rootPath }];
  let acc = rootPath;
  for (const segment of segments) {
    acc = `${acc}/${segment}`;
    result.push({ label: segment, path: acc });
  }
  crumbs.value = result;
}

const paginationInfo = () => {
  if (total.value === 0) {
    return files.value.length === 0 && directories.value.length === 0
      ? 'No tracked files under this folder.'
      : 'No files directly in this folder.';
  }
  const start = offset.value + 1;
  const end = Math.min(offset.value + store.pageState.limit, total.value);
  return `${start}–${end} of ${total.value} file(s) in this folder`;
};

function onOpenDirectory(dir) {
  loadTree(`${currentPath.value}/${dir.name}`);
}

function onCrumbClick(crumb) {
  loadTree(crumb.path);
}

function onPrevPage() {
  offset.value = Math.max(0, offset.value - store.pageState.limit);
  fetchAndRender();
}

function onNextPage() {
  offset.value += store.pageState.limit;
  fetchAndRender();
}

function onPageSizeChange(event) {
  store.pageState.limit = Number(event.target.value);
  offset.value = 0;
  fetchAndRender();
}

// Re-fetches whatever folder is currently displayed, keeping the current
// page position — for App.vue's refreshCurrentView after a bulk-action job
// updates the currently-visible listing.
function reloadTree() {
  return fetchAndRender();
}

// Re-fetches whatever folder is currently displayed, resetting back to its
// first page — for App.vue's filter/search change handlers, since a
// narrower filter can leave a previously-valid offset past the end.
function resetTreePage() {
  offset.value = 0;
  return fetchAndRender();
}

defineExpose({ loadTree, reloadTree, resetTreePage });
</script>

<template>
  <div>
    <div class="flex flex-wrap items-center gap-1 text-sm text-neutral-400 mb-3">
      <template v-for="(crumb, i) in crumbs" :key="crumb.path">
        <span v-if="i > 0"> / </span>
        <button
          :class="i === crumbs.length - 1 ? 'text-neutral-200 font-medium' : 'text-blue-400 hover:underline'"
          @click="onCrumbClick(crumb)"
        >{{ crumb.label }}</button>
      </template>
    </div>

    <div v-if="errorMessage" class="text-red-400 mb-4">{{ errorMessage }}</div>

    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 mb-4">
      <button
        v-for="dir in directories"
        :key="dir.name"
        class="text-left bg-neutral-900 border border-neutral-800 rounded-md px-3 py-2 hover:border-neutral-600"
        @click="onOpenDirectory(dir)"
      >
        <div class="text-sm truncate">&#128193; {{ dir.name }}</div>
        <div class="text-xs text-neutral-500">{{ dir.identified_count }}/{{ dir.total_count }} identified</div>
      </button>
    </div>

    <EntryTable v-if="store.presentation === 'table'" :entries="files" :sortable="false" @refresh="reloadTree" />
    <EntryGrid v-else :entries="files" />

    <div class="flex items-center justify-between mt-3 text-sm text-neutral-400">
      <div>{{ paginationInfo() }}</div>
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
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" :disabled="offset === 0" @click="onPrevPage">Prev</button>
        <button class="rounded-md bg-neutral-900 border border-neutral-800 px-3 py-1.5 disabled:opacity-40 disabled:cursor-not-allowed" :disabled="offset + store.pageState.limit >= total" @click="onNextPage">Next</button>
      </div>
    </div>
  </div>
</template>
