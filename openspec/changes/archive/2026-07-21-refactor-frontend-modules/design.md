## Context

`ui/js/app.js` (~1,200 lines) currently contains, in one file/scope: DOM element references (~35 `const`s), display constants (`STATUS_LABELS`, `DETAILS_FIELD_LABELS`, etc.), mutable module state (`selectedPaths`, `selectionMode`, `filterState`, `sortState`, `pageState`, `total`, `lastEntries`, five `*Running`/`*PollTimer` pairs), the table renderer, the details-view overlay (fields, lyrics, embedded tags, candidate picker, cover browser, fingerprint), five near-identical start/stop polling loops (scan/identify/enrich/tag/relocate), and all event-listener wiring. `ui/index.html` loads it via a single `<script src="/js/app.js">` (no `type="module"`, no bundler, Tailwind via CDN — the project has no frontend build step today and this change should not introduce one).

## Goals / Non-Goals

**Goals:**
- Split the single file into ES modules along the concern boundaries already implicit in the code (state, API calls, table rendering, details view, polling, wiring), using native browser ES module support (`<script type="module">`, `import`/`export`) — zero new tooling.
- Preserve every existing behavior exactly — this change should be invisible to the user.
- Leave a clean seam for `add-library-views-and-playback` to add new view modules (Grid, Tree, Artist-Album) and a player module without another large restructuring.

**Non-Goals:**
- Introducing a bundler, TypeScript, or a frontend framework — out of proportion to this project's size and existing minimal-tooling philosophy; native ES modules are sufficient at this scale.
- Adding or changing any user-facing behavior, including the view-switcher UI itself (only the module seam it will plug into).
- Backend changes of any kind.

## Decisions

### Module boundaries mirror the concerns already implicit in the code
```
ui/js/
  state.js       — filterState, sortState, pageState, selectedPaths, selectionMode, total, lastEntries
  api.js         — every fetch() call, one function per endpoint
  format.js      — formatDuration, formatEta, escapeHtml, STATUS_LABELS/STATUS_CLASSES, DETAILS_FIELD_LABELS, EMBEDDED_TAG_FIELD_LABELS
  table.js       — renderTable/renderRow/renderMetadataCell/renderCoverCell/renderTaggedCell/renderRelocatedCell/renderActionsCell, selection banner, pagination controls, sort indicators
  details.js     — the details overlay: fields, lyrics, embedded tags, candidate picker, cover-browse picker, fingerprint
  polling.js     — the five scan/identify/enrich/tag/relocate poll loops, generalized into one parameterized helper rather than five near-identical copies (see below)
  actions.js     — bulk action triggers (identify/enrich/tag/relocate/delete) and their button-state updates
  main.js        — DOM element lookups, event-listener wiring, initial load — the module actually loaded by `index.html`
```
Alternative considered: one module per existing top-level function — rejected as too fine-grained; the concern-level grouping above matches how a developer already has to think about this code (e.g. no one touches `details.js`'s candidate picker without also thinking about its cover-browse section, since they're both details-overlay concerns).

### The five polling loops collapse into one parameterized helper
`startScanPolling`/`startIdentifyPolling`/`startEnrichPolling`/`startTagPolling`/`startRelocatePolling` (and their matching `set*UI` functions) are structurally identical: poll a status endpoint, update a running flag + button label/disabled state, stop on `running: false`. `polling.js` exports one `pollJob({ statusUrl, onUpdate, intervalMs })`-shaped helper; `main.js` calls it five times with each job's specifics. This is the one place this refactor changes *structure* beyond a mechanical file split — justified because the duplication is exact today and any future job (e.g. a manual-search job, if ever made async) would otherwise mean copying a sixth near-identical block. Risk: subtle behavioral differences between the five existing loops (if any) must be preserved as parameters, not silently dropped — verification must diff each loop's current behavior against the unified helper's output for that job before/after.

### Cross-module state via explicit imports, not globals
`state.js`'s exported `let` bindings (`filterState`, `pageState`, etc.) are imported wherever read/written today — ES modules already give each importer a live binding to the same underlying value, so this requires no state-management library, just replacing "module-global variable" with "imported module-global variable." `main.js` remains the only module that wires DOM event listeners to state mutations + re-render calls, keeping the "what triggers a re-render" logic in one place rather than scattered across every module that happens to mutate state.

### View-switcher seam: a `currentView` state value and a documented module contract, no UI yet
`state.js` gains a `currentView` field (default, and only valid value in this change: `'table'`). `main.js`'s render-dispatch becomes a single `renderCurrentView(entries)` that switches on `currentView` — today with exactly one case. `add-library-views-and-playback` adds `views/grid.js`, `views/tree.js`, `views/artist-album.js` as siblings to `table.js` implementing the same "given entries/params, render into a container" contract `table.js` already follows, plus the tab UI to switch `currentView`. No inert tab buttons ship in this change — an unclickable or non-functional UI element is worse than no element, and this change's whole point is zero visible difference.

## Risks / Trade-offs

- **[Risk] A large mechanical refactor across the entire frontend can silently break a rarely-exercised path** (e.g. the "select all matching" filter-mode selection, or the large-selection ETA notice) → Mitigated by verification exercising every documented UI scenario from `music-library-scan`'s existing spec (table load, filter/search/sort, pagination, all four bulk actions, delete, details view including candidates/cover-browse/lyrics/embedded-tags/fingerprint, all five polling loops), not just a visual smoke test.
- **[Trade-off] Collapsing five polling loops into one helper is the one non-mechanical change bundled into an otherwise "pure refactor"** → Accepted: reviewed as low-risk since the five loops are genuinely identical in shape today (verified by inspection), and doing it now avoids a second refactor pass later once more jobs exist.

## Migration Plan

- No data/schema migration — this is a static-asset-only change. `ui/embed.go`'s existing `//go:embed index.html css js` already embeds the whole `js` directory recursively, so splitting `app.js` into multiple files under `ui/js/` needs no change to the embed directive itself.
- Rollback: revert to the single `app.js` file; no persisted state depends on the module split.
