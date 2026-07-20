## 1. Domain and ports

- [x] 1.1 Add `LibraryFilter` (`Status string`, `Tagged *bool`, `Relocated *bool`, `Search string`) and `LibrarySort` (`By string`, `Desc bool`) structs to `internal/usecases/ports.go`
- [x] 1.2 Add `TrackingStore.QueryPage(ctx, filter LibraryFilter, sort LibrarySort, limit, offset int) (entries []domain.FileRecord, total int, err error)` to the `TrackingStore` interface
- [x] 1.3 Add `TrackingStore.QueryPaths(ctx, filter LibraryFilter) ([]string, error)` to the `TrackingStore` interface — ignores pagination, returns every matching path, for resolving a bulk action's filter-based selection
- [x] 1.4 Add `TrackingStore.Delete(ctx, path string) error` to the `TrackingStore` interface — a plain, ungated row delete; the "only when missing" rule is enforced by a usecase, not here (see task 3.3)

## 2. Persistence

- [x] 2.1 Implement `SQLiteStore.QueryPage` in `internal/infrastructure/persistence/sqlite_store.go`: build a `WHERE` clause from `LibraryFilter` (`status` filter translates to `missing = 1` for the `missing` case, else `missing = 0 AND status = ?`; `tagged`/`relocated` map directly to their boolean columns when non-nil; `Search` becomes a parameterized `LIKE '%'||?||'%' COLLATE NOCASE` across `path`, `artist`, `album`, `title`, OR'd together), run a `SELECT COUNT(*)` with the same `WHERE` for `total`, then a `SELECT ... WHERE ... ORDER BY <mapped column> {ASC|DESC}, id ASC LIMIT ? OFFSET ?` for the page itself
- [x] 2.2 Implement the sort-column allow-list as a Go `map[string]string` (public sort key → literal SQL column name: `path`→`path`, `status`→`status`, `artist`→`artist`, `album`→`album`, `duration`→`duration_seconds`, `year`→`year`), defaulting to `path ASC` for an unrecognized or empty sort key — never interpolate the raw query parameter into the SQL string
- [x] 2.3 Implement `SQLiteStore.QueryPaths`: same `WHERE`-clause construction as `QueryPage` (reuse a shared private helper), `SELECT path FROM files WHERE ...` with no `LIMIT`/`OFFSET`
- [x] 2.4 Implement `SQLiteStore.Delete(ctx, path) error` — `DELETE FROM files WHERE path = ?`
- [x] 2.5 Verify the `id ASC` tie-break: with several rows sharing the same sort-column value, confirm repeated `QueryPage` calls return the same order

## 3. Usecases

- [x] 3.1 No changes needed to `IdentifyManager`/`EnrichManager`/`TagManager`/`RelocateManager` — all four already take `Start(paths []string) error`; filter resolution happens entirely in the web layer (task 4) before calling `Start`
- [x] 3.2 Add a `DeleteMissingFile` usecase in `internal/usecases/` (new file, e.g. `delete_missing_file.go`): loads the record via `TrackingStore.Get`, returns a distinguishable "not found" outcome if absent, a distinguishable "not missing" outcome if `EffectiveStatus() != domain.StatusMissing`, and otherwise calls `TrackingStore.Delete`
- [x] 3.3 Confirm `DeleteMissingFile` never touches `internal/infrastructure/covers.Store` or any file on disk — deletion is a pure tracking-store row removal

## 4. API

- [x] 4.1 Add a shared helper (e.g. `internal/infrastructure/web/v1/selection.go`) that parses a trigger request body shaped as `{"paths": [...]}` OR `{"filter": {"status": "...", "tagged": ..., "relocated": ..., "q": "..."}}`, resolving a filter via `TrackingStore.QueryPaths` into a concrete `[]string`, and returning `400` if neither `paths` nor `filter` yields anything to act on
- [x] 4.2 Update `IdentifyHandler.Trigger`, `EnrichHandler.Trigger`, `TagHandler.Trigger`, `RelocateHandler.Trigger` (in their respective `*_handler.go` files) to use the shared helper from 4.1 instead of parsing `req.Paths` directly, then call `Start(paths)` exactly as before — no change to any manager
- [x] 4.3 Rewrite `LibraryHandler.List` (`internal/infrastructure/web/v1/library_handler.go`): parse `status`, `tagged`, `relocated`, `q`, `sort`, `order`, `limit`, `offset` query parameters into a `LibraryFilter`/`LibrarySort` (with sane defaults: `limit` capped and defaulted, e.g. 50; `order` defaults to `asc`), call `store.QueryPage`, and respond with `{"total": N, "entries": [...]}` instead of a bare array
- [x] 4.4 Remove `Fingerprint` from the `LibraryEntry` DTO
- [x] 4.5 Add `internal/infrastructure/web/v1/fingerprint_handler.go` with a `Get` handler for `GET /api/v1/library/fingerprint?path=...`, mirroring `lyrics_handler.go`'s shape exactly (uses the existing `TrackingStore.Get`, no new store method needed), returning `404` for an untracked path
- [x] 4.6 Add a delete handler (e.g. `internal/infrastructure/web/v1/delete_handler.go`) wrapping `DeleteMissingFile`: `200`/`204` on success, `409 Conflict` when not missing, `404 Not Found` when untracked
- [x] 4.7 Register the new fingerprint and delete routes, and confirm the four existing trigger routes are unaffected by the handler changes, in `internal/infrastructure/web/v1/router.go`

## 5. Composition root

- [x] 5.1 Wire `DeleteMissingFile` and its handler into `cmd/server/main.go`; the fingerprint handler needs only the existing `store`, same as `LyricsHandler`

## 6. Web UI

- [x] 6.1 Add filter controls to `ui/index.html`/`ui/js/app.js`: a status dropdown (All/New/Identified/Not Found/Missing), tagged/relocated toggle filters, and a search input (debounced) — all re-triggering `GET /api/v1/library` with the corresponding query parameters
- [x] 6.2 Add sortable column headers (click to set `sort`/toggle `order`) for the allow-listed columns
- [x] 6.3 Add pagination controls: page size selector and prev/next, driving `limit`/`offset`; update `loadLibrary()` to read the new `{total, entries}` response shape instead of a bare array
- [x] 6.4 Remove the Fingerprint column from the table; keep it only in the details view, fetched on open via `GET /api/v1/library/fingerprint` (mirroring `loadLyrics`/`loadEmbeddedTags`'s existing pattern)
- [x] 6.5 Extend the selection model: alongside the existing `selectedPaths` `Set`, add a "filter selection" mode (store the active `LibraryFilter` instead of enumerating paths) triggered by a "select all N matching" control; visibly distinguish the two modes in the UI (e.g. "3 selected" vs "All 2,700 matching selected")
- [x] 6.6 Update `triggerIdentify`/`triggerEnrich`/`triggerTag`/`triggerRelocate` to submit `{"filter": {...}}` when filter-selection mode is active, `{"paths": [...]}` otherwise
- [x] 6.7 Before starting Identify over a selection above a small threshold (e.g. 20), show an estimated completion time computed as `selectionSize` seconds (MusicBrainz's 1 req/sec) converted to a human-readable duration — computed client-side from the already-known count, no extra request
- [x] 6.8 Add a delete action (e.g. a small icon) to rows with status `missing`, with a confirmation prompt before calling the delete endpoint and removing the row from the table on success
- [x] 6.9 Confirm switching any filter/search/sort control while in filter-selection mode keeps the displayed "all matching" count and the eventual request in sync (both read from the same current filter state)

## 7. Verification

- [x] 7.1 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 7.2 Seed a scratch database with a meaningful number of synthetic rows (hundreds to low thousands is enough to exercise pagination/sorting without needing tens of thousands of real files) and confirm: status/tagged/relocated filters narrow results correctly; search matches across path/artist/album/title; each allow-listed sort column orders correctly in both directions with stable ties; limit/offset paginate correctly and `total` reflects the filtered (not paginated) count
- [x] 7.3 Confirm an unrecognized `sort` value falls back to the documented default rather than erroring or being interpolated into SQL
- [x] 7.4 Confirm a filter-based bulk-action request (e.g. `{"filter": {"status": "new"}}`) resolves to and processes the correct path set, matching what `GET /api/v1/library` reports for the same filter at the same moment
- [x] 7.5 Confirm the fingerprint endpoint returns the correct value and 404s for an untracked path, and that it is no longer present in `GET /api/v1/library`'s response
- [x] 7.6 Confirm deleting a `missing` file removes its row and leaves any shared cover art file on disk untouched (e.g. another track on the same release keeps its cover art); confirm deleting a non-missing file returns `409` and changes nothing; confirm deleting an untracked path returns `404`
- [ ] 7.7 Confirm the web UI end-to-end against a real (or realistically-sized synthetic) library: filter/search/sort/pagination controls work, "select all N matching" behaves distinctly from explicit selection and both correctly drive bulk actions, the Identify ETA notice appears for a large selection, and the delete action works only on `missing` rows
- [ ] 7.8 Rebuild and run via `docker compose up --build` against the user's real music library volume as a final sanity check (small library, but confirms nothing regressed for the existing real-world flow)
