## Context

`store.pageState.limit` is a single shared value read/written from `FilterBar.vue`'s page-size `<select>` today, and consumed by `AllGroupingView.vue`'s and `FolderGroupingView.vue`'s own independent Prev/Next footers (each already computes its own `paginationInfo`/`prevDisabled`/`nextDisabled` against it). The Artist/Album grouping has no pagination at all — artists, albums, and a given album's tracks are each fetched whole in one response, per the existing `library-browsing` capability. `EntryTable.vue` and `EntryGrid.vue` are the shared row/card renderers behind all three groupings' file/track listings; neither currently renders anything when `entries` is empty — the table's `<thead>` (or the grid's empty `<div>`) is simply left with no body content. `App.vue`'s root is a single unconstrained-height div inside `<body class="bg-neutral-950 min-h-screen">` with no ancestor `overflow` clipping, so the document itself scrolls — `position: sticky` on a child works against the viewport with no extra scroll-container plumbing needed.

## Goals / Non-Goals

**Goals:**
- Free up visual space in the filters row by relocating a control that isn't a filter.
- Give every empty listing (a genuinely empty library, a narrow filter, an empty folder, an empty album) a clear, consistent "No items" indicator instead of a bare header.
- Keep bulk actions reachable while scrolling a long list, without pinning more than the title and action row.

**Non-Goals:**
- Consolidating the new generic empty-state message with each grouping's existing, more specific pagination-info text (e.g. Folder's "No files directly in this folder." / "No tracked files under this folder."). Both can coexist — the pagination-info line keeps its contextual wording, the table/grid area gains its own plain indicator where its rows would otherwise be. Unifying these into one message is a reasonable future cleanup, not required here.
- Giving the Artist/Album grouping page-size controls it doesn't have a use for — it has no pagination to control.
- Pinning the description paragraph, filters, grouping/presentation tabs, or selection banner — the request is explicit about title and actions only.

## Decisions

### Page-size selector moves to each grouping's own footer, not a shared component
`FilterBar.vue` loses its page-size `<select>` and `onPageSizeChange` handler. The same markup and handler (mutating `store.pageState.limit` then re-fetching, matching the existing pattern each footer already uses for Prev/Next) is added directly into `AllGroupingView.vue`'s and `FolderGroupingView.vue`'s own pagination footer row, next to their Prev/Next buttons. No shared "pagination controls" component is introduced for two call sites — consistent with this project's existing convention (see `tree-and-artist-album-selection`'s and `grid-view-pagination-fix`'s design docs) that a couple of small, near-identical blocks are cheaper to read at each call site than a new abstraction.

### Empty-state message lives in `EntryTable`/`EntryGrid` themselves
Both already take `entries` as a prop and render purely off it, with no fetching of their own — the natural place for "there's nothing to render" is inside them, not duplicated per grouping. `EntryTable.vue` renders a single-row `<td colspan="...">No items match the current filters.</td>` in place of the row list when `entries.length === 0`; `EntryGrid.vue` renders the same text in a plain centered block where its card grid would be. Same wording in both, since presentation shouldn't change what an empty result means.

### Sticky bar is the existing title/actions row, made sticky in place
`App.vue`'s current `<div class="flex items-start justify-between gap-4 mb-1">` (the `<h1>Music Tagger</h1>` plus the five action buttons) gets `position: sticky; top: 0`, an opaque `bg-neutral-950` (matching `body`'s own background, so scrolled content doesn't show through), a `z-10`, and a bottom border that only becomes visible once actually stuck (a cheap way to signal "this is now floating" without extra JS scroll-listeners) — achievable with a plain border that's only visually distinct against scrolled content because the sticky element now overlaps it, no `IntersectionObserver` needed for this. The description paragraph directly below stays in normal document flow.

## Risks / Trade-offs

- **[Trade-off] Two empty-state messages can appear at once** (the new generic one inside the table/grid area, and an existing specific one in a grouping's pagination-info line) → Accepted, per Non-Goals above; both convey compatible information and neither is wrong.
- **[Risk] An ancestor element unexpectedly clipping overflow would silently break `position: sticky`** → Low risk given the current structure (verified no such ancestor exists today), but worth a quick visual check after implementing, since sticky's failure mode is silent (it just scrolls away like a normal element, no error).
