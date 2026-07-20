## 1. AcoustID confidence threshold

- [ ] 1.1 Add a named `minAcoustIDConfidence` constant (e.g. `0.7`) in `internal/usecases/identify_file.go`
- [ ] 1.2 In `IdentifyFile.Identify`, after `u.acoustID.Lookup`, check `matches[0].Score` against `minAcoustIDConfidence` before calling MusicBrainz; if below threshold, record `IdentificationResult{Status: domain.StatusNotFound}` (same as the existing "no matches at all" path) and return without calling MusicBrainz
- [ ] 1.3 Confirm a match at or above the threshold proceeds exactly as before (no behavior change for the already-working path)

## 2. LRCLIB fuzzy-search fallback

- [ ] 2.1 Add a fuzzy-search call to `LRCLIBClient` (`internal/infrastructure/gateways/lrclib_client.go`) hitting `https://lrclib.net/api/search?track_name=<title>&artist_name=<artist>`, parsing a JSON array of the same shape as `/api/get` (`trackName`, `artistName`, `albumName`, `duration`, `instrumental`, `plainLyrics`, `syncedLyrics`)
- [ ] 2.2 In `Lookup`, when the exact `/api/get` call returns 404, call the new fuzzy search instead of returning `found=false` immediately
- [ ] 2.3 Among the fuzzy search's results, select the one whose `duration` is closest to the given `durationSeconds` (falling back to the first result if `durationSeconds` is 0 or the search response is otherwise ambiguous)
- [ ] 2.4 If the fuzzy search returns zero results, or the selected candidate is marked instrumental, treat it identically to today's "not found"/"instrumental" handling
- [ ] 2.5 Confirm request/error handling (network errors, non-2xx responses) on the fuzzy-search call surfaces the same way `/api/get`'s does — a distinguishable error, not silently treated as "not found"

## 3. `has_lyrics` filter

- [ ] 3.1 Add `HasLyrics *bool` to `LibraryFilter` in `internal/usecases/ports.go`
- [ ] 3.2 Add a `(lyrics != '' OR synced_lyrics != '')` / negated clause to `buildLibraryWhere` in `internal/infrastructure/persistence/sqlite_store.go`, parallel to the existing `tagged`/`relocated` clauses
- [ ] 3.3 Parse a `has_lyrics` query parameter in `LibraryHandler.List` (`internal/infrastructure/web/v1/library_handler.go`), same pattern as `tagged`/`relocated`
- [ ] 3.4 Add a "Lyrics: any/yes/no" filter `<select>` to `ui/index.html`, styled like the existing `tagged`/`relocated` selects
- [ ] 3.5 Wire the new filter control in `ui/js/app.js`: add `hasLyrics` to `filterState`, include it in `buildListParams`/`currentFilterPayload` alongside `tagged`/`relocated`, reset `pageState.offset` on change

## 4. Verification

- [ ] 4.1 Run `go build ./...` and `go vet ./...` inside Docker
- [ ] 4.2 Seed a scratch database with a known-low-score AcoustID scenario (mock `AcoustIDLookup`/`MusicBrainzLookup` in a unit test, or use the real Daft Punk file if reproducible) and confirm the file is recorded `not_found`, not `identified`, and MusicBrainz is never called
- [ ] 4.3 Confirm a match at or above the threshold still resolves and records normally (no regression)
- [ ] 4.4 Confirm the LRCLIB fallback resolves lyrics for the real Lenka "Everything at Once" case (or an equivalent exact-match-miss/fuzzy-match-hit case) end to end through `EnrichFile`
- [ ] 4.5 Confirm `GET /api/v1/library?has_lyrics=false` and `?has_lyrics=true` each return only the expected subset, with `total` reflecting the filtered count
- [ ] 4.6 Confirm the web UI's new lyrics filter control re-fetches and re-renders correctly, and combines correctly with the existing status/tagged/relocated filters and search
