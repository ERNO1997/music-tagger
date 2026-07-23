## 1. Move the page-size selector out of the filters row

- [x] 1.1 Remove the page-size `<select>` and `onPageSizeChange` handler from `FilterBar.vue`
- [x] 1.2 Add the same `<select>` and handler (mutating `store.pageState.limit`, then re-fetching) to `AllGroupingView.vue`'s pagination footer, next to its Prev/Next buttons
- [x] 1.3 Add the same `<select>` and handler to `FolderGroupingView.vue`'s pagination footer, next to its Prev/Next buttons

## 2. Empty-state message in the shared row/card renderers

- [x] 2.1 In `EntryTable.vue`, render a single message row (e.g. a `<td>` spanning all columns) reading "No items match the current filters." when `entries.length === 0`, in place of the (otherwise empty) row list
- [x] 2.2 In `EntryGrid.vue`, render the same message in a plain centered block where the card grid would be, when `entries.length === 0`

## 3. Sticky title/actions row

- [x] 3.1 In `App.vue`, make the existing title + action-buttons row (`<h1>` and the Identify/Enrich/Tag/Relocate Selected/Refresh buttons) `position: sticky; top: 0`, with an opaque background matching `body`'s, a `z-index` above scrolled content, and a bottom border
- [x] 3.2 Confirm the description paragraph, `FilterBar`, grouping/presentation tabs, and `SelectionBanner` remain in normal document flow (not pinned)

## 4. Verification

- [x] 4.1 Confirm the page-size selector appears next to Prev/Next in both the All grouping and the Folder grouping, and that changing it in either place is reflected in both (shared `store.pageState.limit`)
- [x] 4.2 Confirm the filters row no longer contains a page-size control and reads less cramped
- [x] 4.3 Filter the All grouping down to zero matching files and confirm the "No items" message appears in both Table and Grid presentation
- [x] 4.4 Browse to an empty folder (Folder grouping) and an empty/non-existent search result in the Artist-Album grouping's track level, confirming the same message appears there too
- [x] 4.5 Scroll a long list and confirm the title and action buttons stay pinned to the top while the description, filters, tabs, and selection banner scroll away normally
- [x] 4.6 Confirm bulk actions (e.g. Identify Selected) still work correctly when triggered from the pinned/sticky state
