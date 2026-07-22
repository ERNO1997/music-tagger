## Why

Today's four tabs (Table, Grid, Folder Tree, Artist/Album) conflate two independent questions: *how are files grouped* (a flat list, by folder, by artist/album) and *how is the current listing presented* (rows or cover-forward cards). Because they're conflated, Folder Tree and Artist-Album only know how to render their file/track listings as a table — there's no way to browse by folder or by artist/album with the cover-forward card presentation Grid already offers for the flat list. It also means table/card rendering logic is duplicated across as many as four view components for what's fundamentally the same "render this page of entries" concern, and it's the direct root cause of the (separately fixed) grid-pagination bug: pagination lived per-view instead of per-grouping.

## What Changes

- Replace the four flat tabs with two independent controls: a **grouping** selector (All / Folder / Artist-Album) and a **presentation** selector (Table / Grid), so any grouping's file listing can be shown either way.
- Extract the row/card rendering, per-row selection, and pagination logic currently duplicated across `TableView.vue`/`GridView.vue` (and, if already landed, `TreeView.vue`/`ArtistAlbumView.vue`'s selection code from `tree-and-artist-album-selection`) into shared presentation components used by all three groupings.
- Presentation is only offered where there's an actual file listing to present: Folder grouping always has one (a folder's files, however many); Artist-Album grouping only has one at the drilled-into-an-album (`tracks`) level — the artists and albums levels remain card grids of groups, not files, regardless of presentation choice.
- Switching grouping or presentation independently preserves the other, the active filter/search/sort, the current selection, and (within a grouping) the current pagination/drill-down position.
- Supersedes `grid-view-pagination-fix`'s standalone fix: once pagination is hoisted to the grouping level (shared by both presentations), there's no more separate "grid view's own pagination controls" to keep working — this change's spec delta restates the relevant scenarios in the grouping/presentation model. If `grid-view-pagination-fix` has already landed by the time this does, no conflict — this change's broader reframing simply supersedes its narrower one.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: the "Web UI listing of scan results" requirement is reframed from four mutually-exclusive views to an orthogonal grouping × presentation model.

## Impact

- Changed code (frontend only, no backend/API changes): `App.vue`'s view-switching state and dispatch logic; new shared `EntryTable.vue`/`EntryGrid.vue` (or similarly named) components extracted from today's `TableView.vue`/`GridView.vue`; `TreeView.vue` and `ArtistAlbumView.vue` refactored to delegate their file/track-listing portions to those shared components instead of rendering their own table markup; `store.js`'s `currentView` field replaced by two fields (grouping, presentation).
- Sequencing note: this change and `tree-and-artist-album-selection` both touch how Tree/Artist-Album render and select files — whichever lands second will need to reconcile with the other's shape (not a conflict, just an ordering note, since both are otherwise independent and safe to apply in either order). Same note applies to `grid-view-pagination-fix`, as described above.
- No backend, API, or database changes — this is purely a frontend restructuring; every existing endpoint (`GET /api/v1/library`, the tree/artist/album endpoints) is used exactly as it is today, just orchestrated differently.
