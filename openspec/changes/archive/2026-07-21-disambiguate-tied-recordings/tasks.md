## 1. AcoustID grouped results

- [x] 1.1 Replace `AcoustIDMatch{RecordingID, Score}` with `AcoustIDResult{Score float64, RecordingIDs []string}` in `internal/usecases/ports.go`; update `AcoustIDLookup.Lookup`'s return type to `[]AcoustIDResult`
- [x] 1.2 Update `AcoustIDClient.Lookup` (`internal/infrastructure/gateways/acoustid_client.go`) to group each AcoustID result's recording IDs into one `AcoustIDResult` instead of flattening into separate per-recording matches, preserving descending-score order

## 2. Domain and ports

- [x] 2.1 Add `StatusAmbiguous TrackingStatus = "ambiguous"` to `internal/domain/tracking.go`
- [x] 2.2 Add `RecordAmbiguous(ctx context.Context, path string, candidates []RecordingMetadata) error`, `GetCandidates(ctx context.Context, path string) ([]RecordingMetadata, error)`, and `ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error)` to the `TrackingStore` interface in `internal/usecases/ports.go`

## 3. Usecases

- [x] 3.1 Update `IdentifyFile.Identify` (`internal/usecases/identify_file.go`): after the existing confidence-threshold check accepts the top `AcoustIDResult`, if it ties more than one recording ID, resolve each via `MusicBrainzLookup.Lookup` (one call per tied recording, respecting the existing rate gate) and dedupe the results by `(Artist, Title)`
  - **Post-implementation fix**: found live on two more real files ("League of Legends – Legends Never Die", "Sam Smith – Too Good At Goodbyes") whose tied recordings included one with no MusicBrainz release attached (e.g. a bare instrumental entry) — `MusicBrainzClient.Lookup` returns `domain.ErrNoMusicBrainzRelease` for that one recording, and the loop originally aborted the *entire* identify attempt on that single per-candidate error instead of skipping it and continuing with the other resolvable candidates. Fixed: a per-candidate `ErrNoMusicBrainzRelease` is now skipped (not a viable candidate), any other error still aborts the whole attempt (existing gateway-error convention), and if zero candidates remain after skipping, the file is recorded `not_found`. Covered by `TestIdentify_TiedRecordingWithNoRelease_IsSkippedNotAborted` and verified live: both real files now correctly resolve to `ambiguous` with their genuine candidates (instrumental excluded), and were manually resolved to their correct recordings.
  - **Post-implementation refinement (user request)**: originally only the single best-scoring `AcoustIDResult`'s tied recordings were ever considered — every other result AcoustID returned (each with its own score and possibly its own tied recordings) was silently discarded, so a genuinely valid candidate landing in the 2nd/3rd-best result instead of being tied into the top one was never surfaced at all. Changed: every result at or above `minAcoustIDConfidence` (not just the top one) now contributes its recordings to the same resolve-and-dedupe pool, stopping at the first below-threshold result (results are returned in descending-score order, an AcoustID API guarantee already relied on for the top-result confidence check). Covered by `TestIdentify_SecondQualifyingResult_ContributesCandidates` and `TestIdentify_BelowThresholdSecondResult_DoesNotContributeCandidates`; verified live on the real Daft Punk file, which now surfaces a 6th candidate ("Daft Punk feat. Pharrell Williams & Nile Rodgers — Get Lucky", the actual *Random Access Memories* album version) that was previously invisible because it lived in a lower-scoring result.
- [x] 3.2 If dedup collapses to a single distinct identity, record that file `identified` with that identity — unchanged behavior from today's single-recording path
- [x] 3.3 If ≥2 distinct identities remain after dedup, call the new `store.RecordAmbiguous` with the full candidate list instead of calling `RecordIdentification`, and return without picking one
- [x] 3.4 Add `IdentifyFile.ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error)` delegating to `store.ResolveAmbiguous`

## 4. Persistence

- [x] 4.1 Add an `identification_candidates` table in `internal/infrastructure/persistence/sqlite_store.go`'s schema setup: `path`, `recording_mbid`, plus the same resolved-metadata columns already used on `files` (artist, album, title, track_number, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid), primary keyed on `(path, recording_mbid)`
- [x] 4.2 Implement `SQLiteStore.RecordAmbiguous`: in one transaction, delete any existing candidate rows for `path`, insert the new candidate set, and update the `files` row to `status = 'ambiguous'` with resolved metadata cleared and enrichment/tagged/relocated outcomes invalidated (mirroring `RecordIdentification`'s existing not-found/invalidation branch)
- [x] 4.3 Implement `SQLiteStore.GetCandidates`: select all candidate rows for `path`, mapped to `[]usecases.RecordingMetadata`
- [x] 4.4 Implement `SQLiteStore.ResolveAmbiguous`: in one transaction, look up the candidate row matching `(path, recordingMBID)` — if found, perform the same update `RecordIdentification` already does for a successful identification (status=identified, resolved metadata, invalidations) using that candidate's metadata, and delete all candidate rows for `path`; if not found, change nothing and return `found=false`
- [x] 4.5 Extend `RecordIdentification`'s existing update (both its `identified` and `not_found` branches) to also delete any stale candidate rows for `path`, so a previous ambiguous outcome's candidates never linger past a fresh identification attempt

## 5. API

- [x] 5.1 Add a `GET /api/v1/library/candidates` handler (`internal/infrastructure/web/v1/`) mirroring `FingerprintHandler.Get`'s convention: `200` with an (possibly empty) candidate list for a tracked path, `404` for an untracked path
- [x] 5.2 Add a `POST /api/v1/library/identify/resolve` handler parsing `{"path": "...", "recording_mbid": "..."}`, calling `IdentifyFile.ResolveAmbiguous`, responding `200 OK` on success and `404 Not Found` when the candidate doesn't match
- [x] 5.3 Register both new routes in `internal/infrastructure/web/v1/router.go` and wire their handlers in `cmd/server/main.go`
- [x] 5.4 Confirm (read-only check, no code change expected) that `GET /api/v1/library`'s `status` query parameter already passes `ambiguous` straight through to `LibraryFilter.Status` with no allow-list rejecting it — verify directly rather than assuming

## 6. Web UI

- [x] 6.1 Add `ambiguous` as a fourth option to the status filter `<select id="filter-status">` in `ui/index.html`
- [x] 6.2 Add an `ambiguous` entry to `STATUS_LABELS` and `STATUS_CLASSES` in `ui/js/app.js` with its own distinct color, separate from `identified`/`not_found`/`missing`
- [x] 6.3 In the details view (`ui/js/app.js`), when the opened row's status is `ambiguous`, fetch `GET /api/v1/library/candidates` and render each candidate's artist/album/title/track number with a "Use this" button
- [x] 6.4 Wire "Use this" to call `POST /api/v1/library/identify/resolve` with the row's path and the chosen candidate's recording ID, then refresh that row (and the details view) to reflect the new `identified` state on success

## 7. Composition root

- [x] 7.1 `cmd/server/main.go` needed one addition beyond the "no change expected" assumption: a new `CandidatesHandler` wired into the updated `RegisterRoutes` call. `IdentifyFile`'s constructor dependencies themselves are indeed unchanged, as anticipated.

## 8. Verification

- [x] 8.1 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 8.2 Unit test: a top AcoustID result tied to recordings that resolve to distinct identities records the file `ambiguous` with all distinct candidates stored, and writes no resolved metadata to the file's own record
- [x] 8.3 Unit test: a top AcoustID result tied to recordings that all resolve to the same identity records the file `identified` exactly as an unambiguous single-recording match would
- [x] 8.4 Unit test: resolving a valid candidate records the file `identified` with that candidate's metadata and discards its other stored candidates
- [x] 8.5 Unit test: resolving an unrecognized candidate recording ID returns `found=false` without modifying the file's status or stored candidates
- [x] 8.6 Against the real, previously-misidentified Daft Punk "Get Lucky (Radio Edit)" file: re-run identify and confirm it now records `ambiguous` with the official "Daft Punk feat. Pharrell Williams — Get Lucky" candidate among its stored options, rather than silently re-picking the wrong "Walt Ribeiro" compilation match
- [x] 8.7 Confirm resolving that file's correct candidate via `POST /api/v1/library/identify/resolve` records it `identified` with the official metadata, and that `GET /api/v1/library/candidates` returns an empty list for it afterward
- [x] 8.8 Confirm `GET /api/v1/library?status=ambiguous` returns only ambiguous files, with `total` reflecting that filtered count
- [ ] 8.9 Confirm the web UI's ambiguous filter, status indicator, and candidate picker render and resolve correctly for a real ambiguous file (manual browser check)

## 9. Cover-art browsing (post-implementation addition, per user request)

- [x] 9.1 Add `ReleaseGroupRelease`, `MusicBrainzReleaseGroupLookup`, `CoverArtCandidate`, and `CoverArtBrowser` to `internal/usecases/ports.go`
- [x] 9.2 Implement `MusicBrainzClient.Releases` (`internal/infrastructure/gateways/musicbrainz_client.go`): resolve a release-group's sibling releases via `GET /release-group/<id>?inc=releases`, subject to the existing rate gate
- [x] 9.3 Implement `CoverArtClient.FrontImage` and `CoverArtClient.Download` (`internal/infrastructure/gateways/coverart_client.go`): list a single release's front-cover thumbnail/image URLs without downloading, and download arbitrary previously-listed image bytes; `Download` rejects any URL whose host isn't `coverartarchive.org`
- [x] 9.4 Add `internal/usecases/browse_cover_art.go`: `BrowseCoverArt.Candidates` resolves a tracked file's release-group siblings (capped at `maxCoverCandidateReleases = 20`) and checks each for a front image; `Choose` downloads a picked image and records it via the existing `CoverArtStore`/`RecordCoverArt` path, identical to automatic enrichment
- [x] 9.5 Add `GET /api/v1/library/cover/candidates` and `POST /api/v1/library/cover/choose` handlers (`internal/infrastructure/web/v1/cover_browse_handler.go`), register routes, wire `BrowseCoverArt` in `cmd/server/main.go`
- [x] 9.6 Add a "Browse other covers…" toggle to the details view (`ui/index.html`/`ui/js/app.js`), shown only for `identified` rows, rendering a thumbnail grid that calls the choose endpoint on click and refreshes the displayed cover
- [x] 9.7 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 9.8 Verify live: checked 7 real releases in the library against Cover Art Archive first, confirming no single release ever has more than one front-marked image (validating the release-group-siblings approach over a within-release picker)
- [x] 9.9 Verify live end-to-end against a real 23-sibling release-group (Daft Punk's *Random Access Memories*): candidates endpoint returned 18 distinct real cover images; chose one, confirmed it downloaded, saved, and now serves via `GET /library/cover`
- [x] 9.10 Verify 404 for an untracked path and for a tracked-but-unidentified path
- [x] 9.11 Verify the SSRF guard: `POST /api/v1/library/cover/choose` with a non-Cover-Art-Archive image URL is refused rather than fetched
