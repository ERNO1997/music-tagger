## Context

`GridView.vue` and `TableView.vue` both render `store.lastEntries`, populated by the same shared `loadLibrary()` (in `useLibraryList.js`), which reads/writes `store.pageState.offset`/`limit`. `TableView.vue` already has working Prev/Next buttons and a computed pagination-info string bound to that shared state; `GridView.vue`'s equivalent markup exists (ported from the original HTML) but was deliberately left unwired in `vue-adoption-shell` to keep that migration behavior-preserving. This change is exactly the fix that migration's design.md flagged as coming next.

## Goals / Non-Goals

**Goals:**
- Grid view's own Prev/Next controls work, without needing to switch to Table view.
- No change to the fact that Table and Grid share one page position — switching between them mid-browse should keep showing the same page, same as it already does for filter/sort/selection.

**Non-Goals:**
- Giving Grid its own independent offset (like Tree already has) — that would make switching Table→Grid jump back to page 1 unexpectedly, a different (worse) surprise than the one being fixed.
- Any change to page size, sort, or filter handling.

## Decisions

### Copy `TableView.vue`'s pagination bindings into `GridView.vue` verbatim
`GridView.vue` gets the same `paginationInfo` computed (derived from `store.total`/`store.pageState`) and the same `onPrevPage`/`onNextPage` handlers (mutating `store.pageState.offset` then calling `loadLibrary()`) that `TableView.vue` already has. No shared composable is introduced for two near-identical blocks — same reasoning as `tree-and-artist-album-selection`'s design.md: three or four call sites this small aren't worth the indirection, especially with `presentation-grouping-split` likely to reshape pagination into something shared across whatever views/presentations that change introduces.

## Risks / Trade-offs

- **[Risk] None beyond the mechanical change itself** — the underlying state and fetch logic already work correctly (Table view proves it); this only adds the missing UI wiring in Grid.
