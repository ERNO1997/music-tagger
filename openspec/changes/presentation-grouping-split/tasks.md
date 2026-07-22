## 1. State model

- [ ] 1.1 Replace `store.currentView` with `store.grouping` (`'all' | 'folder' | 'artist-album'`) and `store.presentation` (`'table' | 'grid'`) in `ui/src/store.js`
- [ ] 1.2 Update `App.vue`'s dispatch functions (`refreshCurrentView`, `refreshCurrentViewAfterFilterChange`, tab-click handling) to switch on `grouping` only

## 2. Shared presentation components

- [ ] 2.1 Extract `TableView.vue`'s row rendering, checkbox selection, and sort-header logic into `ui/src/components/EntryTable.vue`, taking `entries`/`total` and emitting selection/sort events rather than owning a fetch
- [ ] 2.2 Extract `GridView.vue`'s card rendering and checkbox selection logic into `ui/src/components/EntryGrid.vue`, same input/emit shape as `EntryTable.vue`
- [ ] 2.3 Confirm both components' selection behavior (explicit/filter mode, "select all on this page", drop-out-of-filter-mode-on-manual-toggle) matches today's `TableView.vue`/`GridView.vue` exactly

## 3. Grouping views

- [ ] 3.1 Create `AllGroupingView.vue`: today's flat-list fetch (`useLibraryList.js`) plus its own pagination controls, rendering `<EntryTable>` or `<EntryGrid>` based on `store.presentation`
- [ ] 3.2 Refactor `TreeView.vue` into `FolderGroupingView.vue`: keep breadcrumb/directory-card/drill-down logic and its own pagination controls, replace its inline file table with `<EntryTable>`/`<EntryGrid>` based on `store.presentation`
- [ ] 3.3 Refactor `ArtistAlbumView.vue` into `ArtistAlbumGroupingView.vue`: keep artist/album card-grid levels unconditionally (no presentation toggle shown there), replace the `tracks` level's inline table with `<EntryTable>`/`<EntryGrid>` based on `store.presentation`

## 4. Top-level UI

- [ ] 4.1 Replace `App.vue`'s four-tab row with two independent toggle groups: grouping (All / Folder / Artist-Album) and presentation (Table / Grid)
- [ ] 4.2 Hide the presentation toggle when the active grouping/level has no file listing to present (Artist-Album's `artists`/`albums` levels)

## 5. Verification

- [ ] 5.1 For each grouping (All, Folder, Artist-Album-at-tracks-level), toggle between Table and Grid presentation and confirm entries render correctly in both, with the same filter/search/sort/selection/pagination position preserved across the toggle
- [ ] 5.2 Confirm the presentation toggle is hidden at Artist-Album's artists/albums levels and reappears once drilled into an album's tracks
- [ ] 5.3 Confirm switching grouping preserves the current presentation choice, and switching presentation preserves the current grouping and its drill-down position
- [ ] 5.4 Confirm selection persists across every grouping × presentation combination, and a bulk action triggered after selecting across several combinations operates on exactly the selected files
- [ ] 5.5 Confirm pagination works identically in both presentations for All and Folder groupings (this subsumes and re-verifies `grid-view-pagination-fix`'s scenario in the new model)
- [ ] 5.6 Re-run the full baseline from `vue-adoption-shell`'s verification (filters, bulk actions, details view sections, playback) against the new structure to confirm nothing regressed
