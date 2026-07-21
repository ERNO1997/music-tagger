## 1. Ports

- [x] 1.1 Add `MusicBrainzSearch` interface (`Search(ctx, query string, limit int) ([]RecordingMetadata, error)`) to `internal/usecases/ports.go`

## 2. Infrastructure

- [x] 2.1 Implement `MusicBrainzClient.Search` (`internal/infrastructure/gateways/musicbrainz_client.go`): call `GET /ws/2/recording?query=<query>&fmt=json&limit=<limit>` subject to the existing rate gate, then resolve each hit's recording ID via the same metadata-shaping path `Lookup` already uses (reusing `selectRelease` and friends) — extracted a shared `resolveRecording` helper so `Lookup` and `Search` share one metadata-resolution code path

## 3. Usecases

- [x] 3.1 Add `internal/usecases/manual_search.go`: `ManualSearch.Search(ctx, path, query string) (candidates []RecordingMetadata, err error)` — calls `MusicBrainzSearch.Search`, and on a non-empty result calls `store.RecordAmbiguous(ctx, path, candidates)` before returning them; on an empty result, returns an empty list without touching the store
  - **Design refinement during implementation**: added a `found bool` return and an upfront `store.Get` existence check. `RecordAmbiguous` itself doesn't validate that `path` is tracked — calling it for an untracked path would insert orphaned rows into `identification_candidates` (no FK constraint ties it to `files`) and silently no-op the `files` UPDATE. Checking existence first (mirroring `TagFile.Tag`'s self-loading pattern) lets the API handler return `404` for an untracked path per the spec, without ever reaching the search/store calls for one.

## 4. API

- [x] 4.1 Add a `POST /api/v1/library/identify/search` handler parsing `{"path": "...", "query": "..."}`, calling `ManualSearch.Search`, responding `200 OK` with the candidate list (same shape as `GET /api/v1/library/candidates`), `404` for an untracked path
- [x] 4.2 Register the new route in `internal/infrastructure/web/v1/router.go` and wire `ManualSearch` in `cmd/server/main.go`

## 5. Web UI

- [x] 5.1 Add a "Search manually" control to the details view (`ui/index.html`/`ui/js/app.js`), available regardless of the row's current status: free-text input (or separate artist/title/album fields composed into one query client-side) — implemented as three fields (Artist/Title/Album) composed into a Lucene query client-side
- [x] 5.2 If the file's current status is `identified`, prompt for confirmation before submitting the search (since it discards current resolved metadata immediately)
- [x] 5.3 Render search results using the existing candidate-list/"Use this" rendering already built for ambiguous AcoustID candidates; wire "Use this" to the existing `POST /api/v1/library/identify/resolve` endpoint (no changes needed there) — extracted a shared `renderCandidates` helper reused by both the auto-shown ambiguous-candidate path and manual search results
- [x] 5.4 Handle the zero-results case: indicate no matches found without altering the displayed row

## 6. Verification

- [x] 6.1 Run `go build ./...` and `go vet ./...` inside Docker (also ran `go test ./...` — all 12 tests pass, including the 3 new manual-search tests)
- [x] 6.2 Unit test: `ManualSearch.Search` with results calls `RecordAmbiguous` with the returned candidates
- [x] 6.3 Unit test: `ManualSearch.Search` with zero results does not call `RecordAmbiguous` and returns an empty list
  - Also added a third unit test beyond the plan: `TestManualSearch_UntrackedPath_ReturnsNotFoundWithoutSearching`, covering the `found` existence check added during 3.1's implementation refinement
- [x] 6.4 Against a real file in the library (pick one that's `not_found` or has a wrong/`ambiguous` result), manually search by its real artist/title and confirm the correct recording appears as a candidate — verified live: `/music/05. Beat It.mp3` (real Michael Jackson track) searched with `artist:"Michael Jackson" AND recording:"Beat It"` returned 8 real candidates including the exact match (Thriller, 1982, track 5)
- [x] 6.5 Confirm resolving a manual-search-sourced candidate via `POST /api/v1/library/identify/resolve` records it `identified` exactly like an AcoustID-sourced candidate would, and that tagging/relocation treat it identically — verified live: resolved the correct candidate, then successfully ran `POST /library/tag` against it with no special-casing needed. Did not exercise physical relocation live (moves a real file on disk, more invasive than tagging) — relocation eligibility is structurally guaranteed by both paths converging on the same `identified` status, which is all `RelocateFile` ever checks
- [x] 6.6 Confirm manually searching an already-`identified` file discards its prior resolved metadata and candidates, and that a zero-result search leaves an existing file's state untouched — verified live: a nonsense query against the identified/tagged file returned zero candidates with status/artist/tagged all unchanged; a real query against the same file immediately reset it to `ambiguous` with artist/tagged cleared, confirming the discard-on-search behavior
- [x] 6.7 Confirm the web UI's manual search control, confirmation prompt (for already-identified files), and result rendering work correctly for a real file (manual browser check) — confirmed by user in a real browser
