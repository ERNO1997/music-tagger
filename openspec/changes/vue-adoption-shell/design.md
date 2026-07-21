## Context

The frontend today is `ui/index.html` plus `ui/js/*.js` — native ES modules (`state.js`, `api.js`, `format.js`, `table.js`, `details.js`, `polling.js`, `actions.js`, `player.js`, `views/{grid,tree,artist-album}.js`, `main.js`), loaded directly by the browser via `<script type="module">`, no bundler, Tailwind via CDN. `ui/embed.go` embeds `index.html`, `css`, and `js` verbatim (`//go:embed index.html css js`) into the Go binary. The Dockerfile is already a two-stage build (`golang:1.25-bookworm` builder → `debian:bookworm-slim` runtime); there is no Node/npm anywhere in the repo today. Per standing project preference, only Go should be required to run the app locally — `fpcalc`/`ffmpeg` are confined to the runtime image, never a local dependency — and this change follows the same pattern for the frontend toolchain.

Every view module (`table.js`, `views/grid.js`, `views/tree.js`, `views/artist-album.js`) already shares the same `state` object (`state.js`) for filters/sort/pagination/selection, and each independently re-implements row/card rendering, checkbox wiring, and play-button wiring against it. This proposal replaces that pattern with Vue components sharing the same state shape, without changing what that state contains or how the API is called.

## Goals / Non-Goals

**Goals:**
- Stand up a Vue 3 + Vite build producing the static bundle `ui/embed.go` embeds, built inside a new Docker build stage so the runtime image and local Go-only workflow are unaffected.
- Port every existing view, the player bar, selection banner, and filter/sort/search controls into Vue components with **identical** observable behavior — this change should be invisible to the user.
- Leave clean component boundaries for the next four proposals (`close-player-bar`, `view-selected-files`, `tree-and-artist-album-selection`, `presentation-grouping-split`) to extend without another structural upheaval.

**Non-Goals:**
- Fixing the grid-pagination bug, adding a player close button, adding tree/artist-album selection, or splitting presentation from grouping — each is a separate, already-scoped proposal that lands after this one.
- Adopting TypeScript, a state-management library (Pinia/Vuex), or SSR — the app is a single-page admin dashboard over one API; a plain `reactive()`/`ref()` store mirroring today's `state.js` is sufficient at this scale.
- Any backend, API, or database change.

## Decisions

### Vue 3 + Vite, single-page app, no router
The four "views" (table/grid/tree/artist-album) are tabs within one page, not distinct routes — there's no URL-per-view today and no requirement to add one. A single Vite entry mounts one root Vue app; `vue-router` is not introduced. Alternative considered: `vue-router` with per-view routes — rejected as unneeded complexity; nothing today depends on deep-linking into a specific view/filter state, and it can be added later without disrupting component boundaries if that need arises.

### Component boundaries mirror the existing module split
```
ui/src/
  main.js                 — Vue app entry, mounts <App>
  App.vue                 — top-level layout: filter bar, view tabs, active view, player bar, selection banner
  store.js                — reactive state (filterState, sortState, pageState, selectedPaths, selectionMode, total, lastEntries, currentView) — same shape as today's state.js, exported as one reactive object so every component reads/writes the same live state without prop-drilling
  api.js                  — unchanged: one function per endpoint, ported as-is
  format.js               — unchanged: formatting/label helpers, ported as-is
  components/
    FilterBar.vue          — status/tagged/relocated/has_lyrics/has_cover_art/search/page-size controls
    SelectionBanner.vue     — "N selected" / "select all matching" / clear, reused by every view
    PlayerBar.vue           — persistent audio player
    views/
      TableView.vue         — table rendering + row selection + sort + pagination (today's table.js)
      GridView.vue          — card rendering + selection (today's views/grid.js), including its existing dead pagination controls — ported as-is, not fixed
      TreeView.vue           — breadcrumb + directory cards + file rows + its own pagination (today's views/tree.js)
      ArtistAlbumView.vue    — breadcrumb + artist/album/track drill-down (today's views/artist-album.js)
  DetailsView.vue (or details/*.vue) — the existing details overlay (candidate picker, cover browser, lyrics, embedded tags, fingerprint) — ported as a modal/dialog component
```
This is a near-mechanical mapping from existing files to components, not a redesign — kept intentionally boring so behavior-preservation is easy to verify file-by-file against the current implementation.

### State: one reactive store object, not Pinia
`store.js` exports a single `reactive({...})` object with the same fields `state.js` has today (`filterState`, `sortState`, `pageState`, `selectedPaths`, `selectionMode`, `total`, `lastEntries`, `currentView`), imported directly by any component that needs it — Vue's reactivity makes every importer's template auto-update on mutation, same ergonomics as today's shared mutable object but with automatic re-rendering instead of manually calling `render()` after each mutation. Alternative considered: Pinia — rejected as unneeded ceremony for one flat, ungrouped piece of state with no cross-store concerns; can be introduced later without touching component internals if the state model grows real structure (e.g. once `presentation-grouping-split` lands).

### Build: Vite, output embedded by Go, Node confined to Docker
Add an `ui/package.json` + `vite.config.js`. The Dockerfile gains a `node:XX-bookworm` build stage before the Go build stage: it runs `npm ci && npm run build`, producing `ui/dist/`; the Go build stage `COPY`s `ui/dist/` in place of today's raw `index.html`/`css`/`js`, and `ui/embed.go`'s `//go:embed` directive is updated to embed the built output directory. Local development: `npm run dev` (Vite dev server with a proxy to the Go API's port) is the frontend dev loop; running/building the Go server itself still requires nothing beyond Go. This mirrors the existing `fpcalc`/`ffmpeg` pattern — a tool needed to produce/run the full artifact, confined to Docker, not a bare-metal local requirement for backend work.

## Risks / Trade-offs

- **[Risk] A large mechanical port across the entire frontend can silently change behavior in a rarely-exercised path** (e.g. "select all matching" filter-mode selection surviving a view switch, or the tree view's breadcrumb-driven pagination reset) → Mitigated by verifying every documented scenario from `library-browsing` and `audio-playback` specs, not just a visual smoke test: filter/search/sort/paginate in table and grid, folder-tree navigation and its own pagination, Artist/Album drill-down, all bulk actions, details view (candidates/cover-browse/lyrics/embedded-tags/fingerprint), and playback continuity across view switches.
- **[Risk] Introducing Node/npm as a build dependency, even Docker-confined, is a bigger toolchain change than the prior vanilla-JS refactor** → Accepted, since it's the explicit purpose of this change; scoped by keeping Node confined to a build stage (never in the runtime image) and by this being the only change in this proposal — no new behavior ships bundled with the toolchain change, so if something goes wrong it's easy to attribute to the port itself rather than tangled with a feature change.
- **[Trade-off] Porting the grid view's existing pagination bug as-is (rather than fixing it here)** → Accepted: fixing it during a mechanical port risks conflating "did the port preserve behavior" with "did the fix work," and it's already a separately scoped proposal (`grid-view-pagination-fix`) that lands right after this one.

## Migration Plan

- No data/schema migration — static frontend assets only. `ui/embed.go`'s embed directive changes from `index.html css js` to whatever `vite build`'s output directory is (e.g. `dist`), pointed at the Go build stage's copy of it.
- Deploy: rebuild the Docker image (new Node build stage runs automatically as part of `docker build`); no separate migration step.
- Rollback: revert to the prior commit — the old `ui/index.html` + `ui/js/*.js` + `ui/embed.go` still work standalone since nothing about the Go server or API changes.

## Open Questions

- Exact Vite output directory name and whether `ui/css/app.css` (hand-written Tailwind-style utility CSS) is ported as-is into the Vue build or reorganized alongside the component split — left to implementation, since either is behavior-invisible.
