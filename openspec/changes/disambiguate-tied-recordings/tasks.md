## 1. AcoustID grouped results

- [ ] 1.1 Replace `AcoustIDMatch{RecordingID, Score}` with `AcoustIDResult{Score float64, RecordingIDs []string}` in `internal/usecases/ports.go`; update `AcoustIDLookup.Lookup`'s return type to `[]AcoustIDResult`
- [ ] 1.2 Update `AcoustIDClient.Lookup` (`internal/infrastructure/gateways/acoustid_client.go`) to group each AcoustID result's recording IDs into one `AcoustIDResult` instead of flattening into separate per-recording matches, preserving descending-score order

## 2. Domain and ports

- [ ] 2.1 Add `StatusAmbiguous TrackingStatus = "ambiguous"` to `internal/domain/tracking.go`
- [ ] 2.2 Add `RecordAmbiguous(ctx context.Context, path string, candidates []RecordingMetadata) error`, `GetCandidates(ctx context.Context, path string) ([]RecordingMetadata, error)`, and `ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error)` to the `TrackingStore` interface in `internal/usecases/ports.go`

## 3. Usecases

- [ ] 3.1 Update `IdentifyFile.Identify` (`internal/usecases/identify_file.go`): after the existing confidence-threshold check accepts the top `AcoustIDResult`, if it ties more than one recording ID, resolve each via `MusicBrainzLookup.Lookup` (one call per tied recording, respecting the existing rate gate) and dedupe the results by `(Artist, Title)`
- [ ] 3.2 If dedup collapses to a single distinct identity, record that file `identified` with that identity — unchanged behavior from today's single-recording path
- [ ] 3.3 If ≥2 distinct identities remain after dedup, call the new `store.RecordAmbiguous` with the full candidate list instead of calling `RecordIdentification`, and return without picking one
- [ ] 3.4 Add `IdentifyFile.ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error)` delegating to `store.ResolveAmbiguous`

## 4. Persistence

- [ ] 4.1 Add an `identification_candidates` table in `internal/infrastructure/persistence/sqlite_store.go`'s schema setup: `path`, `recording_mbid`, plus the same resolved-metadata columns already used on `files` (artist, album, title, track_number, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid), primary keyed on `(path, recording_mbid)`
- [ ] 4.2 Implement `SQLiteStore.RecordAmbiguous`: in one transaction, delete any existing candidate rows for `path`, insert the new candidate set, and update the `files` row to `status = 'ambiguous'` with resolved metadata cleared and enrichment/tagged/relocated outcomes invalidated (mirroring `RecordIdentification`'s existing not-found/invalidation branch)
- [ ] 4.3 Implement `SQLiteStore.GetCandidates`: select all candidate rows for `path`, mapped to `[]usecases.RecordingMetadata`
- [ ] 4.4 Implement `SQLiteStore.ResolveAmbiguous`: in one transaction, look up the candidate row matching `(path, recordingMBID)` — if found, perform the same update `RecordIdentification` already does for a successful identification (status=identified, resolved metadata, invalidations) using that candidate's metadata, and delete all candidate rows for `path`; if not found, change nothing and return `found=false`
- [ ] 4.5 Extend `RecordIdentification`'s existing update (both its `identified` and `not_found` branches) to also delete any stale candidate rows for `path`, so a previous ambiguous outcome's candidates never linger past a fresh identification attempt

## 5. API

- [ ] 5.1 Add a `GET /api/v1/library/candidates` handler (`internal/infrastructure/web/v1/`) mirroring `FingerprintHandler.Get`'s convention: `200` with an (possibly empty) candidate list for a tracked path, `404` for an untracked path
- [ ] 5.2 Add a `POST /api/v1/library/identify/resolve` handler parsing `{"path": "...", "recording_mbid": "..."}`, calling `IdentifyFile.ResolveAmbiguous`, responding `200 OK` on success and `404 Not Found` when the candidate doesn't match
- [ ] 5.3 Register both new routes in `internal/infrastructure/web/v1/router.go` and wire their handlers in `cmd/server/main.go`
- [ ] 5.4 Confirm (read-only check, no code change expected) that `GET /api/v1/library`'s `status` query parameter already passes `ambiguous` straight through to `LibraryFilter.Status` with no allow-list rejecting it — verify directly rather than assuming

## 6. Web UI

- [ ] 6.1 Add `ambiguous` as a fourth option to the status filter `<select id="filter-status">` in `ui/index.html`
- [ ] 6.2 Add an `ambiguous` entry to `STATUS_LABELS` and `STATUS_CLASSES` in `ui/js/app.js` with its own distinct color, separate from `identified`/`not_found`/`missing`
- [ ] 6.3 In the details view (`ui/js/app.js`), when the opened row's status is `ambiguous`, fetch `GET /api/v1/library/candidates` and render each candidate's artist/album/title/track number with a "Use this" button
- [ ] 6.4 Wire "Use this" to call `POST /api/v1/library/identify/resolve` with the row's path and the chosen candidate's recording ID, then refresh that row (and the details view) to reflect the new `identified` state on success

## 7. Composition root

- [ ] 7.1 Confirm (read-only check, no code change expected) that `cmd/server/main.go` needs no wiring changes — `IdentifyFile`'s constructor dependencies are unchanged, and the new store methods are already reachable through the existing `TrackingStore` instance

## 8. Verification

- [ ] 8.1 Run `go build ./...` and `go vet ./...` inside Docker
- [ ] 8.2 Unit test: a top AcoustID result tied to recordings that resolve to distinct identities records the file `ambiguous` with all distinct candidates stored, and writes no resolved metadata to the file's own record
- [ ] 8.3 Unit test: a top AcoustID result tied to recordings that all resolve to the same identity records the file `identified` exactly as an unambiguous single-recording match would
- [ ] 8.4 Unit test: resolving a valid candidate records the file `identified` with that candidate's metadata and discards its other stored candidates
- [ ] 8.5 Unit test: resolving an unrecognized candidate recording ID returns `found=false` without modifying the file's status or stored candidates
- [ ] 8.6 Against the real, previously-misidentified Daft Punk "Get Lucky (Radio Edit)" file: re-run identify and confirm it now records `ambiguous` with the official "Daft Punk feat. Pharrell Williams — Get Lucky" candidate among its stored options, rather than silently re-picking the wrong "Walt Ribeiro" compilation match
- [ ] 8.7 Confirm resolving that file's correct candidate via `POST /api/v1/library/identify/resolve` records it `identified` with the official metadata, and that `GET /api/v1/library/candidates` returns an empty list for it afterward
- [ ] 8.8 Confirm `GET /api/v1/library?status=ambiguous` returns only ambiguous files, with `total` reflecting that filtered count
- [ ] 8.9 Confirm the web UI's ambiguous filter, status indicator, and candidate picker render and resolve correctly for a real ambiguous file (manual browser check)
