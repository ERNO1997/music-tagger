// Shared mutable UI state, packaged as one object so importers can mutate
// its properties (`state.total = 5`) without running into ES modules'
// read-only-imported-binding restriction (which would block a bare
// `import { total } from './state.js'; total = 5;`).
export const state = {
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
  total: 0,
  lastEntries: [],
  // Only 'table' is valid today — a seam for add-library-views-and-playback
  // to introduce 'grid' / 'tree' / 'artist-album'.
  currentView: 'table',
};

export function getSelectionCount() {
  return state.selectionMode === 'filter' ? state.total : state.selectedPaths.size;
}

export function currentFilterPayload() {
  const filter = { status: state.filterState.status, q: state.filterState.q };
  if (state.filterState.tagged !== '') filter.tagged = state.filterState.tagged === 'true';
  if (state.filterState.relocated !== '') filter.relocated = state.filterState.relocated === 'true';
  if (state.filterState.hasLyrics !== '') filter.has_lyrics = state.filterState.hasLyrics === 'true';
  if (state.filterState.hasCoverArt !== '') filter.has_cover_art = state.filterState.hasCoverArt === 'true';
  return filter;
}

export function getSelectionBody() {
  if (state.selectionMode === 'filter') {
    return { filter: currentFilterPayload() };
  }
  return { paths: [...state.selectedPaths] };
}

export function buildListParams() {
  const params = new URLSearchParams();
  if (state.filterState.status) params.set('status', state.filterState.status);
  if (state.filterState.tagged !== '') params.set('tagged', state.filterState.tagged);
  if (state.filterState.relocated !== '') params.set('relocated', state.filterState.relocated);
  if (state.filterState.hasLyrics !== '') params.set('has_lyrics', state.filterState.hasLyrics);
  if (state.filterState.hasCoverArt !== '') params.set('has_cover_art', state.filterState.hasCoverArt);
  if (state.filterState.q) params.set('q', state.filterState.q);
  if (state.sortState.by) params.set('sort', state.sortState.by);
  params.set('order', state.sortState.desc ? 'desc' : 'asc');
  params.set('limit', String(state.pageState.limit));
  params.set('offset', String(state.pageState.offset));
  return params;
}
