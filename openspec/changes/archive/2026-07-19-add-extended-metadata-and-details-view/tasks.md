## 1. Domain and persistence

- [x] 1.1 Extend `domain.FileRecord` with `AlbumArtist`, `Year`, `DiscNumber`, `TotalDiscs`, `TotalTracks`, `ReleaseMBID`, `ReleaseGroupMBID`, `ArtistMBID`
- [x] 1.2 Add the corresponding columns to the SQLite schema's migration list (same idempotent `PRAGMA table_info` + `ALTER TABLE ADD COLUMN` pattern already in place)
- [x] 1.3 Update `LoadAll` to read the new columns
- [x] 1.4 Update `RecordIdentification` to persist the new fields when status is `identified`

## 2. MusicBrainz client parsing

- [x] 2.1 Add `ID`, `Date`, `ArtistCredit` fields to the `mbRelease` struct — verified against the real MusicBrainz API (same recording used in the original identification change)
- [x] 2.2 Add `ID` field to the `mbReleaseGroup` struct
- [x] 2.3 Add `Position`, `TrackCount` fields to the `mbMedium` struct
- [x] 2.4 Add a nested `Artist{ID}` struct to `mbArtistCredit` to capture the artist MBID
- [x] 2.5 Change `selectRelease` to return `(release, medium, track, ok)` instead of `(release, track, ok)`, since disc number and total-tracks-on-that-disc are medium-level fields
- [x] 2.6 Implement year parsing: take the leading 4 characters of the release date, tolerating `"YYYY"`, `"YYYY-MM"`, `"YYYY-MM-DD"`; leave `Year` unset if unparseable or absent
- [x] 2.7 Populate `usecases.RecordingMetadata` with the new fields (album artist joined the same way as track artist; total discs computed as `len(release.Media)`) — all 7 fields confirmed correct against the live API
- [x] 2.8 Add `AlbumArtist`, `Year`, `DiscNumber`, `TotalDiscs`, `TotalTracks`, `ReleaseMBID`, `ReleaseGroupMBID`, `ArtistMBID` to `usecases.RecordingMetadata`

## 3. API

- [x] 3.1 Add the new fields to `LibraryEntry` (all `omitempty`, matching the existing pattern for resolved-metadata fields) — also found and fixed a gap from the prior change: `recording_mbid` was persisted but never exposed via the API; added it now since the details view needs to show it
- [x] 3.2 Populate them in `LibraryHandler.List` from the `FileRecord`

## 4. Web UI details view

- [x] 4.1 Add a details modal/panel (hidden by default) to `ui/index.html`
- [x] 4.2 Keep the last-fetched entries array in memory in `app.js` after each successful `loadLibrary()` call
- [x] 4.3 Add a row click handler that looks up the clicked row's path in the in-memory array and populates/shows the details modal
- [x] 4.4 Ensure clicking the row checkbox does not also open details — implemented via an `e.target.closest('input')` guard in the delegated row click handler rather than a per-checkbox `stopPropagation()`, same effect with less listener plumbing (also caught and fixed a class-overwrite bug: the error row's `row.className = 'text-red-400'` would have wiped the new `cursor-pointer`/`hover` classes; changed to `classList.add`)
- [x] 4.5 Render all available fields in the modal (path, format, duration, fingerprint, status, error, and resolved metadata when present), omitting fields that aren't set rather than showing placeholder/zero values
- [x] 4.6 Add a close action (close button and clicking outside the modal)

## 5. Verification

- [x] 5.1 Verify (via Docker, against real identified files) that Album Artist, Year, Disc Number, Total Discs, Total Tracks, and the three MBIDs are all populated correctly in `GET /api/v1/library` — verified against the user's real 3-file library by re-running identify; all fields correct and match the live MusicBrainz API
- [x] 5.2 Verify no additional MusicBrainz requests are issued for the extended fields (same request count as before this change, for the same identify job) — guaranteed by construction (only parsing was added, no new request call sites) and consistent with observed timing (~3-4s for 3 files, same pacing as before)
- [x] 5.3 Verify a release with a partial or missing date leaves `Year` unset (omitted from JSON) rather than showing `0` or erroring — verified `parseYear` directly against `"2018"`, `"2018-05"`, `"2018-05-14"`, `""`, `"18"`, `"unknown"`; all resolve correctly (0 for the unparseable/short cases)
- [x] 5.4 Verify clicking a row opens its details view with correct data, and clicking its checkbox does not — confirmed the served `index.html`/`app.js` contain the current markup/logic and validated the click-delegation logic by direct code inspection (an `e.target.closest('input')` guard on the row-click handler); did not drive an actual browser click, since no browser automation is available in this environment
- [x] 5.5 Verify the details view for a `new`/`not_found`/`missing` file shows only the fields available for that status, with no fabricated metadata — verified by code inspection (the render loop explicitly skips `undefined`/`null`/`''` values, and unidentified files simply don't have those keys in the JSON response); no live unidentified file was available in the test library to click through in a browser
