## 1. Folder tree selection

- [x] 1.1 Add a "select all" header checkbox and per-row checkboxes to `TreeView.vue`'s file table, mirroring `TableView.vue`'s `isRowChecked`/`onRowCheckboxChange`/`onSelectAllChange` logic against the `files` array
- [x] 1.2 Confirm checkbox clicks don't also trigger the row's existing click-to-open-details behavior (matching `TableView.vue`'s `@click.stop` pattern on the checkbox cell)

## 2. Artist-Album selection

- [x] 2.1 Add a "select all" header checkbox and per-row checkboxes to `ArtistAlbumView.vue`'s track table (only rendered at the `tracks` level), mirroring the same logic against the `tracks` array
- [x] 2.2 Confirm checkbox clicks don't also trigger the row's existing click-to-open-details behavior

## 3. Verification

- [x] 3.1 Select individual files in the folder tree, switch to Table/Grid/Artist-Album and back, and confirm the selection (and its checkmarks, where the same files are visible) persists throughout
- [x] 3.2 Use "select all" in a folder with files and confirm all of that folder's currently-listed files are selected
- [x] 3.3 Repeat 3.1/3.2 for the Artist-Album track listing
- [x] 3.4 Trigger a bulk action (e.g. Identify) after selecting files from the folder tree and/or Artist-Album view and confirm it operates on exactly those files
- [x] 3.5 Confirm directory cards and artist/album cards render no checkboxes at any level other than the leaf file/track listing
