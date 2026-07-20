## Context

`GET /api/v1/library` (`internal/infrastructure/web/v1/library_handler.go`) calls `TrackingStore.LoadAll`, which loads the entire `files` table into memory and returns it as a `map[string]domain.FileRecord` — used internally by `ScanLocalVolume`'s refresh diffing (compare a fresh disk walk against everything previously tracked) and, today, directly re-purposed to serve the HTTP list endpoint too. `ui/js/app.js`'s `renderTable` clears and rebuilds the entire `<tbody>` on every poll tick (every 1s while any job runs) and on every manual refresh. Four background-job managers (`IdentifyManager`, `EnrichManager`, `TagManager`, `RelocateManager`) all share the identical `Start(paths []string) error` shape and are triggered by four near-identical handlers, each parsing `{"paths": [...]}` from the request body.

The `files` table already has a surrogate `id INTEGER PRIMARY KEY` (added by the file-relocation change, when `path` stopped being the primary key) — this becomes useful here as a stable pagination tie-breaker.

## Goals / Non-Goals

**Goals:**
- `GET /api/v1/library` scales to tens of thousands of rows: a page of results, not the whole table, on every request.
- Users can filter by status/tagged/relocated, search by text, and sort by column — server-side, so the client never has to filter/sort a large in-memory array.
- Bulk actions (identify/enrich/tag/relocate) work against "everything matching this filter" at scale, without the client ever having to enumerate tens of thousands of paths into a request body.
- A `missing` tracked file can be deleted from the tracking store.

**Non-Goals:**
- Changing `ScanLocalVolume`'s internal diffing, which still needs the full table via `LoadAll` — that method and its usage are untouched by this change.
- Cursor-based pagination. Offset-based pagination with a stable tie-breaker is enough for a single-operator tool where the person browsing the table is generally the same person triggering background jobs (see Decisions).
- Rate-limit ETA notices for enrich/tag/relocate — only identify has a documented, hard, unavoidable external pace (MusicBrainz, charter §4.2); the others don't need the same treatment.
- Any change to how cover art files are stored/shared on disk — deleting a tracked row never touches `/data/covers/`, since cover art can be shared across multiple tracks on the same release.

## Decisions

### Offset pagination with an `id`-based tie-breaker, not cursors
Cursor/keyset pagination is the more "correct" choice when rows are being inserted/deleted at high concurrency and clients hold a cursor across a long session — but this is a single-operator tool where identify/enrich/tag/relocate jobs already run one at a time, one path at a time, and the person browsing the table is generally the same person who triggered the job. Plain `LIMIT`/`OFFSET` is far simpler to implement, reason about, and combine with arbitrary sort columns. Its classic failure mode — "page drift" when a row's sort-key value changes between page loads, causing a duplicate or skipped row — is mitigated by always appending `id ASC` as a secondary sort key after whatever column the user picked, so ties are broken deterministically even if the primary sort column is actively changing in the background (e.g. sorting by `status` while an identify job is resolving files out from under the current page). A user who reloads sees fresh, consistent data; residual drift within a single page-turn is a minor, self-correcting cosmetic issue, not a correctness problem, and not worth cursor complexity to eliminate.

### One `LibraryFilter` + `LibrarySort` pair, shared by the paginated list endpoint and by resolving a bulk-action selection
```go
type LibraryFilter struct {
    Status    string  // "" (any) or a domain.TrackingStatus value
    Tagged    *bool   // nil = don't filter
    Relocated *bool
    Search    string  // case-insensitive substring match against path/artist/album/title
}

type LibrarySort struct {
    By   string // allow-listed: path, status, artist, album, duration, year
    Desc bool
}
```
Two new `TrackingStore` methods:
```go
QueryPage(ctx context.Context, filter LibraryFilter, sort LibrarySort, limit, offset int) (entries []domain.FileRecord, total int, err error)
QueryPaths(ctx context.Context, filter LibraryFilter) ([]string, error)
```
`QueryPage` backs the paginated list endpoint. `QueryPaths` ignores pagination entirely and returns every matching path — used to resolve a bulk action's filter-based selection into the same `[]string` the existing managers already expect (see next decision). Reusing one filter shape for both means "the 2,700 files currently shown as matching this filter" and "the 2,700 files identify will actually process" are always defined identically — no risk of the count shown to the user and the set actually acted on silently diverging.

`Status` filtering has to account for `EffectiveStatus()` being derived, not stored directly: filtering by `missing` means `WHERE missing = 1`; filtering by any other status means `WHERE missing = 0 AND status = ?`. Sort-by-column is implemented as a Go-side allow-list `map[string]string` from public sort key to literal SQL column name — user input is never interpolated directly into the `ORDER BY` clause, since SQL parameterization doesn't cover identifiers.

### Bulk actions gain a filter-based selection mode, resolved once, in a shared handler helper — the four job managers don't change at all
`IdentifyManager.Start`, `EnrichManager.Start`, `TagManager.Start`, and `RelocateManager.Start` already share the exact signature `Start(paths []string) error`, and `IdentifyManager.Start` in particular already calls `store.LoadAll` internally just to look up each path's fingerprint — so there's no need to thread filter-awareness through any of the four managers. Instead, a single shared helper in `internal/infrastructure/web/v1/` parses each trigger endpoint's request body as *either* `{"paths": [...]}` (unchanged, for explicit/page-sized selections) *or* `{"filter": {...}}` (new: resolved via `TrackingStore.QueryPaths` into a concrete path list before calling the existing `Start(paths)`). All four handlers (`identify_handler.go`, `enrich_handler.go`, `tag_handler.go`, `relocate_handler.go`) call this same helper instead of parsing `req.Paths` directly. The background managers remain completely unaware that a filter was ever involved — as far as they're concerned, they always just received a path list.

### Fingerprint moves to its own on-demand endpoint; the details view fetches it like lyrics/embedded-tags already work
`fingerprint` is removed from `LibraryEntry` and the `files` table stops being scanned for it in the list query. A new `GET /api/v1/library/fingerprint?path=...` mirrors `lyrics_handler.go`'s shape exactly (`TrackingStore.Get` already returns the full record including `Fingerprint`, so no new store method is needed — just a thin handler). `ui/js/app.js`'s `openDetails` fetches it only when the modal opens, the same way it already conditionally fetches lyrics/embedded tags — not a new pattern, just one more instance of an existing one.

### Delete is a usecase-level gate over a plain store delete, not a store-level business rule
`SQLiteStore` gets a plain `Delete(ctx, path) error` (`DELETE FROM files WHERE path = ?`) with no built-in status check — matching this codebase's existing split (the store executes; usecases decide when it's allowed). A new small usecase, `DeleteMissingFile`, calls `store.Get(ctx, path)` first, checks `EffectiveStatus() == domain.StatusMissing`, and only then calls `store.Delete`; otherwise it returns a distinct "not missing" outcome the handler turns into `409 Conflict` rather than silently deleting (or silently no-op-ing) a row for a file that might still exist. Deleting never touches `/data/covers/` — cover art is keyed by release MBID and can be shared across tracks on the same release (existing behavior), so a row's removal has no filesystem side effect.

### Pagination controls, not infinite scroll
Infinite scroll (load more on scroll) would reintroduce exactly the "unbounded number of live DOM rows" problem pagination exists to solve at tens-of-thousands scale — a user who scrolls through a big enough library still ends up with thousands of rows in the DOM. Page size + prev/next keeps the number of live rows bounded to whatever the current page size is, at all times, regardless of library size or how far the user has browsed.

### Client-side selection gains a distinct "filter" mode alongside the existing explicit `Set`
`selectedPaths` (currently always an explicit `Set<string>`) gains a sibling mode: when the user clicks "select all N matching current filter", the client stores the *filter criteria* instead of enumerating paths, and every bulk-action trigger sends `{"filter": {...}}` instead of `{"paths": [...]}`. The UI clearly distinguishes the two ("3 selected" vs. "All 2,700 matching current filter selected") so it's never ambiguous which one a button press will act on. Switching any filter/search control while in "filter" selection mode implicitly updates what "all matching" refers to, so the displayed count and the eventual server-side resolution stay in sync (both are the same `LibraryFilter` value at the moment of submission).

## Risks / Trade-offs

- **[Risk] Offset pagination can show minor drift under concurrent background jobs** → Mitigated by the `id` tie-breaker (see Decisions); accepted as a cosmetic, self-correcting edge case rather than something worth cursor-based complexity to fully eliminate, consistent with this project's general risk tolerance for a single-operator tool.
- **[Risk] Free-text search does no relevance ranking or escaping of `LIKE` wildcards (`%`, `_`)** → A search term containing a literal `%` or `_` behaves slightly unexpectedly (matches more or less than a literal reading would suggest). Accepted as a minor, rare, non-security issue (the query is still parameterized — this is only about wildcard semantics, not injection) rather than something to add escaping logic for up front.
- **[Risk] A "select all N matching filter" bulk action can still resolve to a very large path list in memory server-side** (e.g. tens of thousands of strings) → Acceptable: this is the same order of magnitude of data already handled by `LoadAll` for every scan refresh; a slice of paths is far lighter than a slice of full records with fingerprints.
- **[Trade-off] Breaking the `GET /api/v1/library` response shape (bare array → `{total, entries}`)** → Accepted deliberately (see charter's non-goals: no external API contract exists); the one consumer (this project's own UI) is updated in the same change.

## Migration Plan

- No schema migration needed — every field used by the new filter/sort/search is already a column; the pagination tie-breaker reuses the existing `id` column.
- No rollback concern beyond reverting the code: the response shape change and query params are additive at the database level (no data changes), so an older client would simply break against the new response shape, not corrupt anything — consistent with there being exactly one client (this project's own UI), updated alongside the API in the same change.
