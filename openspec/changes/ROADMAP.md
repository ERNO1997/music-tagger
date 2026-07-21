# UI/library improvement roadmap

Not an OpenSpec change itself — a running note of the batch of improvements
discussed on 2026-07-21, so a later session can pick up where this one left
off without re-deriving the investigation. Delete this file once all items
below have their own archived change.

## Status

| # | Change name                        | Status                          |
|---|-------------------------------------|----------------------------------|
| 1 | `vue-adoption-shell`                | **Implemented** (all 27 tasks done, verified live in-browser + `docker compose build`; not yet archived — run `/opsx:archive` when ready) |
| 2 | `close-player-bar`                  | Not started — blocked on #1 |
| 3 | `view-selected-files`               | Not started — blocked on #1 |
| 4 | `tree-and-artist-album-selection`   | Not started — blocked on #1 |
| 5 | `grid-view-pagination-fix`          | Not started — blocked on #1 |
| 6 | `background-library-analysis`       | Not started — no dependency, backend-only |
| 7 | `presentation-grouping-split`       | Not started — blocked on #1 |

Agreed process: draft one OpenSpec change at a time via the `openspec-propose`
skill, review each before moving to the next. `vue-adoption-shell` was picked
to go first so items 2–5 and 7 land as Vue components instead of more vanilla
DOM code that would need porting later.

## Background: how each item was investigated

Grounded by reading the actual code before proposing anything, not just the
user's descriptions:

- **Player not closable**: confirmed in `ui/js/player.js` — `playTrack()` only
  ever removes the `hidden` class from `#player-bar`; nothing re-adds it or
  stops playback.
- **"See selected"**: doesn't exist. `state.js`/`table.js` track selection
  precisely (`selectedPaths` explicit set, or `selectionMode: 'filter'`
  meaning "everything matching"), but the UI only ever shows a *count*
  banner, never the actual list.
- **Selection on tree/artist-album**: confirmed absent — `views/tree.js` and
  `views/artist-album.js` render plain rows/cards with no checkboxes; only
  `table.js` and `views/grid.js` wire into `state.selectedPaths`.
- **Background analysis**: confirmed gap — fingerprinting is lazy (computed
  only when identify runs), and identify/enrich/tag/relocate are all
  strictly on-demand, user-triggered jobs (see `music-library-scan` spec).
  Nothing runs automatically after a scan.
- **Relocated-by-convention**: confirmed gap — `relocated` is only ever set
  by the explicit relocate action actually moving a file (see
  `file-relocation` spec's "already at destination" scenario, which only
  fires *when relocation is triggered*, not passively on scan).
- **Grid pagination bug — root cause found**: `index.html` has real
  `#grid-pagination-info`/`#grid-prev-page`/`#grid-next-page` elements, but
  `ui/js/views/grid.js` never wires listeners to them. Grid silently relies
  on `table.js`'s prev/next buttons mutating the shared
  `state.pageState.offset` — but those buttons live inside `#view-table`,
  hidden while on the Grid tab. Hence: switch to Table to page, switch back
  to Grid to see it.
- **Split presentation from grouping**: today it's 4 parallel,
  independently-implemented views (table/grid/tree/artist-album) instead of
  an orthogonal *grouping* (All / Folder / Artist-Album) × *presentation*
  (Table / Grid) model. Tree and Artist-Album currently only know how to
  render as tables.
- **Answered without a code change**: Artist/Album view already handles
  missing metadata correctly — falls back to a file's raw embedded
  artist/album tags when unresolved, and groups under a distinguished
  "unknown artist"/"unknown album" bucket when there's neither (confirmed in
  both `library-browsing` spec and `sqlite_store.go`).

## Decisions already made (don't re-ask)

- **Framework**: Vue 3 + Vite. User knows Vue. Project currently has zero
  Node/npm toolchain (no `package.json` anywhere, `ui/embed.go` embeds raw
  static files, Go-only Dockerfile). Per the user's standing preference that
  only Go should run locally (fpcalc/ffmpeg are Docker-only), the Vue build
  is confined to a new Node stage in the existing multi-stage `Dockerfile`
  — local frontend dev needs Node only when actively touching the UI;
  backend-only work still needs nothing beyond Go.
- **No router, no Pinia**: single-page app, one flat reactive store mirroring
  today's `state.js` shape — same reasoning as the design.md in
  `vue-adoption-shell`.
- **`background-library-analysis` scope** (user's own words, more specific
  than the original ask): fingerprint automatically after scan; check for
  pre-existing lyrics/cover art by reading a file's *own embedded tags
  directly* (ground truth from the file itself, independent of the app's DB
  — matters because on a first run of the project there's no DB data yet,
  but files may already carry embedded cover art/lyrics from being tagged by
  another tool); and a canonical-location check — if an identified file's
  current path already matches the computed canonical destination
  (`{Artist}/{year - Album}/{track - Title}`), mark it `relocated`
  automatically without requiring the explicit relocate action. This merges
  what were originally two separate ideas (background analysis + passive
  relocation detection) into one capability.
- **`grid-view-pagination-fix`**: intentionally NOT fixed inside
  `vue-adoption-shell` — that change ports the grid view's pagination bug
  as-is, so behavior-preservation isn't muddied with a real fix. Also worth
  checking, once `presentation-grouping-split` lands, whether this fix is
  still a separate change or folds into that refactor (grid pagination may
  cease to exist as a distinct concept once presentation/grouping are
  orthogonal axes).

## Next step

Resume with: draft `close-player-bar` via the `openspec-propose` skill (small,
isolated — a dismiss/stop control on the persistent player bar), once
`vue-adoption-shell` itself has been reviewed/applied. Update the status
table above as each change is drafted/archived.
