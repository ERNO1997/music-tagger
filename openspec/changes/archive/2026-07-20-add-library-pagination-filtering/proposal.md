## Why

`GET /api/v1/library` currently returns every tracked file in one JSON array — including the full raw Chromaprint fingerprint string per row (several KB each) — and the web UI rebuilds its entire `<table>` from scratch on every 1-second status-poll tick while any job is running. There is no search, no status filter, no column sort, and no pagination: it's one flat, ever-growing table. This is fine for the dozens of files tested so far, but the user intends to point this at a library with tens of thousands of tracked files, at which point the list payload, the per-poll full DOM rebuild, and the complete absence of any way to narrow down "what do I need to look at" all become real problems. Separately, a tracked file whose status is `missing` (confirmed gone from disk) has no way to be removed from the tracking store — it sits there forever even though it can never be acted on again.

## What Changes

- `GET /api/v1/library` gains query parameters — `status`, `tagged`, `relocated`, `q` (free-text search across path/artist/album/title), `sort` + `order` (an allow-listed set of sortable columns), and `limit`/`offset` — and its response shape changes from a bare array to `{"total": N, "entries": [...]}` so the UI can render pagination controls. There is no external API contract to preserve (single-operator tool, per the charter's non-goals), so this is a clean break rather than an additive/versioned change.
- The `fingerprint` field is dropped from `GET /api/v1/library` entirely (it was never used in the table, only shown in the details view) and replaced with `GET /api/v1/library/fingerprint?path=...`, an on-demand endpoint fetched only when the details view opens — mirroring the existing lyrics/cover/embedded-tags on-demand pattern already established in this codebase.
- The web UI gains: a status filter, a search box, sortable column headers, and page-based pagination controls (page size + prev/next — not infinite scroll, which would reintroduce the same "too many DOM nodes" problem pagination is meant to solve), all driving the new query parameters. The Fingerprint column is removed from the table (it was never human-meaningful there) and shown only in the details view.
- Bulk actions (Identify/Enrich/Tag/Relocate Selected) gain a second selection mode alongside the existing explicit-path-list one: "select all N matching the current filter", where the request carries the filter criteria itself rather than an explicit list of (potentially tens of thousands of) paths, and the server re-resolves the matching path set at execution time. A shared request-parsing helper handles this uniformly across all four trigger endpoints so none of the four background-job managers need to change.
- Before starting an Identify job over a large selection, the UI shows an ETA reflecting MusicBrainz's hard 1 req/sec rate limit (charter §4.2) — computed client-side from the already-known match count, no extra API call needed. The other three actions (enrich, tag, relocate) don't have the same documented hard external rate limit, so they don't get the same treatment.
- A tracked file with status `missing` can now be deleted from the tracking store — gated so it's only ever allowed when the file is confirmed missing, never for a file that might still exist, to avoid orphaning a real file's tracking state. Surfaced in the UI as a delete action on `missing` rows, with a confirmation prompt first.

## Capabilities

### New Capabilities
(none — everything below is a modification to the existing `music-library-scan` and `file-tracking-store` capabilities)

### Modified Capabilities
- `music-library-scan`: `GET /api/v1/library` gains filtering/sorting/pagination query parameters and a new `{total, entries}` response envelope; `fingerprint` moves to a new on-demand endpoint; the web UI gains filter/search/sort/pagination controls and a delete action for missing files; the identify/enrich/tag/relocate trigger endpoints gain a filter-based selection mode alongside the existing explicit-paths one.
- `file-tracking-store`: new store-level query capability — filtered, sorted, paginated reads, and resolving a filter to a bare path list — distinct from the existing full-table `LoadAll` (which stays as-is for scan's internal diffing). Deletion's gating/API contract lives entirely under `music-library-scan`, consistent with how existing single-field reads (`GetCoverArtPath`, `GetLyrics`) were never given their own `file-tracking-store` requirements either.

## Impact

- New code: query/filter/sort/pagination logic in `internal/infrastructure/persistence/sqlite_store.go`; a new `internal/infrastructure/web/v1/fingerprint_handler.go`; a shared paths-or-filter request-parsing helper used by the four existing trigger handlers; a new delete handler and usecase.
- Breaking (internal only): `GET /api/v1/library`'s response shape changes from a bare array to an object with `total`/`entries`, and drops the `fingerprint` field. The web UI (the only consumer) is updated in the same change.
- Schema: no new columns needed (all filterable/sortable fields already exist); queries will use the existing surrogate `id` column (added by the file-relocation change) as a stable pagination tie-breaker alongside whatever column the user sorts by.
- UI: `ui/index.html`/`ui/js/app.js` gain filter/search/sort controls, pagination, a delete action, and a large-selection ETA notice; the client's selection model gains a second "filter-based" mode alongside the existing explicit `Set` of paths.
- No changes to the scan/refresh pipeline itself, or to how `LoadAll` is used internally for change detection — this change is additive query capability on top of the same underlying table.
