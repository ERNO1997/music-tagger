## Why

Today the web UI has exactly one way to browse the library: a flat, paginated table, 25-200 rows at a time. Over a ~2,600-file library this makes several natural questions hard to answer at a glance — "what's in this folder," "what albums does this artist have," "what does this cover look like among many at once" — none of which the table view is shaped for. There's also no way to hear a track without leaving the app to open it in another player, even though the file is sitting right there on the server this UI already talks to.

## What Changes

- A cover-forward grid view is added as an alternate rendering of the same `GET /api/v1/library` data — same filters/search/sort/selection/bulk-actions, different layout (cover-first cards instead of table rows).
- A folder tree view lets a user browse `/music`'s actual on-disk directory structure, drilling into subdirectories, backed by a new hierarchical browse endpoint (grouping 2,600+ tracked files by directory client-side isn't practical across pagination).
- An Artist → Album → Track view lets a user browse by resolved artist/album (falling back to raw tag data for unidentified files, from `improve-library-visibility`, so unidentified tracks aren't simply invisible in this view), backed by new grouped-query endpoints.
- In-browser audio playback: a new streaming endpoint (supporting range requests, for seeking) and a persistent mini-player bar, so a track can be played directly from any view without leaving the page or interrupting browsing.
- A view-switcher control (Table / Grid / Tree / Artist-Album) lets the user pick which browsing mode is active; the currently-selected filter/search/selection state is preserved across a view switch where it still makes sense (e.g. a table filter still narrows the grid).

## Capabilities

### New Capabilities
- `library-browsing`: hierarchical folder-tree and Artist→Album→Track browsing over the tracked library, as alternatives to the existing flat paginated list.
- `audio-playback`: on-demand, range-request-capable streaming of a tracked file's own audio bytes, for in-browser playback.

### Modified Capabilities
- `music-library-scan`: the web UI gains a grid view (an alternate rendering of the existing list endpoint) and a view-switcher control spanning Table/Grid/Tree/Artist-Album.

## Impact

- Changed code: `internal/usecases/ports.go` (new browse/grouping query methods), `internal/infrastructure/persistence/sqlite_store.go` (new grouped/hierarchical SQL queries), `internal/infrastructure/web/v1/` (new tree, artist/album, and audio-streaming handlers), `cmd/server/main.go` (wiring), `ui/js/views/grid.js`, `ui/js/views/tree.js`, `ui/js/views/artist-album.js`, `ui/js/player.js` (new frontend modules, per `refactor-frontend-modules`'s module structure).
- No schema changes beyond what `improve-library-visibility` already adds (raw tag columns, used here as the Artist-Album view's fallback grouping key for unidentified files) — this change only adds new *read* queries over existing columns.
- Depends on `refactor-frontend-modules` landing first, so these new views/player are added as new modules in the post-refactor structure rather than bolted onto the monolith. Benefits from (but doesn't strictly require) `improve-library-visibility` landing first, since the Artist-Album view is meaningfully more useful with raw-tag fallback grouping for unidentified files; without it, unidentified files would simply have no grouping key in that view and would need an "Unidentified" catch-all bucket instead.
- No changes to identification, enrichment, tagging, or relocation — this is purely new ways to browse and play back already-tracked files.
