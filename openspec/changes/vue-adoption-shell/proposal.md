## Why

The frontend has outgrown hand-rolled DOM code. `main.js` (272 lines) manually wires every filter/tab/poll listener; `table.js`, `views/grid.js`, `views/tree.js`, and `views/artist-album.js` each re-implement near-identical row/card rendering, checkbox selection, and play-button wiring against the shared `state` object, with no framework to express "same data, different presentation" declaratively. Three upcoming changes — a closable player bar, selection on the folder-tree and Artist/Album views, and splitting presentation (table/grid) from grouping (all/folder/artist-album) — all add UI surface of exactly this repetitive kind. Doing them in Vue instead of more vanilla DOM wiring means building each once as a component instead of copy-pasting another render/checkbox/selection block, and avoids building the same restructuring twice (once now, once during a later migration).

## What Changes

- Introduce a Vue 3 + Vite build pipeline for the frontend, replacing the current zero-build, native-ES-module setup (`ui/index.html` + `ui/js/*.js` loaded directly, embedded as-is via `ui/embed.go`).
- Add a Node build stage to the existing multi-stage `Dockerfile` that runs `vite build` and produces the static bundle `ui/embed.go` embeds — mirroring how `fpcalc`/`ffmpeg` are already confined to the runtime image rather than required locally. Local frontend development needs Node only when someone is actively working on the UI (`npm run dev` for Vite HMR against the running Go API); backend-only work still needs nothing beyond Go.
- Port every existing view — table, grid, folder tree, Artist/Album — into Vue components with **identical** behavior to today: same filters, same selection semantics (`selectedPaths`/`selectionMode`, including "select all matching filter" mode), same pagination behavior (including today's existing grid-pagination bug — intentionally not fixed here; see `grid-view-pagination-fix`), same play-button/details-view-open behavior.
- Port the persistent player bar, selection banner, and filter/search/sort controls as Vue components, again with no behavior change (the player bar's missing close button is intentionally not added here; see `close-player-bar`).
- This is a like-for-like structural port, not a redesign — verification is specifically about confirming nothing changed from the user's perspective.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
(none — this is an internal, behavior-preserving refactor; no capability's requirements change)

## Impact

- Changed code: all of `ui/js/*.js` and `ui/index.html` are replaced by a Vue 3 + Vite project (component boundaries decided in `design.md`); `ui/embed.go` embeds the built output instead of raw source files.
- New build dependency: Node/npm + Vite, confined to a Docker build stage (and optional local use for frontend development only) — no change to how the compiled Go binary is deployed or run.
- No backend changes, no API changes, no schema changes.
- No new user-facing behavior. Every subsequent UI proposal (`close-player-bar`, `view-selected-files`, `tree-and-artist-album-selection`, `grid-view-pagination-fix`, `presentation-grouping-split`) depends on this change landing first, so their components are built directly in Vue rather than in vanilla JS and re-ported later.
