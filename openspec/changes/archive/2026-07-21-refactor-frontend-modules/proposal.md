## Why

`ui/js/app.js` is a single ~1,200-line file with module-global DOM references, module-global mutable state (`filterState`, `pageState`, `selectedPaths`, five separate polling loops), and every feature — table rendering, details view, candidate picker, cover browser, filter controls, bulk actions, polling — implemented as top-level functions in one flat scope. Four planned features (`improve-library-visibility`'s raw-tag display, `add-manual-search-identify`'s search control, and `add-library-views-and-playback`'s grid/tree/artist-album views and audio player) all need to add UI surface to this same file. Continuing to grow it monolithically makes each addition harder to reason about and increases the chance one feature's change accidentally breaks another's already-working behavior — already a real risk given how much cross-cutting state (`lastEntries`, `selectedPaths`, `filterState`) nearly every function touches.

## What Changes

- `ui/js/app.js` is split into ES modules by concern (state, API client, table rendering, details view, polling, bootstrap/wiring), loaded via native `<script type="module">` — no bundler, no new build step, no new dependencies, consistent with the project's existing zero-build-tooling approach (Tailwind via CDN, plain JS throughout).
- Behavior is unchanged: the existing table view, filters, selection, bulk actions, details view (including the candidate picker and cover browser already built), and polling all work identically before and after this change — this is a structural refactor, not a feature change.
- A minimal, currently-inert view-switcher scaffold (a `currentView` state value and the module boundary a future view would plug into) is put in place so `add-library-views-and-playback` can add Grid/Tree/Artist-Album views as new modules without another structural upheaval — but no additional view-switcher UI ships in this change, since introducing UI for views that don't exist yet would be confusing and untestable on its own.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
(none — this is an internal, behavior-preserving refactor; no capability's requirements change)

## Impact

- Changed code: `ui/js/app.js` is deleted and replaced by `ui/js/` modules (exact boundaries decided in `design.md`); `ui/index.html`'s script tag(s) updated to load the entry module.
- No backend changes, no API changes, no schema changes.
- No new user-facing behavior — verification for this change is specifically about confirming *nothing* changed from the user's perspective, across every existing flow (list/filter/sort/paginate, select/bulk-act, details view including candidates/cover-browse/lyrics/embedded-tags/fingerprint, delete, all five polling loops).
- This change should land before `add-library-views-and-playback` so that change's new views/player land in the post-refactor module structure rather than being bolted onto the monolith and needing to be moved again later. It has no ordering dependency on `improve-library-visibility` or `add-manual-search-identify` — either could land before or after this one, though this change is easiest to verify cleanly against a stable baseline (ideally before those two add more surface to split apart).
