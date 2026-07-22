## Why

Selection (and the bulk identify/enrich/tag/relocate actions built on it) only exists in the Table and Grid views today — `TreeView.vue`'s file rows and `ArtistAlbumView.vue`'s track rows render no checkboxes at all, so a user browsing by folder or by artist/album has no way to select files without switching to Table or Grid first. Selection state itself (`store.selectedPaths`/`selectionMode`) is already global and already persists across view switches (verified in `vue-adoption-shell`) — the only gap is that two of the four views never give the user a way to add to it.

## What Changes

- Add per-row checkboxes to `TreeView.vue`'s file listing and `ArtistAlbumView.vue`'s track listing (only at the leaf/track level — directory cards and artist/album cards aren't individually trackable files, so they don't get checkboxes), wired identically to `TableView.vue`'s existing checkbox logic.
- Add a "select all on this page" header checkbox to both, matching Table/Grid.
- No changes to the selection banner, bulk actions, or backend — both already work generically off `store.selectedPaths`/`selectionMode` and the currently-loaded entries, regardless of which view populated them.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `library-browsing`: the folder tree and Artist-Album views' file/track listings gain the same selection (and, by extension, bulk-action) capability the table and grid views already have.

## Impact

- Changed code: `ui/src/components/views/TreeView.vue` (file row checkboxes + header checkbox), `ui/src/components/views/ArtistAlbumView.vue` (track row checkboxes + header checkbox, at the `tracks` level only).
- No backend, API, or database changes — selection is entirely client-side state already shared across every view.
- No dependency on any other in-progress change.
