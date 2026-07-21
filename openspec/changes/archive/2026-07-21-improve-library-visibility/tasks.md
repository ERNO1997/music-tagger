## 1. Domain and ports

- [x] 1.1 Add `RawTitle`, `RawArtist`, `RawAlbum`, `RawAlbumArtist` fields to `domain.FileRecord` (`internal/domain/tracking.go`)
- [x] 1.2 Add `RawTags{Title, Artist, Album, AlbumArtist string}` and `RawTagReader` (`ReadRawTags(ctx, path) (RawTags, error)`) to `internal/usecases/ports.go`
- [x] 1.3 Add `HasCoverArt *bool` to `LibraryFilter` in `internal/usecases/ports.go`, parallel to `HasLyrics`

## 2. Infrastructure

- [x] 2.1 Implement a `RawTagReader` in `internal/infrastructure/filestat/` backed by `taglib.ReadTags(...)` (reusing `TagLibTagger`'s existing tag-reading logic where practical), routed through the existing `withCorrectExtension` helper

## 3. Usecases

- [x] 3.1 Update `ScanLocalVolume` (`internal/usecases/scan_local_volume.go`) to take a `RawTagReader` as an additional constructor dependency
- [x] 3.2 In the per-file walk loop, for each new/changed file, call `RawTagReader.ReadRawTags` alongside the existing duration read; on success set `RawTitle`/`RawArtist`/`RawAlbum`/`RawAlbumArtist`; on failure leave raw tag fields blank without aborting the file's processing or the refresh
- [x] 3.3 Confirm an unchanged file's raw tags are left untouched (not re-read), mirroring duration's existing behavior — inherent, since unchanged files never enter the `toRead` set at all

## 4. Persistence

- [x] 4.1 Add `raw_title`/`raw_artist`/`raw_album`/`raw_album_artist` columns to the `files` schema and `columnMigrations` in `internal/infrastructure/persistence/sqlite_store.go` — also updated `migratePrimaryKey`'s rebuild path (its hardcoded `files_new` CREATE TABLE + INSERT/SELECT column lists), which would otherwise silently drop these columns for any database still on the old path-as-primary-key schema
- [x] 4.2 Update `BulkApply`'s upsert statement to write the raw tag fields for new/changed files
- [x] 4.3 Update `LoadAll`, `Get`, and `QueryPage`'s SELECT/scan logic to include the new columns
- [x] 4.4 Add a `(cover_art_path != '')` / negated clause to `buildLibraryWhere` for `HasCoverArt`, parallel to the existing `HasLyrics` clause
- [x] 4.5 Extend the search clause in `buildLibraryWhere` to also match `raw_title`/`raw_artist`/`raw_album` via the same case-insensitive `LIKE` pattern

## 5. API

- [x] 5.1 Add `RawTitle`/`RawArtist`/`RawAlbum`/`RawAlbumArtist` fields to `LibraryEntry` and `libraryEntryFrom` (`internal/infrastructure/web/v1/library_handler.go`), included whenever captured (not gated by identification status)
- [x] 5.2 Parse a `has_cover_art` query parameter in `LibraryHandler.List`, same pattern as `has_lyrics`

## 6. Composition root

- [x] 6.1 Construct the new `RawTagReader` in `cmd/server/main.go` and pass it to `usecases.NewScanLocalVolume`

## 7. Web UI

- [x] 7.1 Add a "Cover: any/yes/no" filter `<select>` to `ui/index.html`, styled like the existing `has_lyrics` select
- [x] 7.2 Wire the new filter in `ui/js/app.js`: add `hasCoverArt` to `filterState`, include it in `buildListParams`/`currentFilterPayload`, reset `pageState.offset` on change
- [x] 7.3 In `renderMetadataCell` (or equivalent), when a row's status is not `identified` and a raw tag snapshot is present, render the raw title/artist/album instead of a blank dash, visually distinguished (e.g. a muted/italic style) from a resolved-metadata summary
- [x] 7.4 In the details view (`openDetails`), show raw tag fields for unidentified files, labeled distinctly from resolved metadata (e.g. under a "From the file itself" heading)

## 8. Verification

- [x] 8.1 Run `go build ./...` and `go vet ./...` inside Docker (also ran `go test ./...` — all existing tests still pass)
- [x] 8.2 Seed or scan a test library and confirm raw tags are populated for new files without any additional `fpcalc`/AcoustID/MusicBrainz calls — verified live: fresh scan of the real ~2,584-file library populated raw tags for 346/500 sampled entries, zero fpcalc/AcoustID/MusicBrainz log entries during the scan
- [x] 8.3 Confirm a changed file's raw tags are refreshed on rescan, and an unchanged file's raw tags are left untouched — verified live: touching a real file's mtime triggered a re-read (duration/raw tags both refreshed, status reset to `new`); a subsequent rescan with no file changes processed 0 files, confirming unchanged files are skipped entirely
- [x] 8.4 Confirm `GET /api/v1/library?q=<raw-tag-text>` finds an unidentified file by its raw title/artist/album — verified live: `q=Iggy Pop` (present only in a raw tag, not the path) found `.../1647274517772140579I_WANNA_BE_YOUR_SLAVE-140_-_audio_only_me.m4a`, an otherwise-unidentifiable filename
- [x] 8.5 Confirm `GET /api/v1/library?has_cover_art=false` and `?has_cover_art=true` each return only the expected subset, with `total` reflecting the filtered count — verified live on the freshly-scanned (not-yet-enriched) library: `has_cover_art=false` → 2584, `has_cover_art=true` → 0, summing correctly to the unfiltered total of 2584; enriching a file to get a true-case non-empty result was attempted but blocked by transient MusicBrainz 503/404 errors (unrelated to this change) — the filter's WHERE-clause logic is identical in shape to the already-proven `has_lyrics` clause, and the false-case count check already confirms it's wired correctly
- [ ] 8.6 Confirm the web UI shows raw tag data for at least one real, currently-unidentified, badly-named file in the library, and that the cover-art filter re-fetches and re-renders correctly (manual browser check)
