## Context

`EntryTable.vue` renders every entry's `path`, `format`, `duration_seconds`, `status`, a metadata summary (resolved or raw-tag fallback), and boolean-ish indicator columns for lyrics/tagged/relocated, plus Play and Actions columns, driven entirely by fields already present in `GET /api/v1/library`'s (and the tree/artist/album/track endpoints') response shape — no endpoint changes are needed for any of the five items here.

Two backend facts materially shape this design, both confirmed by reading the code rather than assumed:
- `internal/usecases/tag_file.go`'s `Tag` skips (rather than errors) any path whose status isn't `identified` — tagging, and therefore the old boolean "tagged" concept, only ever applies to identified rows. The new completeness icon should follow the same scope.
- `internal/usecases/analysis_manager.go`'s `detectRelocated` (part of the already-shipped `background-library-analysis` capability) sets `rec.Relocated = true` passively, on every analysis pass, whenever an identified+tagged file's current path already equals its computed canonical destination (via the same `ComputeDestination` the on-demand relocate action uses) — independent of whether the user ever triggered "Relocate Selected". So today's `entry.relocated` already means "currently sitting at its canonical path," not merely "was explicitly relocated through this app." This change's Path-column check icon can reuse it as-is.

## Goals / Non-Goals

**Goals:**
- Make the Table presentation's Path column readable without the constant `/music` prefix, without changing what value is actually used for selection/actions/details lookup.
- Fold the Tagged and Relocated columns' signal into the columns a user is already looking at (metadata summary, path), rather than dedicating a column to each.
- Make the Details view's fingerprint field skimmable instead of always expanding to its full length.
- Consolidate row-level actions (Play, Delete) into one Actions column.

**Non-Goals:**
- Changing what data the backend returns, or adding any new field to the API — every value used here (`path`, `artist`/`album`/`title`/`track_number`, `relocated`, `relocate_error`, `fingerprint`) already exists in current responses.
- Restoring a literal "was this file's on-disk tags written" indicator in the table — that remains a Details-view concern (its existing embedded-tags section already answers it for a single file on demand). Bulk-visibility of tag-write success/failure is deliberately traded away in favor of the more immediately actionable completeness signal.
- Changing `EntryGrid.vue` (the card presentation) — none of these five items were requested for Grid, and Grid doesn't have a Path or Tagged/Relocated column to begin with.

## Decisions

### Path prefix stripping is a pure display transform, not a data change
Add a small helper (e.g. `displayPath(entry.path)`, colocated with the other display helpers in `entryDisplay.js`) that strips a literal leading `/music/` (or bare `/music` for a root-level file) from the string shown in the Path cell, while every existing use of `entry.path` (row key, selection, details lookup, delete/play calls) keeps using the untouched value. The full path is kept available via the cell's `title` attribute so it's still visible on hover — useful since two files with the same filename in different folders would otherwise look identical once the shared prefix is stripped. The prefix is hardcoded as `/music` rather than derived from a config value or the tree endpoint's resolved root, since the mounted volume path is fixed by `docker-compose.yml`'s `MUSIC_DIR: /music` for this single-tenant local tool — introducing a dynamic root would be solving a problem this deployment doesn't have.

### Tagged column → completeness icon on the metadata column
`isRowChecked`-adjacent logic in `EntryTable.vue` gains a small computed per row: for `status === 'identified'` rows, check `artist`, `album`, `title`, `track_number` are all truthy. All four present → a green check icon next to the metadata text; one or more missing → a warning icon whose `title` lists exactly which of the four are absent (e.g. "Missing: track number"). Non-identified rows (raw-tag fallback or otherwise) render neither icon, matching the old Tagged column's scope (tagging itself only ever applied to identified rows, per `tag_file.go`). The dedicated Tagged `<th>`/`<td>` and its `statusClass`-style coloring are removed entirely.

### Relocated column → check icon inline in the Path cell
The dedicated Relocated `<th>`/`<td>` is removed. In its place, the Path cell gets a small trailing icon: a check when `entry.relocated` is true (meaning, per `detectRelocated`, the file is confirmed sitting at its canonical destination — whether that happened via an explicit relocate action or was passively detected), or a warning icon with `entry.relocate_error` as its tooltip when a relocate attempt is on record as failed. Neither icon renders when a file is simply not yet identified/tagged/relocated (today's "—" case) — absence of the icon communicates "not applicable yet," consistent with how the old column already behaved for those rows. No color is used for either icon (per the proposal's explicit request) — check and warning are distinguished by icon shape alone, consistent with how the new Tagged-replacement icon is styled.

### Fingerprint truncation is special-cased by field label, not a generic `fields` array change
`DetailsView.vue`'s generic `fields` array/`<dl>` loop (`DETAILS_FIELD_LABELS`-driven, `internal` line ~319) stays generic for every other field. Only the Fingerprint row — pushed in separately via `loadFingerprint` — gets its own small local `ref` (e.g. `fingerprintExpanded`) and conditional rendering: truncate the displayed value (e.g. to ~40 characters) with a trailing "See more…" button that flips `fingerprintExpanded` to show the full string, collapsing back via "Show less" a second click. This is scoped to the one field long enough to need it rather than adding truncation behavior to the shared loop, which would require every other (short) field to opt out.

### Play action moves into the Actions column
`EntryTable.vue`'s standalone Play `<th>`/`<td>` (with its `@click.stop` play button) is removed; the play button moves into the existing Actions cell, rendered before the Delete button (or in place of it when a row's status is `missing`, since Play is already hidden for `missing` rows and Delete is already hidden for non-`missing` rows — the two remain mutually exclusive per row, just sharing one column now instead of two).

## Risks / Trade-offs

- **[Trade-off] Losing at-a-glance "was this file's on-disk tags written" visibility in the table** → Accepted per the proposal: the Details view's embedded-tags section already answers this per file on demand, and the new completeness icon is a more actionable bulk signal (it tells you what to fix, not just pass/fail).
- **[Risk] Hardcoding `/music` as the stripped prefix** → Low risk for this single-tenant, Docker-Compose-pinned deployment (`MUSIC_DIR: /music` is not user-configurable per-request); if the mount path ever became configurable, this would need revisiting, but that's not the case today.
- **[Trade-off] Reusing `relocated`/`relocate_error` for the Path-column icon rather than adding a distinct "at canonical path" field** → Accepted: `detectRelocated`'s existing semantics already are "at canonical path" for identified+tagged files, so introducing a separate field would be duplicating data the backend already computes.
