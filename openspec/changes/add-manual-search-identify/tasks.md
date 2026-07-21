## 1. Ports

- [ ] 1.1 Add `MusicBrainzSearch` interface (`Search(ctx, query string, limit int) ([]RecordingMetadata, error)`) to `internal/usecases/ports.go`

## 2. Infrastructure

- [ ] 2.1 Implement `MusicBrainzClient.Search` (`internal/infrastructure/gateways/musicbrainz_client.go`): call `GET /ws/2/recording?query=<query>&fmt=json&limit=<limit>` subject to the existing rate gate, then resolve each hit's recording ID via the same metadata-shaping path `Lookup` already uses (reusing `selectRelease` and friends)

## 3. Usecases

- [ ] 3.1 Add `internal/usecases/manual_search.go`: `ManualSearch.Search(ctx, path, query string) (candidates []RecordingMetadata, err error)` — calls `MusicBrainzSearch.Search`, and on a non-empty result calls `store.RecordAmbiguous(ctx, path, candidates)` before returning them; on an empty result, returns an empty list without touching the store

## 4. API

- [ ] 4.1 Add a `POST /api/v1/library/identify/search` handler parsing `{"path": "...", "query": "..."}`, calling `ManualSearch.Search`, responding `200 OK` with the candidate list (same shape as `GET /api/v1/library/candidates`), `404` for an untracked path
- [ ] 4.2 Register the new route in `internal/infrastructure/web/v1/router.go` and wire `ManualSearch` in `cmd/server/main.go`

## 5. Web UI

- [ ] 5.1 Add a "Search manually" control to the details view (`ui/index.html`/`ui/js/app.js`), available regardless of the row's current status: free-text input (or separate artist/title/album fields composed into one query client-side)
- [ ] 5.2 If the file's current status is `identified`, prompt for confirmation before submitting the search (since it discards current resolved metadata immediately)
- [ ] 5.3 Render search results using the existing candidate-list/"Use this" rendering already built for ambiguous AcoustID candidates; wire "Use this" to the existing `POST /api/v1/library/identify/resolve` endpoint (no changes needed there)
- [ ] 5.4 Handle the zero-results case: indicate no matches found without altering the displayed row

## 6. Verification

- [ ] 6.1 Run `go build ./...` and `go vet ./...` inside Docker
- [ ] 6.2 Unit test: `ManualSearch.Search` with results calls `RecordAmbiguous` with the returned candidates
- [ ] 6.3 Unit test: `ManualSearch.Search` with zero results does not call `RecordAmbiguous` and returns an empty list
- [ ] 6.4 Against a real file in the library (pick one that's `not_found` or has a wrong/`ambiguous` result), manually search by its real artist/title and confirm the correct recording appears as a candidate
- [ ] 6.5 Confirm resolving a manual-search-sourced candidate via `POST /api/v1/library/identify/resolve` records it `identified` exactly like an AcoustID-sourced candidate would, and that tagging/relocation treat it identically
- [ ] 6.6 Confirm manually searching an already-`identified` file discards its prior resolved metadata and candidates, and that a zero-result search leaves an existing file's state untouched
- [ ] 6.7 Confirm the web UI's manual search control, confirmation prompt (for already-identified files), and result rendering work correctly for a real file (manual browser check)
