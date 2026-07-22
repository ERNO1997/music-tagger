import { reactive } from 'vue';

// Shared reactive state — the Vue equivalent of the old state.js, same
// shape and same fields, just wrapped in reactive() so components
// re-render automatically on mutation instead of a manual render() call.
export const store = reactive({
  filterState: { status: '', tagged: '', relocated: '', hasLyrics: '', hasCoverArt: '', q: '' },
  sortState: { by: 'path', desc: false },
  pageState: { limit: 50, offset: 0 },
  selectedPaths: new Set(),
  // 'explicit': selectedPaths enumerates exactly what's selected, across
  // however many pages the user has visited. 'filter': every file currently
  // matching filterState is considered selected, however many that is — the
  // server re-resolves the matching set at execution time (see
  // resolveSelection on the API side), so this never enumerates paths
  // client-side.
  selectionMode: 'explicit',
  // Whether the table/grid view is currently showing only the explicitly
  // selected files (via the selection endpoint) instead of the current
  // filter (via GET /api/v1/library). Meaningless in 'filter' selection
  // mode, where the current filtered listing already is the selection.
  showSelectedOnly: false,
  total: 0,
  lastEntries: [],
  grouping: 'all',
  presentation: 'table',
});

export function getSelectionCount() {
  return store.selectionMode === 'filter' ? store.total : store.selectedPaths.size;
}

export function currentFilterPayload() {
  const filter = { status: store.filterState.status, q: store.filterState.q };
  if (store.filterState.tagged !== '') filter.tagged = store.filterState.tagged === 'true';
  if (store.filterState.relocated !== '') filter.relocated = store.filterState.relocated === 'true';
  if (store.filterState.hasLyrics !== '') filter.has_lyrics = store.filterState.hasLyrics === 'true';
  if (store.filterState.hasCoverArt !== '') filter.has_cover_art = store.filterState.hasCoverArt === 'true';
  return filter;
}

export function getSelectionBody() {
  if (store.selectionMode === 'filter') {
    return { filter: currentFilterPayload() };
  }
  return { paths: [...store.selectedPaths] };
}

// buildFilterParams returns just the status/tagged/relocated/has_lyrics/
// has_cover_art/q query parameters — shared by every view (table, grid,
// tree, artist-album), each of which layers its own sort/pagination
// parameters on top where applicable.
export function buildFilterParams() {
  const params = new URLSearchParams();
  if (store.filterState.status) params.set('status', store.filterState.status);
  if (store.filterState.tagged !== '') params.set('tagged', store.filterState.tagged);
  if (store.filterState.relocated !== '') params.set('relocated', store.filterState.relocated);
  if (store.filterState.hasLyrics !== '') params.set('has_lyrics', store.filterState.hasLyrics);
  if (store.filterState.hasCoverArt !== '') params.set('has_cover_art', store.filterState.hasCoverArt);
  if (store.filterState.q) params.set('q', store.filterState.q);
  return params;
}

export function buildListParams() {
  const params = buildFilterParams();
  if (store.sortState.by) params.set('sort', store.sortState.by);
  params.set('order', store.sortState.desc ? 'desc' : 'asc');
  params.set('limit', String(store.pageState.limit));
  params.set('offset', String(store.pageState.offset));
  return params;
}
