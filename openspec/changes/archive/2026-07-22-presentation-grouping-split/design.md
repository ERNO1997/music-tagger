## Context

Post-`vue-adoption-shell`, the frontend has four sibling view components (`TableView.vue`, `GridView.vue`, `TreeView.vue`, `ArtistAlbumView.vue`) switched by a single `store.currentView` enum in `App.vue`. Each of `TableView.vue`/`GridView.vue` independently fetches and renders `store.lastEntries` (the flat `GET /api/v1/library` result); `TreeView.vue` and `ArtistAlbumView.vue` each own their own local fetch/state (current folder + its file page; current artist/album/tracks drill-down) and render their file/track listing as a plain table — neither has a card presentation. Table/Grid's row/card rendering and per-row selection logic are near-identical (by design, documented in `vue-adoption-shell`'s design.md); once `tree-and-artist-album-selection` lands, Tree/Artist-Album will duplicate that same selection logic a second and third time.

## Goals / Non-Goals

**Goals:**
- Any grouping's file listing can be presented as either a table or cover-forward cards.
- Row/card rendering, selection, and pagination are each implemented once and shared, not duplicated per grouping.
- Grouping and presentation are independently selectable and independently preserved when the other changes.
- No backend/API change — this is purely how the frontend orchestrates and renders already-existing endpoints.

**Non-Goals:**
- A grid/card presentation for the Artist-Album view's *artists* or *albums* levels (those are already card grids of groups, not files) — presentation only applies to an actual file/track listing.
- New sort UI for the grid presentation — sorting remains driven by table column headers when in table presentation; grid presentation continues to just reflect whatever sort is currently set, unchanged from today.
- Any change to the folder tree's or Artist-Album's own navigation/drill-down/breadcrumb behavior — only how their *file listings* are rendered and paginated changes.

## Decisions

### Two independent state fields replace `currentView`
`store.js` drops `currentView` in favor of `store.grouping` (`'all' | 'folder' | 'artist-album'`) and `store.presentation` (`'table' | 'grid'`). `App.vue`'s dispatch functions (`refreshCurrentView`, `refreshCurrentViewAfterFilterChange`, the tab-click handler) switch on `grouping` alone — presentation never affects which fetch/reload function runs, since it's purely how already-fetched entries are rendered. This mirrors `vue-adoption-shell`'s existing pattern of a small set of reactive fields driving dispatch, just with one enum split into two orthogonal ones.

### Shared `EntryTable.vue` / `EntryGrid.vue` components, used by all three groupings
The row/card rendering, per-row/card selection checkbox logic, and play/details-open handling currently living in `TableView.vue` and `GridView.vue` move into two new components taking `entries`, `total`, and pagination state/handlers as inputs (props/emits) rather than owning their own fetch — `AllGroupingView.vue` (today's flat list, using `useLibraryList.js` exactly as today), `FolderGroupingView.vue` (today's `TreeView.vue`, minus its own table markup), and `ArtistAlbumGroupingView.vue` (today's `ArtistAlbumView.vue`, minus its own table markup — only rendered at the `tracks` level) each own their fetch/drill-down state and render `<EntryTable>` or `<EntryGrid>` based on `store.presentation`. Selection logic (checkbox state, "select all on this page") moves into these two shared components too, rather than being implemented three times across the groupings — this is the concrete fix for the duplication `vue-adoption-shell` and `tree-and-artist-album-selection` each separately accepted as a short-term trade-off.

### Pagination is hoisted to each grouping, not duplicated per presentation
Each grouping owns exactly one set of pagination controls (Folder already has its own working one; All already shares `store.pageState`; Artist-Album's `tracks` level currently has none since an album's full track list isn't paginated today and this change doesn't add that) rendered once, regardless of which presentation is active — `EntryTable`/`EntryGrid` themselves render no pagination controls of their own, only the entries. This is what makes the grid-pagination bug structurally impossible to reintroduce: there's no longer a separate "grid's controls" to leave unwired, since there's only ever one set of controls per grouping.

### Presentation toggle visibility follows "is there a file listing here"
The presentation toggle (Table/Grid) is shown whenever the active grouping currently has a file/track listing to present: always for All and Folder groupings; only at the `tracks` level for Artist-Album (hidden at the `artists`/`albums` levels, which keep their existing card-grid-of-groups rendering unconditionally, unaffected by the presentation setting). Switching grouping into a state where presentation doesn't apply (e.g. Artist-Album at the `artists` level) doesn't reset the stored `presentation` value — it's simply not shown until relevant again.

## Risks / Trade-offs

- **[Risk] This is the largest single frontend restructuring since `vue-adoption-shell` itself, touching every view** → Mitigated the same way that change was: mechanical extraction of already-working logic into shared components, not a rewrite of behavior; verification should re-exercise every grouping × presentation combination plus every filter/selection/pagination scenario already covered by `vue-adoption-shell`'s and `tree-and-artist-album-selection`'s own verification passes.
- **[Trade-off] Sequencing overlap with `tree-and-artist-album-selection` and `grid-view-pagination-fix`** → Accepted, documented in the proposal's Impact — both are safe to apply in either order relative to this change; whichever lands second does a bit of reconciliation (superseding narrower code/spec text with this change's shared-component version), not a real conflict.

## Migration Plan

- No data/schema/API migration — frontend-only.
- Deploy: ships in the next frontend build, same as any other Vue-side change.
- Rollback: revert to the four-view component structure; no persisted state depends on the `grouping`/`presentation` split (view/presentation selection isn't persisted across a reload today, and doesn't need to be after this change either).
