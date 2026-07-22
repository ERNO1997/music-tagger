## 1. Wire grid pagination

- [ ] 1.1 Add a `paginationInfo` computed to `GridView.vue`, identical in logic to `TableView.vue`'s (derived from `store.total`/`store.pageState.offset`/`store.pageState.limit`)
- [ ] 1.2 Add `onPrevPage`/`onNextPage` handlers to `GridView.vue`, identical to `TableView.vue`'s (mutate `store.pageState.offset`, call `loadLibrary()`)
- [ ] 1.3 Bind `GridView.vue`'s existing Prev/Next buttons and info text to these, replacing the static/disabled placeholders left by `vue-adoption-shell`

## 2. Verification

- [ ] 2.1 From Grid view, page forward and backward using its own controls and confirm the displayed cards update and the info text is accurate
- [ ] 2.2 Switch to Table view mid-browse (after paging in Grid) and confirm Table shows the same page, and vice versa
- [ ] 2.3 Confirm filter/search/sort/selection are unaffected by paging in Grid
- [ ] 2.4 Confirm the Prev button is disabled on the first page and Next is disabled on the last page, matching Table view's behavior
