## 1. Domain and persistence

- [x] 1.1 Add `Lyrics` and `SyncedLyrics` string fields to `FileRecord` in `internal/domain/tracking.go`
- [x] 1.2 Add `lyrics`/`synced_lyrics` columns to the `columnMigrations` list in `internal/infrastructure/persistence/sqlite_store.go` (idempotent `ALTER TABLE ADD COLUMN`, same pattern as `cover_art_path`)
- [x] 1.3 Add a `RecordLyrics(ctx, path, lyrics, syncedLyrics string) error` method to the store, mirroring `RecordCoverArt`
- [x] 1.4 Add a `GetLyrics(ctx, path string) (lyrics, syncedLyrics string, ok bool, err error)` dedicated single-row lookup method, mirroring `GetCoverArtPath`
- [x] 1.5 Update `RecordIdentification` to also clear `Lyrics`/`SyncedLyrics` (alongside the existing `cover_art_path` reset) on both `identified` and `not_found` outcomes
- [x] 1.6 Update `LoadAll` to populate `Lyrics`/`SyncedLyrics` on the returned `FileRecord`s

## 2. LRCLIB gateway client

- [x] 2.1 Add `LyricsLookup` port to `internal/usecases/ports.go`: `Lookup(ctx, artist, title, album string, durationSeconds int) (plainLyrics, syncedLyrics string, found bool, err error)`
- [x] 2.2 Implement `internal/infrastructure/gateways/lrclib_client.go`: `GET https://lrclib.net/api/get` with `artist_name`, `track_name`, `album_name`, `duration` query params
- [x] 2.3 Parse `plainLyrics`, `syncedLyrics`, `instrumental` from the response body
- [x] 2.4 Treat HTTP 404 (`TrackNotFound`) as `found=false, err=nil`
- [x] 2.5 Treat `instrumental: true` as `found=false, err=nil` (leave lyrics fields empty)
- [x] 2.6 Treat any other non-2xx response or a network/decode error as a returned `err`
- [x] 2.7 Set a descriptive `User-Agent` header on the request, consistent with the MusicBrainz client's convention

## 3. Enrichment pipeline

- [x] 3.1 Introduce an `EnrichmentInput` struct (`Path`, `ReleaseMBID`, `ReleaseGroupMBID`, `Artist`, `Title`, `Album`, `DurationSeconds`) in `internal/usecases/enrich_file.go`
- [x] 3.2 Change `EnrichFile.Enrich`'s signature from `(ctx, path, releaseMBID, releaseGroupMBID string) error` to `(ctx context.Context, input EnrichmentInput) error`
- [x] 3.3 Add a `LyricsLookup` dependency to `EnrichFile`
- [x] 3.4 Attempt cover art lookup and lyrics lookup independently within `Enrich` (neither short-circuits the other); combine any errors from both via `errors.Join`
- [x] 3.5 On successful lyrics resolution, call `RecordLyrics`; on "not found"/instrumental, leave lyrics fields untouched (empty) without treating it as an error
- [x] 3.6 Update `EnrichManager` (and any caller of `Enrich`) to build and pass an `EnrichmentInput` populated from the tracked file's resolved metadata (artist, title, album, duration, release/release-group MBIDs)

## 4. API

- [x] 4.1 Add `has_lyrics` field to the `LibraryEntry` DTO in `internal/infrastructure/web/v1/library_handler.go`, derived from whether `Lyrics` or `SyncedLyrics` is non-empty
- [x] 4.2 Add a `GET /api/v1/library/lyrics?path=...` handler that calls `GetLyrics` and returns `200` with `{plain_lyrics, synced_lyrics}` JSON, or `404` if none stored
- [x] 4.3 Register the new route in `internal/infrastructure/web/v1/router.go`

## 5. Composition root

- [x] 5.1 Construct the LRCLIB client in `cmd/server/main.go` and wire it into `EnrichFile` as the `LyricsLookup` dependency

## 6. Web UI

- [x] 6.1 Add a small lyrics indicator to each table row when `has_lyrics` is true, in `ui/js/app.js`
- [x] 6.2 Add a scrollable lyrics section to the details modal, showing `plain_lyrics` (falling back to `synced_lyrics` if `plain_lyrics` is empty)
- [x] 6.3 Fetch `GET /api/v1/library/lyrics` only when the details view opens for a file with `has_lyrics` true
- [x] 6.4 Update the "Enrich Selected" button's label/tooltip if needed to reflect that it now also resolves lyrics

## 7. Verification

- [x] 7.1 Run `go build ./...` and `go vet ./...` inside Docker
- [x] 7.2 Verify the LRCLIB gateway client against the real API for a known track (found case) and a known-instrumental or nonexistent track (not-found case)
- [x] 7.3 Rebuild and run via `docker compose up --build` against the user's real music library volume; identify and enrich a handful of real files
- [x] 7.4 Confirm `GET /api/v1/library` includes `has_lyrics`, `GET /api/v1/library/lyrics` returns expected text, and the details view renders it
- [x] 7.5 Confirm re-identifying a previously enriched file clears its stored lyrics (and cover art) as expected
- [x] 7.6 Confirm the server restarts with lyrics still retrievable without re-running enrichment
