## 1. Domain and persistence

- [x] 1.1 Extend `domain.FileRecord` with `CoverArtPath`
- [x] 1.2 Add the corresponding column to the SQLite schema's migration list (same idempotent pattern already in place)
- [x] 1.3 Update `LoadAll` to read the new column
- [x] 1.4 Add a `TrackingStore.RecordCoverArt` method: sets `CoverArtPath` for a path without altering fingerprint, status, or resolved metadata — also found and fixed a gap not explicitly called out in design.md: `RecordIdentification` now resets `cover_art_path` to blank on both `identified` and `not_found` outcomes, since a re-identification can resolve to a different release, which would otherwise leave stale cover art attached to the wrong release

## 2. Cover Art Archive gateway client

- [x] 2.1 Define a `CoverArtLookup` port in `internal/usecases/ports.go`
- [x] 2.2 Implement `internal/infrastructure/gateways/coverart_client.go`: `GET https://coverartarchive.org/release/{mbid}`
- [x] 2.3 Parse the `images` array; prefer `front: true`, fall back to the first image
- [x] 2.4 Select the `large` (~500px) thumbnail URL; upgrade `http://` to `https://` before requesting
- [x] 2.5 Treat a 404 response as "no cover art available" (empty result, not an error); treat any other non-2xx/network/malformed-response failure as an error — verified against the real API with a release that has art, one that doesn't, and an invalid ID (all three 404 identically for "no art"/"unknown ID", correctly returning `(nil, nil)`)
- [x] 2.6 Download the selected image's bytes and return them to the caller — verified: 62857 bytes downloaded, matching an independent manual check exactly
- [x] 2.7 **Added after real-world verification**: fall back to Cover Art Archive's `/release-group/{mbid}` endpoint when the specific release 404s — found live that a release-group can have dozens of "Official" sibling editions and our release-selection heuristic has no way to know in advance which has art; the release-group endpoint auto-resolves to a sibling that does. `CoverArtLookup.Lookup` now takes both `releaseMBID` and `releaseGroupMBID`. Verified against the exact real failing case (a 2025 release with no art directly, 89080 bytes recovered via its release-group) and confirmed no regression on the already-working case (still exactly 62857 bytes)

## 3. Image storage

- [x] 3.1 Implement storage under `/data/covers/<release-mbid>.jpg` (directory created as needed, same pattern as the SQLite DB's directory creation) — implemented as `internal/infrastructure/covers.Store`, deriving the `/data` base directory from the same path already used for the SQLite DB (no new configuration needed)
- [x] 3.2 Before calling Cover Art Archive, check whether `/data/covers/<release-mbid>.jpg` already exists; if so, skip the API call and reuse the existing file (avoids redundant downloads for multiple tracks on the same release) — exposed as `Store.Path`, to be used by the enrich usecase (§4)

## 4. Enrich usecase and background job

- [x] 4.1 Implement an enrich-one-file usecase: look up the file's Release MBID from its tracking record; skip (log, don't abort the batch) if not yet `identified`; call `CoverArtLookup`, store the image (per §3), and call `RecordCoverArt`
- [x] 4.2 Implement `EnrichManager` composing the shared `JobManager` (same shape as `IdentifyManager`): `Start(paths []string) error`, `Status() JobStatus`
- [x] 4.3 Confirm `EnrichManager` uses its own independent guard — can run concurrently with both `RefreshManager` and `IdentifyManager` (structurally guaranteed: separate `JobManager` instance, same as `IdentifyManager`'s relationship to `RefreshManager`)

## 5. API

- [x] 5.1 Add `POST /api/v1/library/enrich`: parses `{"paths": [...]}`, calls `EnrichManager.Start`, returns `202 Accepted` or `409 Conflict`
- [x] 5.2 Add `GET /api/v1/library/enrich/status`: returns running/processed/total
- [x] 5.3 Add `GET /api/v1/library/cover?path=...`: looks up the file's `CoverArtPath` and serves the image bytes with `Content-Type: image/jpeg`, or `404` if none is stored — added a dedicated `TrackingStore.GetCoverArtPath` single-row lookup rather than reusing `LoadAll`, since the browser calls this once per rendered thumbnail and a full-table load per image would be wasteful at any real library size
- [x] 5.4 Add a `has_cover_art` (or equivalent) field to `LibraryEntry` so the UI knows whether to request the image

## 6. Web UI

- [x] 6.1 Add an "Enrich Selected" button alongside "Identify Selected", wired to `POST /api/v1/library/enrich`
- [x] 6.2 Poll `GET /api/v1/library/enrich/status` (and re-fetch `GET /api/v1/library`) while a job is running; disable the enrich action and show progress; re-enable on completion — same pattern as identify polling
- [x] 6.3 Render a small cover art thumbnail (`<img src="/api/v1/library/cover?path=...">`) in each row when `has_cover_art` is true (a gray placeholder box otherwise)
- [x] 6.4 Render a larger cover art image in the details view when present

## 7. Verification

- [x] 7.1 Verify (via Docker, against the user's real identified library) that cover art is fetched, stored under `/data/covers/`, and served correctly through `GET /api/v1/library/cover` — 2 of 3 real files fetched and served correctly (500x500 JPEG, correct `Content-Type`); confirmed the schema migration applies cleanly against the user's existing volume
- [x] 7.2 Verify a release with no uploaded cover art (404 from Cover Art Archive) leaves `CoverArtPath` empty without an error — the 3rd real file's release genuinely 404s from Cover Art Archive (a brand-new 2025 release with no art uploaded yet); confirmed correctly handled as "no art", not an error
- [x] 7.3 Verify two tracks resolving to the same release share one stored image file (no duplicate download) — the user's 3 real files each resolve to distinct releases, so directly re-verified the dedup mechanism instead: re-enriching an already-covered file leaves its stored image's mtime unchanged (no re-download/re-write)
- [x] 7.4 Verify enriching an unidentified file is skipped without aborting the rest of the batch — verified in a disposable scratch volume with a `status: new` file; job completed (1/1 processed), no cover art path written, no crash
- [x] 7.5 Verify `409 Conflict` on a concurrent enrich trigger — confirmed with two near-simultaneous triggers (202/409)
- [x] 7.6 Verify an enrich job runs concurrently with a scan refresh and/or an identify job without error — confirmed all three (scan/identify/enrich) accepted concurrently (202/202/202)
- [x] 7.7 Verify the UI shows thumbnails in the table and in the details view — confirmed the served `index.html`/`app.js` contain the current markup/logic (Enrich Selected button, cover cell rendering, details-view image) and validated the rendering logic by code inspection; did not drive an actual browser click, since no browser automation is available in this environment
