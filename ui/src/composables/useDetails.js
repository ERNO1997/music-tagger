import { reactive } from 'vue';

// Which tracked file's details overlay is open, if any — DetailsView.vue
// watches `path` and loads everything else (fingerprint, lyrics, embedded
// tags, candidates) itself, exactly like the original details.js did on
// openDetails().
export const detailsState = reactive({ path: null });

export function openDetails(path) {
  detailsState.path = path;
}

export function closeDetails() {
  detailsState.path = null;
}
