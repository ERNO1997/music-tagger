## 1. Backend: path-list filtering

- [x] 1.1 Add `Paths []string` to `usecases.LibraryFilter` (`internal/usecases/ports.go`), documented as taking priority over other filter fields when non-empty
- [x] 1.2 Update the SQLite store's `QueryPage` (and `QueryPaths` if it shares the same filter-building code) to add a `path IN (...)` clause when `Paths` is non-empty, skipping the other filter fields entirely in that case
- [x] 1.3 Add a Go test confirming a non-empty `Paths` filter returns exactly those records (including one that wouldn't match an unrelated status/tagged/etc. filter also present in the same `LibraryFilter` value) and ignores the other fields

## 2. Backend: selection read endpoint

- [x] 2.1 Add a `POST /api/v1/library/selection` handler reusing `SelectionRequest`/`resolveSelection`'s body-parsing (or a variant that resolves `paths` directly into `LibraryFilter.Paths` instead of expanding a filter into a path list first, avoiding an unnecessary `QueryPaths` round trip when a filter is given) — accepting `sort`/`order`/`limit`/`offset` query parameters identical to `GET /api/v1/library`, returning the same `LibraryListResponse` shape
- [x] 2.2 Register the route in `internal/infrastructure/web/v1`'s route registration alongside the other library endpoints
- [x] 2.3 Add a Go test for both request shapes (`paths` and `filter`), confirming pagination/sort behave identically to `GET /api/v1/library`

## 3. Backend: relocate reports old→new paths

- [x] 3.1 Change `RelocateFile.Relocate`'s signature (`internal/usecases/relocate_file.go`) to also return the computed `newPath` alongside the existing `skipped`/`err`
- [x] 3.2 `RelocateManager` (`internal/usecases/relocate_manager.go`) accumulates a mutex-protected `[]Relocation{OldPath, NewPath}` list of every file successfully relocated in the current job, reset at the start of each `Start()` call, exposed via a new `Relocations()` method — kept separate from `Status() JobStatus` so `RelocateManager` still satisfies the `StatusChecker` interface unchanged (required by `refresh_manager.go`'s `SetRelocateStatus`)
- [x] 3.3 `RelocateStatusResponse` (`internal/infrastructure/web/v1/relocate_handler.go`) gains a `relocations: [{old_path, new_path}]` field, populated from `RelocateManager.Relocations()`
- [x] 3.4 Add a Go test confirming that after a job relocates a file, `GET /api/v1/library/relocate/status` includes its old and new path, and that a skipped or failed file does not appear in the list

## 4. Frontend: show-selected-only toggle (table + grid)

- [x] 4.1 Delete the modal-based implementation: `ui/src/components/SelectedFilesModal.vue`, `ui/src/composables/useSelectedFilesModal.js`, and the "View selected" link/wiring in `SelectionBanner.vue`/`App.vue`
- [x] 4.2 Add a "Show selected only" toggle control near the selection banner (e.g. a new small reactive state in `ui/src/store.js` or its own composable), disabled/hidden while `store.selectionMode === 'filter'`
- [x] 4.3 `ui/src/components/views/TableView.vue` and `ui/src/components/views/GridView.vue` (or their shared load path in `ui/src/composables/useLibraryList.js`) fetch via `fetchSelection(getSelectionBody(), params)` instead of `fetchLibrary(params)` whenever the toggle is enabled, using that view's own current sort/pagination state exactly as today
- [x] 4.4 Unchecking a row's checkbox continues to just call `store.selectedPaths.delete(path)` (no new removal code needed); if unchecking the last row on the current page while the toggle is enabled would leave a stranded empty page on the next fetch, reset to page 1
- [x] 4.5 Switching between table and grid view preserves `showSelectedOnly`, same as it already preserves filter/sort state

## 5. Frontend: selection follows relocation

- [x] 5.1 `ui/src/composables/useJobs.js`'s relocate poll `onUpdate` reconciles `store.selectedPaths` against `status.relocations`: for each `{old_path, new_path}`, if `old_path` is currently selected, delete it and add `new_path` (a no-op once already applied, so safe to run on every poll tick without tracking "already handled" state)

## 6. Verification

- [x] 6.1 Select files across more than one page in explicit mode, enable "show selected only" in table view, and confirm it shows exactly the selected files across as many pages as needed
- [x] 6.2 Use "select all matching" to enter filter mode and confirm the toggle is unavailable/no-op, since the current filtered listing already is the selection
- [x] 6.3 With the toggle enabled, uncheck a file and confirm the selection banner's count updates immediately and a subsequent bulk action excludes it
- [x] 6.4 With the toggle enabled, uncheck the last file on the current page and confirm the view falls back sensibly (e.g. to page 1) rather than showing a stranded empty page
- [x] 6.5 Switch between table and grid view while the toggle is enabled and confirm both reflect the selection-only listing
- [x] 6.6 Select an identified, tagged file, trigger Relocate Selected, and confirm that once the job completes the file remains selected (banner count unchanged) even though its path changed, and that a subsequent bulk action (e.g. Tag Selected) targets its new path
