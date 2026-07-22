## Why

Grid view's pagination controls (`#grid-pagination-info`/`#grid-prev-page`/`#grid-next-page` in the original markup, now `GridView.vue`) render but do nothing — they were never wired to anything, ported as-is (intentionally, per `vue-adoption-shell`'s design.md) rather than fixed during that migration. Table and Grid already share the same underlying page (`store.pageState.offset`, via the same `loadLibrary()`), so the only thing missing is Grid having its own working controls over that same shared state — today, paging only works from the Table view, forcing a user in Grid view to switch to Table, page, and switch back to see the effect.

## What Changes

- Wire Grid view's Prev/Next controls and page-info text to `store.pageState.offset`/`loadLibrary()`, identically to how Table view's already work — since both already read/write the same shared page state, this is a same-page, same-selection, same-filter fix with no other behavioral change.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: the "Web UI listing of scan results" requirement's grid-view scenario is tightened to explicitly include page navigation, and a new scenario documents that grid's own pagination controls must work without needing the table view.

## Impact

- Changed code: `ui/src/components/views/GridView.vue` only — add a computed pagination-info string and Prev/Next handlers mutating `store.pageState.offset` then calling `loadLibrary()`, copied from `TableView.vue`'s existing equivalents.
- No backend, API, or database changes.
- No dependency on any other in-progress change, though `presentation-grouping-split` may later fold this concern into a shared pagination component — not a blocker for landing this fix now.
