## Context

The only browsing surface today is `GET /api/v1/library` (flat, filtered/sorted/paginated) rendered as a table (`ui/js/table.js`, post-`refactor-frontend-modules`). Over ~2,600 files, "what's in this folder" and "what does this artist have" are both real, common questions the table can't answer without manually filtering by search text. `CoverHandler` (`internal/infrastructure/web/v1/cover_handler.go`) already establishes the pattern this change reuses for streaming: `c.SendFile(coverArtPath)`, where `coverArtPath` always comes from the tracking store (never client-supplied), so there's no path-traversal exposure. `improve-library-visibility` (assumed landed first) adds `raw_title`/`raw_artist`/`raw_album` columns, giving unidentified files a grouping key besides "no artist at all."

## Goals / Non-Goals

**Goals:**
- A grid view: same data, same filters/selection/actions, cover-forward layout.
- A folder tree view reflecting the actual on-disk directory structure under `/music`.
- An Artist→Album→Track view, using resolved metadata when available and raw tag data (from `improve-library-visibility`) as a fallback grouping key for unidentified files.
- In-browser playback of any tracked, non-missing file, with seeking.

**Non-Goals:**
- A now-playing queue, shuffle/repeat, or playlist management — a single mini-player playing one track at a time, replaced when another track's play button is clicked, is the whole scope here.
- Editing tags, moving files, or any write operation from any of the new views — browsing and playback only; existing bulk actions (identify/enrich/tag/relocate/delete) remain available from these views exactly as from the table, but no new actions are introduced.
- Waveform visualization, lyrics-synced playback highlighting, or any playback feature beyond standard transport controls.

## Decisions

### Grid view needs no new backend endpoint
Grid is purely `table.js`'s sibling `views/grid.js`, rendering the exact same `GET /api/v1/library` response (same filter/search/sort/pagination query parameters) as cover-forward cards instead of table rows. Selection, bulk actions, and the details view on click all work identically — only the per-entry rendering function differs. This is the lowest-risk, most self-contained piece of this change.

### Folder tree: group in application code over a prefix-filtered query, not recursive SQL
`GET /api/v1/library/tree?path=<prefix>` (prefix defaults to the music root) returns the immediate subdirectories under `prefix` (each with aggregate counts: total files, identified count) and the files directly at that level (as ordinary `LibraryEntry` rows, honoring the same filter/sort/pagination parameters the flat list already accepts). Implementation: a new `TrackingStore.PathsUnder(ctx, prefix string) ([]domain.FileRecord, error)` fetches every record whose path starts with `prefix` (same `LIKE 'prefix%'` mechanism `buildLibraryWhere`'s search clause already uses) — bounded in practice by the size of the library under that prefix, which for a directory-organized music collection is never large even browsing from the root (~2,600 rows worst case, a single cheap local query, not a scaling concern at this library's size). Grouping into "immediate subdirectory vs. direct file" and computing per-subdirectory counts happens in Go: for each record, take the path segment immediately following `prefix`; if more segments follow, it belongs to that subdirectory bucket (tally counts); if not, it's a direct file at this level. Alternative considered: hand-written recursive/window-function SQL to compute this in one query — rejected as unnecessary complexity for a dataset this size; a single flat fetch plus in-memory grouping is simpler to write, test, and reason about, and the cost profile doesn't demand a database-side aggregation.

### Artist→Album→Track: three new grouped-query endpoints, resolved-metadata-first with raw-tag fallback
- `GET /api/v1/library/artists` — distinct artists with track counts, grouping by `COALESCE(NULLIF(artist, ''), raw_artist)` (an identified file's resolved artist wins; an unidentified file with a raw tag falls back to that; a file with neither groups under a distinguished `(Unknown Artist)` bucket).
- `GET /api/v1/library/albums?artist=<name>` — distinct albums for that artist name (matched against the same coalesced expression), with track counts, similarly coalescing `album`/`raw_album`.
- `GET /api/v1/library/tracks?artist=<name>&album=<name>` — the actual tracks for that artist+album, as ordinary `LibraryEntry` rows, sorted by track number.
Query parameters carry artist/album names rather than REST path segments (`/artists/{artist}/albums`), consistent with this API's existing all-query-param convention (e.g. `?path=`) and avoiding double-encoding pitfalls with artist/album names containing slashes or other special characters. Alternative considered: derive this grouping purely client-side from an already-fetched flat list — rejected, since a correct grouped count (how many tracks does this artist have, across the *entire* library, not just the current page) requires a server-side aggregate query, the same reasoning `QueryPage`'s existing `total` count already reflects for the flat list.

### Audio playback: range-request streaming via the same trusted-path pattern `CoverHandler` already uses
`GET /api/v1/library/audio?path=<path>` looks up `path` in the tracking store (404 if untracked or currently `missing`) and serves it via `c.SendFile`, exactly like `CoverHandler` already does for cover images — Fiber/fasthttp's `SendFile` already handles HTTP Range requests, which `<audio>` elements rely on for seeking without downloading the whole file first. `Content-Type` is set from the file's tracked `Format` (`audio/mpeg` for mp3, `audio/flac` for flac, `audio/mp4` for m4a) rather than sniffed per-request, since format is already known and stored. No identification status gate — any tracked, non-missing file is playable, unlike cover-browsing (which needs a resolved release-group) or tagging (which needs resolved metadata); playback only needs the file's own bytes.

### A persistent mini-player, not a per-row inline player
A single `<audio controls>` element lives in a fixed bar at the bottom of the page (in `ui/js/player.js`, mounted once in `main.js`, outside the table/grid/tree/artist-album view containers), showing the currently-loaded track's title/artist (resolved, falling back to raw tags) alongside native transport controls. Clicking a "Play" affordance on any row/card/tree-file/track (in any of the four views) sets the player's `src` to that file's audio URL and calls `.play()`. Keeping the player outside the view containers means switching views, paging, or filtering never tears down or interrupts playback. Alternative considered: an inline `<audio>` per visible row — rejected, since a table/grid page can show up to 200 rows, and mounting that many audio elements (even paused) is wasteful and doesn't support "keep playing while I browse elsewhere," which a single persistent player does trivially.

### View-switcher: a tab control over `state.currentView`, reusing `refactor-frontend-modules`'s seam
A row of four tabs (Table / Grid / Tree / Artist-Album) sets `state.currentView` and calls `renderCurrentView`, extended from its single-case form in `refactor-frontend-modules` to dispatch across all four. Filter/search state is shared and continues to narrow whichever view is active (a status filter still applies when switching from Table to Grid); Tree and Artist-Album additionally apply the current filter/search within their own grouped queries, but pagination is view-specific (Table/Grid page through the flat list; Tree/Artist-Album page through directory/artist listings, each level's own concern).

## Risks / Trade-offs

- **[Risk] Streaming large FLAC files ties up a request per concurrent playback** → Accepted: this is a single-operator, local-network tool (per `project.md`'s existing scope), not a public multi-user service; Range support means typical playback only requests needed byte spans, not the whole file at once.
- **[Risk] The folder-tree endpoint's "fetch everything under this prefix, group in Go" approach could become a real cost if a single directory ever holds an unusually large number of files directly** → Accepted for this library's actual shape (organized by album, rarely more than a few dozen files per directory); if a pathological "everything dumped in one folder" case arises, the direct-files portion of the response is already paginated via the existing `limit`/`offset` mechanism, bounding the worst case.
- **[Trade-off] Artist/Album grouping by a coalesced resolved-or-raw string can create near-duplicate buckets** (e.g. "Daft Punk" resolved vs. a slightly different raw-tag spelling for an unidentified file) → Accepted as the same class of imprecision already accepted elsewhere (LRCLIB closest-duration matching, tied-recording dedup by artist/title) — a cheap, good-enough grouping, not a canonicalized one; identifying the file collapses it into the correct bucket.

## Migration Plan

- No schema changes beyond what `improve-library-visibility` already introduces.
- New capabilities (`library-browsing`, `audio-playback`) are purely additive read endpoints plus new frontend views — no change to any existing endpoint's behavior, no rollback concern beyond reverting the added code.
