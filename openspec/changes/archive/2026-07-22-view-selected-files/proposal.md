## Why

Selection is already tracked precisely — an explicit set of paths that can span many pages, or a `filter` mode meaning "every file currently matching the filter, however many that is" (see `state.selectedPaths`/`selectionMode` in `ui/src/store.js`) — but the UI only ever shows a *count* (`SelectionBanner.vue`: "N selected." / "All N matching file(s) selected."). Before running a bulk action (identify/enrich/tag/relocate) against dozens or hundreds of files accumulated across several pages, there's no way to actually see which files are selected, or to back out one that got checked by mistake.

## What Changes

- Add a "Show selected only" toggle to the table and grid views (not a separate modal) that, while enabled, re-points that view's existing pagination/sort/checkbox machinery at exactly the currently-selected files instead of the current filter.
- In explicit selection mode, unchecking a row while the toggle is enabled removes it from the selection — the existing per-row checkbox is the removal mechanism; no separate remove control is needed.
- In filter mode ("all matching" selected), the toggle is unavailable (or a no-op), since the current filtered listing already *is* the selection — toggling would show the same rows already visible.
- New backend support: since selection can be an arbitrary, possibly cross-page set of paths, listing it requires a new endpoint that resolves either an explicit path list or a filter to a paginated, sorted result — extending the same `{paths, filter}` selection shape the identify/enrich/tag/relocate trigger endpoints already accept.
- Bug fix bundled into this change: relocating a selected file changes its tracked path server-side, but the client's selection tracks the old path, so a relocated file silently falls out of the selection. The relocate status endpoint now also reports each successfully relocated file's old→new path, and the UI reconciles the current selection against that mapping on every status poll, so a relocated file stays selected under its new path.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: adds a `POST /api/v1/library/selection` endpoint that returns a paginated, sorted page of tracked entries matching an explicit path list or a filter (the same request shape `POST /api/v1/library/identify` etc. already accept); adds a "show selected only" toggle to the table and grid views; and extends relocate status reporting with an old→new path mapping so the UI can keep a relocated file's selection state consistent across its path change.

## Impact

- Backend: `internal/usecases/ports.go`'s `LibraryFilter` gains an optional `Paths []string` (restrict to exactly these paths when non-empty); the SQLite store's `QueryPage` implementation honors it; a new `POST /api/v1/library/selection` handler in `internal/infrastructure/web/v1/`. `RelocateFile.Relocate` (`internal/usecases/relocate_file.go`) returns the computed new path to its caller instead of discarding it after recording it in the store. `RelocateManager` (`internal/usecases/relocate_manager.go`) accumulates each job's successful old→new path pairs (reset at the start of each job) and exposes them via a new method. `RelocateStatusResponse`/`GET /api/v1/library/relocate/status` (`internal/infrastructure/web/v1/relocate_handler.go`) includes them as `relocations: [{old_path, new_path}]`.
- Frontend: no separate modal — `ui/src/components/views/TableView.vue` and `GridView.vue` gain a "show selected only" toggle that swaps their fetch between `fetchLibrary` and the existing `fetchSelection`, reusing each view's own sort/pagination/checkbox state. The relocate poll handler in `ui/src/composables/useJobs.js` reconciles `store.selectedPaths` against each poll's `relocations` list.
- No changes to existing endpoints' behavior beyond the additive `relocations` field on relocate status; `resolveSelection` (identify/enrich/tag/relocate triggers) in `internal/infrastructure/web/v1/selection.go` is unchanged — this only adds a way to *read* the same selection shape those actions already accept, and a way to keep that selection state accurate across a relocation.
- No dependency on any other in-progress change.
