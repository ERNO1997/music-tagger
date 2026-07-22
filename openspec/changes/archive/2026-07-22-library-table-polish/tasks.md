## 1. Path column: drop the /music prefix

- [x] 1.1 Add a `displayPath(path)` helper to `entryDisplay.js` that strips a leading `/music/` (or bare `/music`) from the given path, leaving everything else unchanged
- [x] 1.2 Use `displayPath(entry.path)` for the Path cell's rendered text in `EntryTable.vue`, keeping `entry.path` itself (row key, selection, details lookup, play/delete calls) untouched
- [x] 1.3 Add a `title` attribute on the Path cell showing the full, unstripped `entry.path`

## 2. Tagged column → metadata-completeness icon

- [x] 2.1 Remove the Tagged `<th>`/`<td>` (header and per-row cell) from `EntryTable.vue`
- [x] 2.2 Add a computed per-row check (identified rows only) for whether `artist`, `album`, `title`, and `track_number` are all present
- [x] 2.3 Render a check icon next to the Artist/Album/Title/Track cell's text when all four are present, or a warning icon whose `title` lists exactly which are missing when one or more are absent, for identified rows only (no icon for non-identified rows)

## 3. Relocated column → canonical-path icon on the Path cell

- [x] 3.1 Remove the Relocated `<th>`/`<td>` (header, per-row cell, and its color-based styling) from `EntryTable.vue`
- [x] 3.2 Add a check icon (no color) next to the Path cell when `entry.relocated` is true
- [x] 3.3 Add a warning icon (no color) next to the Path cell, with `entry.relocate_error` as its tooltip, when a relocation attempt has failed
- [x] 3.4 Show neither icon when an entry has neither been relocated/detected-at-destination nor had a relocation attempt fail

## 4. Details view: truncate the fingerprint

- [x] 4.1 Add a local `fingerprintExpanded` ref in `DetailsView.vue`, reset to `false` whenever `loadDetails` runs
- [x] 4.2 Render the Fingerprint field specially (outside the generic `fields` loop, or via a per-row conditional keyed on `f.label === 'Fingerprint'`): truncated value plus a "See more…" toggle that expands it, and "Show less" to collapse it back

## 5. Play action → Actions column

- [x] 5.1 Remove the standalone Play `<th>`/`<td>` from `EntryTable.vue`
- [x] 5.2 Move the play button into the Actions cell, ahead of the Delete button, preserving each button's existing visibility rule (play hidden for `missing` rows, delete shown only for `missing` rows) and the `@click.stop` on both

## 6. Verification

- [x] 6.1 In the All grouping, Table presentation, confirm paths render without their `/music` prefix and the full path still appears on hover
- [x] 6.2 Confirm an identified row with all four metadata fields shows a check icon, and one missing a field shows a warning icon whose tooltip names the missing field(s)
- [x] 6.3 Confirm a non-identified row shows neither metadata-completeness icon
- [x] 6.4 Confirm a relocated (or passively-detected-at-destination) row shows an uncolored check icon by its path, and a row with a failed relocation shows an uncolored warning icon with the failure reason on hover
- [x] 6.5 Confirm a row that's neither relocated nor relocation-failed shows no path icon
- [x] 6.6 Open the details view for a file with a stored fingerprint, confirm it's truncated with "See more…", and confirm expanding/collapsing works
- [x] 6.7 Confirm Play and Delete both appear in the single Actions column and each still only shows for the correct row status
- [x] 6.8 Repeat 6.1–6.7 (where applicable) for the Folder grouping and the Artist-Album grouping's track-level table, since `EntryTable.vue` is shared across all three
