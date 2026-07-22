## 1. Extract reusable, filesystem-free helpers

- [ ] 1.1 In `internal/infrastructure/filestat/relocator.go`, extract `PathRelocator.Relocate`'s destination-path computation (sanitize segments, build `dir`/`filename`/`dest`) into its own function taking `RelocateInput` and returning the computed path, with no filesystem access; have `Relocate` call it, preserving its existing behavior exactly
- [ ] 1.2 Check whether the existing TagLib wrapper backing `TagFile.GetEmbeddedTags` can already return raw embedded picture bytes and lyrics text (not just presence booleans); extend it if not, keeping the existing `GetEmbeddedTags`/`GET /api/v1/library/tags` boolean-only response contract unchanged

## 2. Background analysis manager

- [ ] 2.1 Add a new manager (mirroring `IdentifyManager`/`RelocateManager`'s shape) that, for a batch of tracked files: computes a fingerprint for any file lacking one (reusing the existing fingerprinting mechanism); reads embedded cover art/lyrics for any file whose tracking record lacks them and stores whichever is found, without overwriting an existing value; and, for `identified`+`tagged`+not-yet-`relocated` files, compares the current path against the computed destination (task 1.1) and marks `relocated` on a match
- [ ] 2.2 Chain this manager to run automatically after every refresh completes (`RefreshManager`, alongside its existing `SetRelocateStatus` chaining), covering both the startup-triggered refresh and any on-demand one
- [ ] 2.3 Serialize this pass with an in-progress relocate job (reusing whatever mutual-exclusion mechanism `RefreshManager`/`RelocateManager` already share for scan-vs-relocate)
- [ ] 2.4 Add `GET /api/v1/library/analyze/status` reporting `{running, processed, total}`, registered alongside the other status endpoints

## 3. Frontend progress indicator

- [ ] 3.1 Add a fifth job to `ui/src/composables/useJobs.js` (`analyze`), polling the new status endpoint the same way the other four already do
- [ ] 3.2 Show an "Analyzing… X/Y" state on the existing library status line while it's running, and refresh the current view when it completes (same pattern as the other four jobs)

## 4. Verification

- [ ] 4.1 Against a library with files that already have embedded cover art/lyrics but no prior enrichment, run a refresh and confirm the analysis pass fills in `has_cover_art`/`has_lyrics` (and the details view's cover/lyrics sections) without requiring Enrich to be clicked
- [ ] 4.2 Confirm a file that already has enriched cover art/lyrics is left unchanged by the analysis pass, even if its embedded tags differ
- [ ] 4.3 Identify and tag a file already sitting at its canonical destination path, run a refresh, and confirm the analysis pass marks it `relocated` without moving it
- [ ] 4.4 Confirm a fresh (never-fingerprinted) library gets every file fingerprinted automatically after its first refresh, observable via the new status endpoint, and that a subsequent manual Identify does not recompute those fingerprints
- [ ] 4.5 Confirm the analysis pass does not start while a relocate job is running, and that triggering relocate while analysis is running is handled the same way scan-vs-relocate already is
