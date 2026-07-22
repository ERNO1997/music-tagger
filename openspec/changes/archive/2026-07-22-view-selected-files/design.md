## Context

`usecases.LibraryFilter` (`internal/usecases/ports.go`) already narrows a `QueryPage`/`QueryPaths` read by status/tagged/relocated/has-lyrics/has-cover-art/search — but has no way to restrict a read to an explicit, arbitrary set of paths. `internal/infrastructure/web/v1/selection.go`'s `resolveSelection` already accepts exactly that shape (`{paths: [...]} | {filter: {...}}`) for the identify/enrich/tag/relocate trigger endpoints, but those endpoints only ever consume the resolved path list to kick off a background job — none of them return the matching entries for display, and none of them paginate (a job processes every matching path regardless of count).

On the frontend, `store.js`'s `getSelectionBody()` already produces this exact `{paths}`/`{filter}` shape today for those same trigger endpoints — this change's UI work is mostly about calling a new endpoint with that already-existing body.

## Goals / Non-Goals

**Goals:**
- Let a user see exactly which tracked files are selected, paginated, without materializing a filter-mode selection into an explicit list.
- Let a user remove an individual file from an explicit selection while reviewing it.
- Reuse the existing selection-request shape and pagination/sort conventions rather than inventing new ones.
- Reuse the table/grid views' existing pagination/sort/checkbox machinery rather than building a separate list component.
- Keep a selected file's selection state accurate when its tracked path changes out from under it (relocation).

**Non-Goals:**
- Editing a filter-mode selection (removing one file from "all N matching") — out of scope; filter mode stays exactly what it means today (see Why).
- Any change to how bulk actions themselves resolve or process a selection.
- Bulk-removing multiple files at once — one-at-a-time removal (unchecking a row) is enough for a review-before-acting use case; a "select fewer" bulk-edit tool is a different feature.
- Introducing a stable per-file identifier (e.g. surfacing the DB's internal row id end-to-end) to replace path as the selection key — the relocate-mapping fix below solves the concrete staleness bug without that much larger, cross-cutting change.

## Decisions

### `LibraryFilter` gains an optional `Paths []string`
When non-empty, `QueryPage`/`QueryPaths` restrict to exactly those paths (an `IN` clause), ignoring the other filter fields — mirroring how `resolveSelection` already treats an explicit `paths` list as taking priority over `filter` in the trigger endpoints, for the same reason: a client-provided path list is already fully resolved, so re-applying status/tagged/etc. filters on top would just be surprising (a path the user explicitly selected disappearing from "view selected" because it no longer matches some unrelated filter). Alternative considered: a separate `QueryByPaths` store method — rejected, since it would duplicate `QueryPage`'s existing sort/pagination logic for no benefit.

### One new endpoint, reusing the existing selection body shape
`POST /api/v1/library/selection` accepts the same `SelectionRequest` (`{paths, filter}`) as `POST /api/v1/library/identify` etc., plus `sort`/`order`/`limit`/`offset` query parameters identical to `GET /api/v1/library`, and returns the same `LibraryListResponse` shape. POST (not GET) because an explicit path list can be large enough to exceed a practical URL length. Alternative considered: extending `GET /api/v1/library` itself with a `paths` query parameter — rejected for the same URL-length reason, and because every other selection-consuming endpoint in this codebase is already POST-with-body by convention.

### A toggle re-points the existing table/grid views at the selection, instead of a separate modal
`TableView.vue`/`GridView.vue` already own a load function, pagination state, sort state, and per-row checkboxes bound to `store.selectedPaths`. A "show selected only" toggle swaps just the data source that load function calls — `fetchSelection(getSelectionBody(), params)` instead of `fetchLibrary(params)` — leaving sort, page size, page navigation, and checkbox behavior completely unchanged. Unchecking a row while the toggle is on already calls `store.selectedPaths.delete(path)` today; no new removal code is needed. If unchecking the last row on the current page would leave a stranded empty page, the next fetch falls back to page 1, mirroring what the original modal design did. Alternative considered: a dedicated modal/list component (the original design) — rejected on user feedback: it duplicates pagination/sort/checkbox logic the table/grid views already have, and reviewing a selection alongside the rest of the library (rather than in a popup) is more consistent with how every other filter in this app works.

The toggle is unavailable (or a no-op) in filter mode: "all matching" selection is *already* whatever the current filtered listing shows, so toggling would fetch and display the same rows already on screen.

### Relocate status reports each successfully relocated file's old→new path
`RelocateFile.Relocate` already computes `newPath` and passes it to `store.RecordRelocation`, but discards it once recorded — nothing upstream (the job loop, the status endpoint, the frontend) ever learns the mapping. `RelocateManager` gains a mutex-protected `[]Relocation{OldPath, NewPath}` slice, reset at the start of each `Start()` call (mirroring how the shared `JobManager` already resets processed/total), appended to as each file is successfully relocated, and exposed via a new `Relocations()` method — kept as a method separate from `Status() JobStatus` so `RelocateManager` continues to satisfy the `StatusChecker` interface (`Status() JobStatus`) unchanged; `refresh_manager.go`'s `SetRelocateStatus(relocateManager)` call depends on that exact signature. `RelocateStatusResponse` gains a `relocations: [{old_path, new_path}]` field populated from it.

On the frontend, `useJobs.js`'s relocate poll `onUpdate` reconciles `store.selectedPaths` against `status.relocations` on every tick: for each `{old_path, new_path}`, if `old_path` is currently selected, delete it and add `new_path`. This is naturally idempotent — once applied, `old_path` is no longer in `selectedPaths`, so re-running the same reconciliation on a later poll (the list isn't reset until the *next* job starts) is a no-op. That idempotence also means a client that misses one or more polls mid-job still ends up correct on its next poll, since each poll's `relocations` list is a full accumulation since the job started, not just the delta since the last poll.

Alternative considered: replace path with a stable per-file identifier (the DB's existing internal `id` column, currently unused above the store layer) as the selection key everywhere — rejected as a much larger change (touches `domain.FileRecord`, `LibraryEntry`, every handler, and every frontend selection call site) for a benefit narrower than what this bug actually needs.

## Risks / Trade-offs

- **[Risk] A large explicit selection (thousands of individually-checked paths) makes for a large `paths` array in the POST body** → Accepted: bulk action trigger endpoints already accept the same shape today with no size limit; this read endpoint is no riskier than those already-shipped ones.
- **[Trade-off] Filter-mode selection can't be reviewed via the toggle** → Accepted and documented in the proposal's Why — filter mode is already fully visible as the current filtered listing, so there's nothing additional to show.
- **[Trade-off] Relocate-selection reconciliation depends on polling** → Accepted: idempotent accumulation (see above) means a missed poll or two doesn't leave the selection wrong, only briefly stale until the next poll.
