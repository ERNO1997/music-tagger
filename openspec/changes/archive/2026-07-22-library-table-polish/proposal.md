## Why

The library listing's Table presentation and Details view have accumulated some rough edges now that the underlying data (resolved metadata, the passive relocation check from `background-library-analysis`, fingerprints) is richer than when these UI pieces were first built: every path is shown in full even though it's always rooted at the same mounted volume, the Tagged/Relocated columns show a bare boolean where the row's own metadata already tells a more useful story, and the Details view's fingerprint field renders a long opaque string at full length. None of this requires new data — it's UI-only cleanup of how already-available fields are presented.

## What Changes

- Drop the leading `/music` path prefix from the Path column's displayed value (display-only; the underlying path used for selection/actions/details lookup is unchanged). Applies everywhere `EntryTable.vue` renders a Path column (All, Folder, and Artist-Album's track level), since it's a shared component.
- Remove the standalone "Tagged" column. **BREAKING** (UI-visible behavior change): its signal is replaced by a small completeness icon on the existing "Artist / Album / Title / Track" column — a check when an identified row's artist, album, title, and track number are all present, a warning icon (with a tooltip listing exactly which fields are missing) otherwise. This is a different signal than before: it reflects resolved-metadata completeness, not whether the file's on-disk tags were successfully written. The on-disk write outcome remains inspectable via the Details view's existing embedded-tags section.
- Remove the standalone "Relocated" column and its color-coded status. **BREAKING** (UI-visible behavior change): replaced by a check icon inline in the Path cell when the file's current path already matches its computed canonical destination (reusing the `relocated` flag the already-shipped `background-library-analysis` capability sets passively via `detectRelocated`, without requiring an explicit relocate action), and a warning icon with tooltip when a relocate attempt has failed (`relocate_error`).
- In the Details view, truncate the Fingerprint field's long value with a "See more…" toggle to expand it, instead of always rendering it in full.
- Move the Play action from its own dedicated column into the existing Actions column, alongside the Delete action, and remove the standalone Play column/header.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: the "Web UI listing of scan results" requirement's language around a "tagged indicator" and a "relocated indicator" is updated to reflect that these are now surfaced as a metadata-completeness icon (on the artist/album/title/track summary) and a canonical-path icon (on the path itself), respectively, rather than as their own dedicated columns.

## Impact

- Changed code: `ui/src/components/EntryTable.vue` (path display, tagged-column removal + completeness icon, relocated-column removal + path icon, Play/Actions column merge), `ui/src/components/DetailsView.vue` (fingerprint truncation).
- No backend, API, or database changes — every field these changes need (`path`, `artist`/`album`/`title`/`track_number`, `relocated`, `relocate_error`, `fingerprint`) is already present in the existing API responses.
- No dependency on any other in-progress change.
