import { store, buildListParams, getSelectionBody } from '../store.js';
import { fetchLibrary, fetchSelection } from '../api.js';
import { libraryStatus } from './useLibraryStatus.js';

// Shared fetch for the table AND grid views — both render the exact same
// response (store.lastEntries/store.total), just differently. This mirrors
// today's table.js loadLibrary(), which both views relied on via main.js's
// renderCurrentView dispatch — including its single shared
// store.pageState.offset, which is the root cause of the existing
// grid-pagination bug (ported as-is; see grid-view-pagination-fix).
//
// When store.showSelectedOnly is on (explicit selection mode only — it's
// meaningless in filter mode, where the current filtered listing already is
// the selection), this fetches via the selection endpoint instead of
// GET /api/v1/library, reusing the exact same sort/pagination params.
export async function loadLibrary() {
  if (store.showSelectedOnly && store.selectedPaths.size === 0) {
    // Nothing left to show selected-only (e.g. the last row was unchecked) —
    // fall back to the normal filtered listing rather than erroring.
    store.showSelectedOnly = false;
  }

  try {
    const params = buildListParams();
    const data = store.showSelectedOnly
      ? await fetchSelection(getSelectionBody(), params)
      : await fetchLibrary(params);

    if ((data.entries || []).length === 0 && store.pageState.offset > 0 && (data.total || 0) > 0) {
      // The current page emptied out from under us (e.g. unchecking the
      // last selected row on this page) — fall back to page 1 rather than
      // showing a stranded empty page.
      store.pageState.offset = 0;
      return loadLibrary();
    }

    store.total = data.total || 0;
    store.lastEntries = data.entries || [];

    if (store.lastEntries.length === 0) {
      if (store.showSelectedOnly) {
        libraryStatus.text = 'No selected files.';
      } else {
        libraryStatus.text = store.total === 0 ? 'No files match the current filters.' : 'No tracked files yet.';
      }
    } else {
      libraryStatus.text = `Showing ${store.lastEntries.length} of ${store.total} tracked file(s).`;
    }
    libraryStatus.isError = false;
  } catch (err) {
    libraryStatus.text = `Failed to load library: ${err.message}`;
    libraryStatus.isError = true;
  }
}
