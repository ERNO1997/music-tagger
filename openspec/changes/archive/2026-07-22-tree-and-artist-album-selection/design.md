## Context

`TableView.vue` and `GridView.vue` both already implement the same selection pattern against the shared `store` (`store.js`): a per-row checkbox bound to `store.selectedPaths.has(entry.path)` (or forced-checked/disabled when `store.selectionMode === 'filter'`), a change handler that drops out of filter mode into explicit mode (seeding every currently-visible entry into `selectedPaths`) the first time a checkbox is toggled while in filter mode, and a header "select all" checkbox toggling every currently-visible entry. `SelectionBanner.vue` already computes its "select all N matching" / "N selected" / clear-selection UI purely from `store.selectedPaths`, `store.selectionMode`, `store.total`, and `store.lastEntries` — it has no view-specific logic, and both `TreeView.vue` and `ArtistAlbumView.vue` already set `store.lastEntries` to whatever they're currently displaying (originally so `DetailsView.vue` can look up an entry regardless of active view).

## Goals / Non-Goals

**Goals:**
- Identical selection behavior in Tree and Artist-Album to what Table/Grid already have, reusing the exact same store fields and interaction pattern.
- No changes anywhere selection is already handled generically (`SelectionBanner.vue`, bulk action buttons, backend).

**Non-Goals:**
- Selecting a whole folder or a whole album in one click (i.e., a "select all files under this directory" shortcut beyond what "select all on this page" already does for the currently-listed files) — a reasonable future enhancement, but a different, larger feature than "these two views can select files like the other two already do."
- Changing Tree's or Artist-Album's own pagination/navigation behavior.

## Decisions

### Copy `TableView.vue`'s checkbox pattern verbatim into both views
`TreeView.vue`'s file rows and `ArtistAlbumView.vue`'s track rows (at the `tracks` level) get the same three pieces `TableView.vue` already has: a `<th>`/header checkbox, a per-row `<td>` checkbox, and the same `isRowChecked`/`onRowCheckboxChange`/`onSelectAllChange` logic (parameterized over whichever local array — `files` in Tree, `tracks` in Artist-Album — stands in for `store.lastEntries` there, since both already assign that array to `store.lastEntries` for `DetailsView.vue`'s benefit). No new shared component is introduced for this — the three call sites (Table, Tree, Artist-Album; Grid uses cards instead of a table but the same underlying logic) are simple enough that extracting a shared composable would be more indirection than the duplication it removes, especially since `presentation-grouping-split` may reshape all of this again shortly.

### "Select all on this page" means "select all currently-listed rows," exactly as it already does elsewhere
In Tree, that's the current folder's current page of files (`files`, paginated). In Artist-Album, that's the current album's full track list (`tracks`, not paginated today — an album's tracks are all returned in one response). Both match how "select all" already behaves in Table (current page) and Grid (current page) — no special-casing needed, since `SelectionBanner.vue`'s "select all N matching" affordance already derives from whatever's in `store.lastEntries`, which these views already keep in sync.

## Risks / Trade-offs

- **[Trade-off] Duplicating the same ~15-line checkbox-handling block into two more files instead of extracting a composable** → Accepted: three near-identical blocks is still small enough to read directly at each call site, and a shared composable now would likely need reshaping again once `presentation-grouping-split` changes how these views are composed.
