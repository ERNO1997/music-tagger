## 1. Grid view (no new backend endpoint)

- [ ] 1.1 Add `ui/js/views/grid.js`: cover-forward card rendering of the same `GET /api/v1/library` response `table.js` already renders, reusing the existing selection/action/details-view wiring
- [ ] 1.2 Add the grid rendering function to `renderCurrentView`'s dispatch (extended from `refactor-frontend-modules`'s single-case form)

## 2. Ports and persistence — folder tree

- [x] 2.1 Add `TrackingStore.PathsUnder(ctx, prefix string) ([]domain.FileRecord, error)` to `internal/usecases/ports.go`
- [x] 2.2 Implement `SQLiteStore.PathsUnder` (`internal/infrastructure/persistence/sqlite_store.go`): `SELECT ... WHERE path LIKE 'prefix%'`, reusing the existing filter/search WHERE-clause building where applicable
- [x] 2.3 Add a `TreeBrowse` usecase (`internal/usecases/tree_browse.go`): given a prefix and the existing `LibraryFilter`/`LibrarySort`/limit/offset, fetch matching records under that prefix, group into immediate-subdirectory buckets (name, total count, identified count) vs. direct files at this level, paginate the direct-files list using the existing `QueryPage`-equivalent mechanism

## 3. Ports and persistence — Artist/Album browsing

- [x] 3.1 Implement grouped queries for distinct artists, distinct albums for an artist, and tracks for an artist+album, each coalescing resolved metadata with the raw tag fallback (`COALESCE(NULLIF(artist, ''), raw_artist)` and equivalent for album), falling back to a distinguished "(Unknown Artist)"/"(Unknown Album)" bucket when both are empty
- [x] 3.2 Add corresponding `TrackingStore` methods (e.g. `ListArtists`, `ListAlbums(artist string)`, `ListTracks(artist, album string)`) honoring the existing `LibraryFilter` dimensions

## 4. API — browsing endpoints

- [x] 4.1 Add a `GET /api/v1/library/tree` handler accepting `path` plus the existing filter/sort/limit/offset query parameters, returning `{"directories": [...], "files": {"total": N, "entries": [...]}}`
- [x] 4.2 Add `GET /api/v1/library/artists`, `GET /api/v1/library/albums?artist=<name>`, `GET /api/v1/library/tracks?artist=<name>&album=<name>` handlers, each honoring the existing filter query parameters
- [x] 4.3 Register the new routes in `internal/infrastructure/web/v1/router.go` and wire the new usecases in `cmd/server/main.go`

## 5. Audio streaming

- [x] 5.1 Add a `GET /api/v1/library/audio` handler: look up `path` in the tracking store (404 if untracked or `missing`), set `Content-Type` from the tracked `Format`, serve via `c.SendFile` (same trusted-path pattern as `CoverHandler`)
- [x] 5.2 Register the route and confirm Range-request behavior (seeking) works via Fiber/fasthttp's built-in `SendFile` support

## 6. Frontend — tree and artist-album views, player

- [x] 6.1 Add `ui/js/views/tree.js`: breadcrumb navigation, subdirectory list (with counts), direct-files list at the current level (reusing `table.js`'s row-rendering helpers where practical)
- [x] 6.2 Add `ui/js/views/artist-album.js`: three-level drill-down (artists → albums → tracks), reusing existing row-rendering helpers for the final tracks level
- [x] 6.3 Add `ui/js/player.js`: a persistent `<audio controls>` bar mounted once in `main.js`, outside all view containers; exposes a `playTrack(entry)` function setting `src` to the audio endpoint and showing the track's resolved (or raw tag) title/artist
- [x] 6.4 Add a "Play" affordance to table rows, grid cards, tree file rows, and artist-album track rows, each calling `playTrack`
- [x] 6.5 Add the four-tab view-switcher control (Table / Grid / Tree / Artist-Album) to `ui/index.html`/`main.js`, wired to `state.currentView`, preserving the active filter/search across a switch

## 7. Verification

- [x] 7.1 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 7.2 Confirm grid view renders the same data as table view and supports selection/bulk-actions/details identically
- [x] 7.3 Confirm `GET /api/v1/library/tree` at the root and at a nested real subdirectory returns correct subdirectory counts and direct files, and that filters narrow both
- [x] 7.4 Confirm the Artist/Album endpoints group an unidentified file under its raw-tag artist/album, and group a file with neither resolved nor raw metadata under the "(Unknown Artist)"/"(Unknown Album)" bucket
- [x] 7.5 Confirm `GET /api/v1/library/audio` streams a real file with the correct `Content-Type`, supports a Range request (partial content), and 404s for an untracked or `missing` path
- [x] 7.6 Confirm playback survives switching views and paginating, and that starting a second track stops the first (manual browser check)
- [x] 7.7 Confirm the view-switcher preserves the active filter/search when switching between all four views (manual browser check)
