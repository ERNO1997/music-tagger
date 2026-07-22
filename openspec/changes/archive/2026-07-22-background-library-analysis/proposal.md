## Why

Today, `has_lyrics`/`has_cover_art`/`status`/`relocated` only ever reflect what this app itself has done: fingerprinting is lazy (only computed when identify runs), enrichment only runs when a user selects files and clicks "Enrich," and `relocated` is only ever set by the explicit relocate action actually moving a file. On a first run against an already-organized library — files already embedded with cover art and lyrics from another tool, or already sitting in an Artist/Album/Track folder structure from prior manual organization — none of that ground truth is reflected until the user manually selects everything and runs identify/enrich/relocate, even though the answer is already sitting in the files and their paths. The `has_lyrics`/`has_cover_art`/`relocated` filters are effectively unusable against such a library until then.

## What Changes

- After each library refresh, automatically (no user action) run a background analysis pass over every new/changed tracked file:
  - Compute its Chromaprint fingerprint if it doesn't have one yet — the same computation identify already does lazily, just run proactively instead of waiting for a user to trigger identify.
  - Read the file's own embedded cover art and lyrics directly from the file itself (extending the same tag-reading path already used by the details view's "Embedded Tags" section) and, if either is present and the tracking record doesn't already have one (from a prior enrichment or a prior pass of this same analysis), store it — so `has_cover_art`/`has_lyrics` and their filters reflect what's actually embedded in the file, not only what this app has separately downloaded.
  - For every `identified` and `tagged` file not already marked `relocated`, check whether its current path already equals its computed canonical destination (the same `{Artist}/{year - Album}/{track - Title}` computation the relocate action already uses) and, if so, mark it `relocated` — without moving the file, since it's already there.
- Surface this as a background job with its own progress indicator, following the same UI convention as scan/identify/enrich/tag/relocate (a status line while running, filters/table updating as it completes files).

## Capabilities

### New Capabilities
- `background-library-analysis`: automatic, no-trigger-required fingerprinting, embedded-content detection, and passive relocation detection, run after each refresh.

### Modified Capabilities
- `file-tracking-store`: broadens "Enrichment results are recorded per file" and "Relocation results are recorded per file" to note that cover art/lyrics and the relocated outcome can now also be recorded by this new automatic pass, not only by the explicit enrich/relocate actions — without changing what those explicit actions themselves do.

## Impact

- Backend: a new background-analysis manager (mirroring `RefreshManager`/`IdentifyManager`'s existing shape), chained to run automatically after each refresh completes; a new `GET /api/v1/library/analyze/status` endpoint for progress polling; the embedded-tag reader extended to return actual lyrics text and cover image bytes (not just the existing boolean flags) when present; the relocation destination-path computation (currently inline inside `PathRelocator.Relocate`) extracted into a reusable, filesystem-free function so this new passive check can call it without performing a move.
- Frontend: a progress indicator for this job on the existing library status line, following the same pattern as the other four background jobs.
- No change to what identify/enrich/tag/relocate themselves do when explicitly triggered — this only adds an automatic path that can get to the same outcomes sooner, and never overwrites something those actions (or a prior pass of this one) already recorded.
- No dependency on any other in-progress change.
