## 1. Domain and ports

- [ ] 1.1 Add `RawTitle`, `RawArtist`, `RawAlbum`, `RawAlbumArtist` fields to `domain.FileRecord` (`internal/domain/tracking.go`)
- [ ] 1.2 Add `RawTags{Title, Artist, Album, AlbumArtist string}` and `RawTagReader` (`ReadRawTags(ctx, path) (RawTags, error)`) to `internal/usecases/ports.go`
- [ ] 1.3 Add `HasCoverArt *bool` to `LibraryFilter` in `internal/usecases/ports.go`, parallel to `HasLyrics`

## 2. Infrastructure

- [ ] 2.1 Implement a `RawTagReader` in `internal/infrastructure/filestat/` backed by `taglib.ReadTags(...)` (reusing `TagLibTagger`'s existing tag-reading logic where practical), routed through the existing `withCorrectExtension` helper

## 3. Usecases

- [ ] 3.1 Update `ScanLocalVolume` (`internal/usecases/scan_local_volume.go`) to take a `RawTagReader` as an additional constructor dependency
- [ ] 3.2 In the per-file walk loop, for each new/changed file, call `RawTagReader.ReadRawTags` alongside the existing duration read; on success set `RawTitle`/`RawArtist`/`RawAlbum`/`RawAlbumArtist`; on failure leave raw tag fields blank without aborting the file's processing or the refresh
- [ ] 3.3 Confirm an unchanged file's raw tags are left untouched (not re-read), mirroring duration's existing behavior

## 4. Persistence

- [ ] 4.1 Add `raw_title`/`raw_artist`/`raw_album`/`raw_album_artist` columns to the `files` schema and `columnMigrations` in `internal/infrastructure/persistence/sqlite_store.go`
- [ ] 4.2 Update `BulkApply`'s upsert statement to write the raw tag fields for new/changed files
- [ ] 4.3 Update `LoadAll`, `Get`, and `QueryPage`'s SELECT/scan logic to include the new columns
- [ ] 4.4 Add a `(cover_art_path != '')` / negated clause to `buildLibraryWhere` for `HasCoverArt`, parallel to the existing `HasLyrics` clause
- [ ] 4.5 Extend the search clause in `buildLibraryWhere` to also match `raw_title`/`raw_artist`/`raw_album` via the same case-insensitive `LIKE` pattern

## 5. API

- [ ] 5.1 Add `RawTitle`/`RawArtist`/`RawAlbum`/`RawAlbumArtist` fields to `LibraryEntry` and `libraryEntryFrom` (`internal/infrastructure/web/v1/library_handler.go`), included whenever captured (not gated by identification status)
- [ ] 5.2 Parse a `has_cover_art` query parameter in `LibraryHandler.List`, same pattern as `has_lyrics`

## 6. Composition root

- [ ] 6.1 Construct the new `RawTagReader` in `cmd/server/main.go` and pass it to `usecases.NewScanLocalVolume`

## 7. Web UI

- [ ] 7.1 Add a "Cover: any/yes/no" filter `<select>` to `ui/index.html`, styled like the existing `has_lyrics` select
- [ ] 7.2 Wire the new filter in `ui/js/app.js`: add `hasCoverArt` to `filterState`, include it in `buildListParams`/`currentFilterPayload`, reset `pageState.offset` on change
- [ ] 7.3 In `renderMetadataCell` (or equivalent), when a row's status is not `identified` and a raw tag snapshot is present, render the raw title/artist/album instead of a blank dash, visually distinguished (e.g. a muted/italic style) from a resolved-metadata summary
- [ ] 7.4 In the details view (`openDetails`), show raw tag fields for unidentified files, labeled distinctly from resolved metadata (e.g. under a "From the file itself" heading)

## 8. Verification

- [ ] 8.1 Run `go build ./...` and `go vet ./...` inside Docker
- [ ] 8.2 Seed or scan a test library and confirm raw tags are populated for new files without any additional `fpcalc`/AcoustID/MusicBrainz calls
- [ ] 8.3 Confirm a changed file's raw tags are refreshed on rescan, and an unchanged file's raw tags are left untouched
- [ ] 8.4 Confirm `GET /api/v1/library?q=<raw-tag-text>` finds an unidentified file by its raw title/artist/album
- [ ] 8.5 Confirm `GET /api/v1/library?has_cover_art=false` and `?has_cover_art=true` each return only the expected subset, with `total` reflecting the filtered count
- [ ] 8.6 Confirm the web UI shows raw tag data for at least one real, currently-unidentified, badly-named file in the library, and that the cover-art filter re-fetches and re-renders correctly (manual browser check)
