## 1. Dependencies

- [x] 1.1 Add `go.senan.xyz/taglib` (`github.com/sentriz/go-taglib`) to `go.mod`, pinning a specific version
- [x] 1.2 Confirm `CGO_ENABLED=0 GOOS=linux go build ./...` still succeeds inside Docker with the new dependency (per the project's no-local-native-deps constraint — this build/test loop stays inside Docker, not on the host)
- [x] 1.3 Spot-check `taglib.ReadTags`/`WriteTags`/`WriteImage` against one real file per format (`.mp3`, `.flac`, `.m4a`) in a throwaway scratch script to confirm the exact normalized-key behavior (esp. how track/disc totals are represented) before wiring it into the tagger implementation — confirmed against real fixtures in Docker: `TRACKNUMBER`/`DISCNUMBER` accept combined `"N/M"` strings and round-trip correctly across MP3/FLAC/M4A; default (non-`Clear`) `WriteTags` preserves an untouched pre-existing tag; `WriteImage` embeds correctly on all three

## 2. Domain and ports

- [x] 2.1 Add `Tagged bool` and `TagError string` fields to `domain.FileRecord` (`internal/domain/tracking.go`)
- [x] 2.2 Add a `Tagger` port to `internal/usecases/ports.go`: `Tag(ctx context.Context, path string, meta TagInput) error`, plus a `TagInput` struct carrying artist/album/title/albumArtist/trackNumber/discNumber/totalDiscs/totalTracks/year/coverArt bytes/lyrics (format param dropped — `go-taglib` detects format itself)
- [x] 2.3 Add a `TrackingStore.Get(ctx, path) (domain.FileRecord, bool, error)` method (single-record lookup, distinct from `LoadAll`) to the `TrackingStore` interface

## 3. Tagger implementation

- [x] 3.1 Implement `internal/infrastructure/filestat/taglib_tagger.go`: a `TagLibTagger` satisfying the `Tagger` port, building a normalized `map[string][]string` from `TagInput` using `taglib.Title`/`Artist`/`Album`/`AlbumArtist`/`TrackNumber`/`DiscNumber`/`Date` keys and calling `taglib.WriteTags` (default merge behavior, not `taglib.Clear`, to preserve unrelated existing fields)
- [x] 3.2 Confirm and encode how total-tracks/total-discs are represented via `go-taglib` (per the task 1.3 spike finding) in the tag map construction — combined `"n/total"` string, via `formatNumberPair`
- [x] 3.3 Call `taglib.WriteImage` with the cover art bytes when present, and set the `taglib.Lyrics` key when lyrics are present
- [x] 3.4 Verify existing unrelated tags (e.g. a custom comment, ReplayGain fields) are preserved after a write — not just that the target fields were set correctly — for one real file per format (confirmed at the library level in the task 1.3 spike; re-verified end-to-end in task 9.3)
- [x] 3.5 Add a `ReadEmbeddedTags(ctx context.Context, path string) (EmbeddedTags, error)` method to the `Tagger` port (or a sibling interface) that calls `taglib.ReadTags`/`ReadImage` live against the file and returns title/artist/album/album artist/track number/disc number/year plus lyrics-present/cover-art-present booleans; implement it on `TagLibTagger`
- [x] 3.6 Implement `internal/infrastructure/filestat/format_detect.go`: `sniffFormat` (content-based format detection via leading bytes — `ftyp` box for M4A, `fLaC` magic, ID3 header or bare MPEG frame sync for MP3) and `withCorrectExtension` (renames to a correctly-extensioned sibling path for the duration of a TagLib call when the real format disagrees with the file's extension, then renames back) — added after real-file verification (task 9.5) surfaced that `go-taglib` dispatches purely by extension
- [x] 3.7 Wire `Tag` and `ReadEmbeddedTags` on `TagLibTagger` through `withCorrectExtension`

## 4. Usecase and job manager

- [x] 4.1 Implement `internal/usecases/tag_file.go`: `TagFile.Tag(ctx, path)` loads the file's record via `TrackingStore.Get`, skips (returns `skipped=true`, not an error) if status isn't `identified`, reads cover art bytes from disk directly (`os.ReadFile` on `CoverArtPath`) when set, builds a `TagInput` from the record, calls `Tagger.Tag`, and records the outcome via `TrackingStore.RecordTagged`
- [x] 4.2 Implement `internal/usecases/tag_manager.go`: `TagManager` wrapping its own `JobManager` (independent of scan/identify/enrich), iterating submitted paths and calling `TagFile.Tag` per path, logging and continuing past per-file skips/errors, following `EnrichManager`'s shape exactly
- [x] 4.3 Add `ErrTagInProgress = ErrJobInProgress` alias and a `TagStatus = JobStatus` alias, matching `EnrichManager`'s pattern
- [x] 4.4 Implement `TagFile.GetEmbeddedTags(ctx, path)`, which loads the file's record via `TrackingStore.Get` (to confirm it's tracked and not `missing`) and delegates to `Tagger.ReadEmbeddedTags` — a pure read, no store writes

## 5. Persistence

- [x] 5.1 Add `tagged` (INTEGER/boolean) and `tag_error` (TEXT) columns to the migration column list in `internal/infrastructure/persistence/sqlite_store.go` (same `ALTER TABLE files ADD COLUMN` pattern as the existing cover-art/lyrics columns)
- [x] 5.2 Implement `SQLiteStore.Get(ctx, path) (domain.FileRecord, bool, error)` — single-row `SELECT` of all tracked columns by path
- [x] 5.3 Implement `SQLiteStore.RecordTagged(ctx, path, tagged bool, tagErr string)` — updates only `tagged`/`tag_error`/`updated_at`, matching `RecordCoverArt`'s shape
- [x] 5.4 Update `SQLiteStore.LoadAll` to scan the new `tagged`/`tag_error` columns into `FileRecord`
- [x] 5.5 Update `SQLiteStore.RecordIdentification` to also reset `tagged = 0, tag_error = ''` in both the identified and not-identified branches (alongside the existing `cover_art_path`/`lyrics`/`synced_lyrics` reset), per the "re-identification invalidates prior tagged outcome" requirement

## 6. API

- [x] 6.1 Add `tagged` (and, when failed, an error indicator) to the `LibraryEntry` DTO in `internal/infrastructure/web/v1/library_handler.go`
- [x] 6.2 Add `internal/infrastructure/web/v1/tag_handler.go` with `Trigger` (`POST /api/v1/library/tag`) and `Status` (`GET /api/v1/library/tag/status`) handlers, mirroring `enrich_handler.go` exactly (400 on empty paths, 409 via `ErrTagInProgress`, 202 on accept)
- [x] 6.3 Add an `EmbeddedTagsHandler` (`GET /api/v1/library/tags?path=...`), mirroring `lyrics_handler.go`'s shape (200 with JSON body, 404 if the path is unknown or missing from disk) — `internal/infrastructure/web/v1/embedded_tags_handler.go`
- [x] 6.4 Register the three new routes in `internal/infrastructure/web/v1/router.go`

## 7. Composition root

- [x] 7.1 Construct the `TagLibTagger` and `TagFile`/`TagManager` in `cmd/server/main.go`, wiring in the existing `TrackingStore` (`TagFile` reads cover art bytes directly via `os.ReadFile`, so no separate `CoverArtStore` wiring is needed here)
- [x] 7.2 Wire the new `TagHandler` (including the embedded-tags read endpoint) into the router registration in `main.go`

## 8. Web UI

- [x] 8.1 Add a "Tag Selected" bulk action button to `ui/js/app.js`, alongside the existing "Identify Selected"/"Enrich Selected" actions, calling `POST /api/v1/library/tag` with selected paths and polling `GET /api/v1/library/tag/status`
- [x] 8.2 Disable the tag action and show progress while a tag job is running, re-enabling on completion (same pattern as the enrich action)
- [x] 8.3 Add a small tagged indicator to each table row when `tagged` is true, and a distinct indicator when tagging previously failed
- [x] 8.4 In the details view, when a file's `tagged` indicator is present, fetch `GET /api/v1/library/tags` and render an "Embedded tags" section (title/artist/album/album artist/track/disc/year, plus lyrics-present/cover-art-present) positioned directly next to the existing resolved-metadata section, so the two are visually comparable at a glance
- [x] 8.5 Do not fetch or render the embedded-tags section for files with no `tagged` indicator

## 9. Verification

- [x] 9.1 Run `go build ./...` and `go vet ./...` inside Docker — clean
- [x] 9.2 Copy a handful of real `.mp3`, `.flac`, and `.m4a` files (not the user's live library) into a scratch test directory; exercised the tag → embedded-tag-read-back path end-to-end via a throwaway harness (`TagFile.Tag` + `TagFile.GetEmbeddedTags` against a real SQLite store and real audio fixtures for all three formats, seeded to simulate a completed identify+enrich without needing live AcoustID/MusicBrainz/LRCLIB calls) — all three formats wrote and read back title/artist/album/album artist/track/disc/year/lyrics/cover-art correctly; harness deleted after passing
- [x] 9.3 Confirm a file's pre-existing unrelated tag data survives tagging (test per format, per task 3.5) — confirmed for all three formats in the same harness (a pre-set `COMMENT` tag survived a full `Tag` call)
- [x] 9.4 Confirm re-identifying a previously tagged file clears its `tagged` flag (per `RecordIdentification`'s reset) and that re-running tag writes updated values — confirmed for all three formats in the same harness
- [x] 9.5 Built the real Docker image and ran the full app (via `docker run`, real `ACOUSTID_API_KEY`/`MUSICBRAINZ_USER_AGENT` from `.env`) against copies of 3 real files from the user's library (2 `.m4a`, 1 `.mp3` — copies only, originals never touched) through the actual scan → identify → enrich → tag HTTP API end to end. All 3 correctly identified via live AcoustID/MusicBrainz, enriched (cover art + lyrics; one release had no cover art available, correctly non-error), and tagged.
- [x] 9.5.1 **Found via 9.5**: one file named `....mp3` was actually a valid M4A/MP4 container (no ID3 header, real `ftyp M4A` box) — tagging it by trusting the `.mp3` extension caused an ID3v2 header to be prepended onto the MP4 content; our own read-back (same extension-based dispatch) reported success, but independent tools (`ffprobe`, `mutagen`) still read the file as MP4 and saw only its original, untouched tags. Reported to the user; fixed via content-based format detection (tasks 3.6/3.7). Re-verified after the fix: `mutagen` confirms all fields (title/artist/album/album artist/track/disc/year/lyrics/cover art) now land correctly in the file's real MP4 atoms, and the filename/extension is unchanged.
- [x] 9.6 Confirm `GET /api/v1/library` includes `tagged` — confirmed via the real run above (all 3 files showed `tagged: true`)
- [ ] 9.6.1 Confirm the web UI's "Tag Selected" action and tagged indicator work end-to-end in an actual browser (API-level behavior is confirmed; the button/indicator rendering itself has not been visually checked in a browser)
- [x] 9.7 Confirm `GET /api/v1/library/tags` returns values matching what was written — confirmed via the real run above for all 3 files, cross-checked independently against `mutagen`'s reading of the actual file bytes
- [ ] 9.7.1 Confirm the details view's embedded-tags section renders correctly in an actual browser next to the resolved metadata for a tagged file, and is absent for an untagged one
- [x] 9.8 Rebuilt and ran via `docker compose up --build` against the user's real music library volume and tagged the 3 real, already-identified/enriched files present there. Confirmed via `mutagen` (independent of our code) that all fields, cover art, and lyrics are correctly embedded in the real `Let Me Down Slowly...mp3` file's actual MP4 atoms (title/artist/album/album artist/track/disc/year/lyrics/cover all match resolved metadata).
- [x] 9.8.1 **Found via 9.8**: macOS Finder/Spotlight shows no metadata or cover art for the mislabeled `.mp3` file even though tags were correctly written into its real MP4 format — confirmed via `mdls` that Finder determines `kMDItemContentType` from the file's extension (`public.mp3`) rather than its content, so it looks for ID3 data that doesn't exist there. This is the same extension-mismatch problem recurring at the OS metadata layer; documented as a known, accepted limitation (design.md, proposal.md) rather than fixed — the complete fix is renaming the file to match its real format, which is out of scope (file relocation, a separate future change). User decision: leave as documented limitation for this change.
