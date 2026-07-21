## 1. Baseline

- [x] 1.1 Before making any change, manually exercise and note the current behavior of every UI flow in the real browser: table view (filters, search, sort, pagination, selection modes, bulk actions), grid view (including its existing dead pagination buttons — note this as expected, not a regression to fix), folder tree (breadcrumb navigation, its own pagination), Artist/Album view (drill-down through artists → albums → tracks), the persistent player bar (play from every view, continuity across view switches/pagination), the selection banner, and the details view (candidates, cover-browse, lyrics, embedded tags, fingerprint) — this is the baseline this change must reproduce exactly — done via live side-by-side comparison against the running (old-UI) container on the same populated library, rather than a separate written log

## 2. Scaffold the Vite + Vue project

- [x] 2.1 Create `ui/package.json` and `vite.config.js` for a Vue 3 + Vite project rooted at `ui/`
- [x] 2.2 Configure a dev-server proxy from Vite to the Go server's API routes (`/api/v1/**`) so `npm run dev` works against a locally-running `go run ./cmd/server`
- [x] 2.3 Add a `.gitignore` entry for `ui/node_modules` and the Vite build output directory

## 3. Port state and shared helpers

- [x] 3.1 Create `ui/src/store.js`: a single `reactive({...})` object with the same fields as today's `ui/js/state.js` (`filterState`, `sortState`, `pageState`, `selectedPaths`, `selectionMode`, `total`, `lastEntries`, `currentView`)
- [x] 3.2 Port `ui/js/format.js` to `ui/src/format.js` unchanged (formatting/label helpers have no DOM dependency)
- [x] 3.3 Port `ui/js/api.js` to `ui/src/api.js` unchanged (one function per endpoint, no DOM dependency)

## 4. Port the player bar, filter bar, and selection banner

- [x] 4.1 Create `ui/src/components/PlayerBar.vue` from today's `ui/js/player.js` + its `index.html` markup, preserving exact behavior (no close button — that's `close-player-bar`)
- [x] 4.2 Create `ui/src/components/FilterBar.vue` from `main.js`'s filter/sort/search/page-size wiring
- [x] 4.3 Create `ui/src/components/SelectionBanner.vue` from `table.js`'s `updateSelectionBanner` and the "select all matching"/"clear selection" controls

## 5. Port each view

- [x] 5.1 Create `ui/src/components/views/TableView.vue` from `ui/js/table.js`, preserving row rendering, checkbox selection, column sort, pagination, and the play/delete/details-open row actions
- [x] 5.2 Create `ui/src/components/views/GridView.vue` from `ui/js/views/grid.js`, preserving card rendering, checkbox selection, and play button — including the existing non-functional grid pagination controls, ported as-is
- [x] 5.3 Create `ui/src/components/views/TreeView.vue` from `ui/js/views/tree.js`, preserving breadcrumb navigation, directory cards, file rows, and its own working pagination
- [x] 5.4 Create `ui/src/components/views/ArtistAlbumView.vue` from `ui/js/views/artist-album.js`, preserving the artists → albums → tracks drill-down and breadcrumb

## 6. Port the details view

- [x] 6.1 Create `ui/src/components/DetailsView.vue` from `ui/js/details.js`, preserving the candidate picker, cover-browse picker, lyrics, embedded tags, and fingerprint sections

## 7. Wire the app together

- [x] 7.1 Create `ui/src/App.vue`: view tabs switching `store.currentView`, mounting the active view component, the filter bar, selection banner, and player bar — matching today's `main.js` dispatch logic (`renderCurrentView`/`refreshCurrentView`/`refreshCurrentViewAfterFilterChange`)
- [x] 7.2 Create `ui/src/main.js`: Vue app entry, mounts `<App>`
- [x] 7.3 Port the five polling loops (scan/identify/enrich/tag/relocate) from `ui/js/polling.js` + `main.js`'s startup wiring into a composable or `App.vue`, preserving `pollJob`'s parameterized behavior

## 8. Wire the build into Docker and Go

- [x] 8.1 Add a Node build stage to `Dockerfile` (before the Go build stage) that runs `npm ci && npm run build` inside `ui/`
- [x] 8.2 Update the Go build stage to `COPY` the Vite build output from the Node stage in place of today's raw `ui/index.html`/`ui/css`/`ui/js`
- [x] 8.3 Update `ui/embed.go`'s `//go:embed` directive to embed the built output directory instead of `index.html css js`
- [x] 8.4 Delete `ui/js/` now that the Vue port is verified working end-to-end (`ui/index.html` was overwritten in place as the new Vite entry rather than kept as a separate file; `ui/css/app.css` is kept and now imported from `ui/src/main.js` rather than linked — it's still live source, not dead code)

## 9. Verification

- [x] 9.1 Run `docker compose build` (or equivalent) to confirm the new multi-stage build produces a working image with no local Node install — succeeded: Node stage builds `ui/dist`, Go stage embeds it and compiles, runtime stage assembles the final image; the user's running `music-tagger` container was left untouched (build only, no `up`)
- [x] 9.2 Confirm `go build ./...` and `go vet ./...` still pass with no local Node/npm present, confirming backend-only work needs nothing beyond Go
- [x] 9.3 Re-run every flow noted in the 1.1 baseline against the Vue app and confirm identical behavior, including the player bar's continuity across view switches and pagination — verified live against the running container's populated library (table/grid/tree/artist-album, selection persisting across view switches, filters/search/sort, details view incl. candidates/cover-browse/lyrics/embedded-tags/fingerprint); playback itself verified by serving the production `ui/dist` build directly via the compiled Go binary (see note below)
- [x] 9.4 Open browser devtools console during the full pass in 9.3 and confirm no Vue warnings, hydration/reactivity errors, or JS exceptions — none observed
- [x] 9.5 Confirm the grid view's pagination controls are exactly as broken as before (dead buttons) — this change should not have accidentally fixed or further broken them — confirmed: static, always-disabled, unwired

**Note on verifying playback via `npm run dev`:** the audio element stalled at `readyState 0` when streamed through Vite's dev-server proxy specifically (plain `fetch()`/`curl` through the same proxy, including with a `Range` header, succeeded fine — this looks like a Vite dev-proxy quirk with the browser's native media-streaming request pattern, not a bug in the port). Confirmed correct by building `ui/dist` and serving it directly from the compiled Go binary (no proxy in the loop, exactly like the real deployment) — playback reached `readyState 4`, `duration`/`currentTime` advanced normally. Since production has no proxy at all, this doesn't affect the real app; noting it so a future session doesn't re-debug it if `npm run dev` playback looks stuck.
