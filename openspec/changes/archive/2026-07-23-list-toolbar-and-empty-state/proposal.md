## Why

Three small usability gaps in the library listing: the page-size selector sits crowded among five status/outcome filters even though it's not a filter at all, an empty result set renders as a bare table header with no explanation, and scrolling down a long list carries the bulk-action buttons out of view along with everything else, forcing a scroll back to the top to act on a selection.

## What Changes

- Move the page-size selector ("50 / page") out of the filters row and next to each grouping's own Prev/Next controls, since it's a pagination control, not a filter.
- When a table or grid has no entries to show, render an explicit "No items" message in its place instead of an empty header with no rows.
- Make the page's title and its bulk-action buttons (Identify/Enrich/Tag/Relocate Selected, Refresh) stick to the top of the viewport while scrolling, so they stay reachable over a long list. The description text, filters, grouping/presentation tabs, and selection banner are not pinned — only the title and action row.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: the "Web UI listing of scan results" requirement gains an explicit scenario for what the UI shows when a grouping's current listing has no entries.

## Impact

- Changed code: `ui/src/components/FilterBar.vue` (page-size selector removed), `ui/src/components/views/AllGroupingView.vue` and `ui/src/components/views/FolderGroupingView.vue` (page-size selector added next to their own Prev/Next), `ui/src/components/EntryTable.vue` and `ui/src/components/EntryGrid.vue` (empty-state message), `ui/src/App.vue` (sticky title/action row).
- The Artist/Album grouping's artists/albums/tracks levels aren't paginated today (each fetched whole in one response), so it gets no page-size control — nothing changes there beyond the empty-state message, which already applies to it via the shared `EntryTable`/`EntryGrid` components.
- No backend, API, or database changes.
- No dependency on any other in-progress change.
