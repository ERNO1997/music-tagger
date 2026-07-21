import { store, buildListParams } from '../store.js';
import { fetchLibrary } from '../api.js';
import { libraryStatus } from './useLibraryStatus.js';

// Shared fetch for the table AND grid views — both render the exact same
// GET /api/v1/library response (store.lastEntries/store.total), just
// differently. This mirrors today's table.js loadLibrary(), which both
// views relied on via main.js's renderCurrentView dispatch — including its
// single shared store.pageState.offset, which is the root cause of the
// existing grid-pagination bug (ported as-is; see grid-view-pagination-fix).
export async function loadLibrary() {
  try {
    const params = buildListParams();
    const data = await fetchLibrary(params);
    store.total = data.total || 0;
    store.lastEntries = data.entries || [];

    if (store.lastEntries.length === 0) {
      libraryStatus.text = store.total === 0 ? 'No files match the current filters.' : 'No tracked files yet.';
    } else {
      libraryStatus.text = `Showing ${store.lastEntries.length} of ${store.total} tracked file(s).`;
    }
    libraryStatus.isError = false;
  } catch (err) {
    libraryStatus.text = `Failed to load library: ${err.message}`;
    libraryStatus.isError = true;
  }
}
