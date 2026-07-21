## Context

`ScanLocalVolume.Refresh` (`internal/usecases/scan_local_volume.go`) already performs a cheap, local, no-decode TagLib read per new/changed file to get duration (`DurationReader.ReadDuration`, added in `speed-up-library-scan`). Separately, `TagLibTagger.ReadEmbeddedTags` (`internal/infrastructure/filestat/taglib_tagger.go`) already reads a file's actual embedded Title/Artist/Album/AlbumArtist/TrackNumber/DiscNumber/Year/HasLyrics/HasCoverArt directly from disk — but it's only ever called on-demand, gated behind `entry.tagged` in the UI (`ui/js/app.js`'s `openDetails`), for verifying a tag write succeeded. For an unidentified file, none of this is surfaced anywhere: `GET /api/v1/library` returns only `path`/`format`/`duration_seconds`/`status`/`error` for a `new` file, so a badly-named file is indistinguishable from any other until identified.

## Goals / Non-Goals

**Goals:**
- Show a file's own embedded title/artist/album, if present, in the library list and details view — even before (or instead of, if identification never resolves) AcoustID/MusicBrainz identification.
- Make this raw tag data searchable and, eventually, useful as a grouping key for hierarchical views (Artist→Album→Track, planned in a later change) even for unidentified files.
- Add `has_cover_art` filtering, closing the last gap in the `tagged`/`relocated`/`has_lyrics` filter family.

**Non-Goals:**
- Using raw tags as an identification signal (e.g., feeding them into AcoustID/MusicBrainz lookup) — the project's Acoustic-First Identification Rule deliberately never derives identity from filenames or embedded tags; this change is purely about *display/search*, not resolution.
- Re-reading raw tags for unchanged files on every scan — same "only new/changed files get re-read" discipline the duration read already follows.
- Editing embedded tags from this raw-tag display — read-only, same as the existing details-view embedded-tags section.

## Decisions

### A new, separate `RawTagReader` port, not an extension of `DurationReader` or reuse of `Tagger.ReadEmbeddedTags` wholesale
`ScanLocalVolume` takes `DurationReader` today (one TagLib `ReadProperties` call per file). Raw tags require a second, distinct TagLib call (`ReadTags`). Rather than bloating `DurationReader`'s single-purpose interface, a new port is added:
```go
type RawTags struct { Title, Artist, Album, AlbumArtist string }
type RawTagReader interface {
    ReadRawTags(ctx context.Context, path string) (RawTags, error)
}
```
`ScanLocalVolume` gains this as a second constructor dependency, called once per new/changed file alongside (not instead of) the existing duration read — two cheap local TagLib calls per file, still nothing like the `fpcalc` decode cost `speed-up-library-scan` already eliminated. Implemented by a new `TagLibRawTagReader` (or a method added to the existing `TagLibTagger`, which already has the `ReadTags`-calling logic in `ReadEmbeddedTags` to share) in `internal/infrastructure/filestat/`, routed through the existing `withCorrectExtension` helper exactly like duration and full embedded-tag reads already are. Alternative considered: have `ScanLocalVolume` take the full `Tagger` interface and call `ReadEmbeddedTags` — rejected, since that also reads `Properties.Images` (`HasCoverArt`) and lyrics-presence, neither needed here (cover/lyrics presence for scan-time purposes is unrelated to a fresh disk walk and would be redundant/stale by the time enrichment runs), and pulls in `Tag`-writing capability the scanner has no business depending on.

### Raw tags are stored as a separate field group, never merged into resolved metadata columns
`domain.FileRecord` gains `RawTitle`, `RawArtist`, `RawAlbum`, `RawAlbumArtist` — populated by scan, left untouched by `RecordIdentification`/`RecordAmbiguous`/`ResolveAmbiguous` (which only ever touch resolved-metadata columns). This keeps "what the file's own tags say" and "what AcoustID/MusicBrainz resolved" permanently distinguishable — critical since a file's raw tags are frequently wrong (mislabeled downloads, generic "Track 1" titles) in ways resolved metadata, once present, is more trustworthy about. `libraryEntryFrom` (`internal/infrastructure/web/v1/library_handler.go`) includes raw fields in the JSON response unconditionally (they're informational, not gated by status), but the UI only *renders* them when resolved metadata is absent (an `identified` row already shows its trustworthy resolved artist/album/title; showing raw tags alongside would be redundant clutter).

### `RawArtist`/`RawAlbum` join the existing search clause; a changed file's raw tags are refreshed, cleared on read failure
`buildLibraryWhere`'s existing search clause (`internal/infrastructure/persistence/sqlite_store.go`) is extended to also match `raw_title`/`raw_artist`/`raw_album` via the same case-insensitive `LIKE`, so `q=slave` finds `1647274517772140579I_WANNA_BE_YOUR_SLAVE...m4a` by its embedded title even pre-identification. A changed file (per existing size/mtime change detection) has its raw tags re-read exactly like duration is re-read — the old snapshot could be stale (or belong to entirely different audio) after a content change. If the raw-tag read itself fails independently of the duration read (rare — same file, same TagLib session in practice, but not guaranteed atomic), raw tag fields are simply left blank for that pass rather than aborting the scan or duplicating `FingerprintError`'s error-surfacing mechanism — a blank raw tag is already the correct "nothing to show" UI state, so there's no need for a third failure-reason field alongside `FingerprintError`.

### `has_cover_art` filter: identical shape to `has_lyrics`
`LibraryFilter` gains `HasCoverArt *bool`; `buildLibraryWhere` gains `(cover_art_path != '')` / negated, exactly parallel to the `has_lyrics` clause added in `improve-match-quality`. No new column needed — `cover_art_path` already exists and is already exposed as `has_cover_art` in `libraryEntryFrom`'s response; this only adds it as a query-side filter dimension.

## Risks / Trade-offs

- **[Risk] A file's raw tags can be wrong or misleading** (e.g. a downloader that writes a generic "Track 01" title, or tags copied from an unrelated file) → Accepted and expected: raw tags are explicitly labeled/styled as "as embedded in the file" rather than authoritative, and are never used for anything beyond display/search — the exact same trust posture the existing embedded-tags details-view section already has.
- **[Trade-off] One more TagLib call per new/changed file during scan** → Accepted: still a local, no-decode read; `speed-up-library-scan`'s whole point was eliminating the `fpcalc` decode cost, and this stays well within the cheap-TagLib-read budget that change established (confirmed cheap in practice: the existing `ReadEmbeddedTags` used for details-view/tag-verification already does this same `ReadTags` call on demand with no reported latency concern).

## Migration Plan

- Schema change: add `raw_title`/`raw_artist`/`raw_album`/`raw_album_artist` columns to `files` (idempotent `ALTER TABLE`, following the existing `columnMigrations` pattern in `sqlite_store.go`). Existing rows simply have these blank until their next scan pass naturally repopulates them (no need to force a full rescan — the fields degrade gracefully to "no raw tag data yet," same as any newly-added optional field).
- No rollback concern: reverting the code leaves the columns in place, harmlessly populated or blank.
