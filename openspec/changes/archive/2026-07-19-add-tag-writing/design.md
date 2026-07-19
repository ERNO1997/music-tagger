## Context

Every prior step in the pipeline (fingerprint, identify, enrich) only reads from external services and writes to the SQLite tracking store — none of them touch a file under `/music`. This change is the first to write to the audio files themselves, which changes the risk profile: a bug here can corrupt or destroy a user's actual library, not just a database row.

The existing background-job pattern (`JobManager` → `IdentifyManager`/`EnrichManager` → `*_handler.go`) already solves "run this over N selected paths, in the background, one job at a time, with pollable progress." Tagging reuses that pattern rather than inventing a new one.

`internal/domain/audiofile.go` and `FileRecord` already carry every field tagging needs (`Artist`, `Album`, `Title`, `TrackNumber`, `AlbumArtist`, `Year`, `DiscNumber`, `TotalDiscs`, `TotalTracks`, `CoverArtPath`, `Lyrics`), so no new upstream lookups are required — this change is purely "take what's already resolved and stored, and write it into the file."

## Goals / Non-Goals

**Goals:**
- Write resolved metadata, cover art, and plain lyrics into the physical MP3/FLAC/M4A file at its current path, for one or more selected already-identified files, on demand.
- Preserve every existing frame/atom/comment not explicitly listed as a target field (e.g. custom tags, ReplayGain, encoder comments).
- Make tagging idempotent and safe to re-run: re-tagging a file with the same resolved metadata produces the same on-disk result, and a failed tag attempt leaves the original file untouched.
- Surface per-file tagged/failed status through the API and UI, consistent with how identification/enrichment status is already surfaced.
- Let a user visually verify what was actually written, by reading a tagged file's real embedded tags back from disk (independent of the tracking store) and showing them in the details view.

**Non-Goals:**
- File relocation/renaming into `Artist/Album/Track - Title` (separate future change).
- Automatic tagging as part of scan or identify (stays on-demand/human-triggered, same as identify and enrich).
- Writing synced (LRC-timestamped) lyrics — only plain lyrics are embedded, since none of ID3v2/Vorbis/MP4's standard lyrics fields support LRC timing natively; synced lyrics remain a details-view-only feature.
- Embedding cover art larger than what Cover Art Archive returns (no resizing/transcoding of the image).

## Decisions

### Library choice: `go.senan.xyz/taglib` (`github.com/sentriz/go-taglib`), not three per-format libraries
A single library — a Go binding to the real TagLib C++ library, compiled to **Wasm** and run via `github.com/tetratelabs/wazero` (a pure-Go Wasm runtime) — replaces the originally-considered three separate per-format pure-Go libraries (`bogem/id3v2`, `go-flac`, and an MP4 box-level writer):
- **No CGO**: the module's only dependency is `wazero`; `CGO_ENABLED=0` builds cleanly, satisfying the charter's static-binary constraint (§2.2) without a native TagLib install anywhere in the Docker image.
- **One API for all three formats** (plus WAV/OGG/WMA, unused here): `taglib.ReadTags`/`WriteTags` operate on a normalized `map[string][]string` keyed by constants (`taglib.Title`, `Artist`, `Album`, `AlbumArtist`, `TrackNumber`, `DiscNumber`, `Date`, `Lyrics`, ...) that TagLib itself maps to the correct underlying representation per format — `TIT2`/`TPE1`/... for ID3v2, `TITLE`/`ARTIST`/... Vorbis comments for FLAC, `©nam`/`©ART`/... atoms for MP4. `taglib.WriteImage`/`ReadImage` handle cover art the same way across all three.
- **Removes the MP4 risk entirely**: because this is TagLib's actual, decades-battle-tested C++ code (not a reimplementation), MP4's chunk-offset (`stco`/`co64`) bookkeeping on rewrite is handled correctly by the library itself — no spike or fallback plan needed (see Risks, below, for what this decision removes).
- `WriteTags` defaults to merging into existing tags; passing the `taglib.Clear` flag replaces the tag set wholesale — this change uses the default (merge) behavior, consistent with "preserve unrelated existing tag data."
- License is LGPL-2.1; the Wasm blob is loaded at runtime by `wazero` rather than statically linked into the Go binary, which is a materially different (and simpler) situation than CGO-linking LGPL C++ code directly.

Rejected alternative: the original three-pure-Go-library plan. Trades a small amount of "more implementation code, one library per format, full manual control over exact frame IDs" for "one dependency, proven MP4 handling, no spike" — a clear win given identical functional requirements (per the `audio-tag-writing` spec, which describes target frame/atom/comment names, not which library produces them).

### One `Tagger` port, one implementation wrapping `go-taglib`
Mirrors `Fingerprinter`'s existing shape (`internal/usecases/ports.go`): a single interface,
```go
type Tagger interface {
    Tag(ctx context.Context, path string, meta TagInput) error
}
```
implemented by a single `TagLibTagger` in `internal/infrastructure/filestat/taglib_tagger.go` that builds the normalized tag map and calls `taglib.WriteTags` (and `taglib.WriteImage` when cover art is present) — no per-format dispatch needed, since `go-taglib` determines the file's format itself. This is simpler than originally planned (no `FormatTagger` dispatcher, no three separate files).

### `TagFile` usecase reads from the tracking store, not from its caller
Like `EnrichFile`, `TagFile.Tag(ctx, path)` loads the file's `FileRecord` itself (via a single-record store lookup, not `LoadAll`) rather than requiring the caller to assemble a DTO — keeps `TagManager` thin and matches the "single indexed lookup" pattern already established for `GetCoverArtPath`/`GetLyrics`. Requires a new `TrackingStore.Get(ctx, path) (domain.FileRecord, bool, error)` method (currently only `LoadAll` exists for bulk and per-field getters exist for cover art/lyrics specifically — none return a full record for one path).

### Read cover art bytes from disk via `CoverArtStore`
`EnrichFile` already downloads cover art to disk via `CoverArtStore.Save`/`.Path` (`internal/infrastructure/covers`). `TagFile` reads the same file back into memory (`os.ReadFile` on `FileRecord.CoverArtPath`) rather than re-fetching from Cover Art Archive — no new external calls, and consistent with the file being the on-disk cache it was designed as.

### Tagging outcome stored as a status + error, not a boolean
Mirrors `FingerprintError`'s existing pattern on `FileRecord`: add `Tagged bool` and `TagError string`. A file can be re-tagged after a prior failure (e.g. file was locked) without needing a distinct retry endpoint — the same `POST /api/v1/library/tag` call is used for both first-time tagging and re-tagging.

### Endpoint and job shape: identical to enrich
`POST /api/v1/library/tag` (body: `{"paths": [...]}`) → `202 Accepted`, backed by a new `TagManager` (its own `JobManager`, independent of identify/enrich/refresh — tagging touches the filesystem, a distinct resource from any of the three). `GET /api/v1/library/tag/status` for polling. Skip-and-log (not abort) for any path that isn't `StatusIdentified`, exactly like `EnrichManager.Start`.

### Format is determined by content-sniffing, not file extension
`go-taglib` (like TagLib itself) dispatches purely by file extension. Discovered during real-file verification (task 9.2): a user file named `....mp3` was actually an M4A/MP4 container (no ID3 header, valid `ftyp M4A` box) — almost certainly saved with the wrong extension by whatever tool produced it. Tagging it by trusting the `.mp3` extension caused TagLib to prepend a valid ID3v2 header onto the front of the MP4 content. Our own `ReadEmbeddedTags` (using the same extension-based dispatch) read that ID3v2 header back and reported success — but independent, content-aware tools (`ffprobe`, `mutagen`) still saw the file as MP4 and read its original, untouched MP4 atoms, unaware anything had changed. The write had no effect on anything that reads the file by its real container type.

This is the same class of problem the charter's Acoustic-First Identification Rule already exists to prevent — filenames are untrusted input — just recurring one layer up, for the tag-writing format instead of track identity. The fix: `internal/infrastructure/filestat/format_detect.go` sniffs a file's real format from its leading bytes (`ftyp` box → M4A, `fLaC` → FLAC, `ID3` header or bare MPEG frame sync → MP3) before every `Tag`/`ReadEmbeddedTags` call. When the sniffed format disagrees with the extension, the file is renamed to a sibling path with the *correct* extension for the duration of the TagLib call, then renamed back to its original name immediately after — so tagging targets the file's real format, while its filename on disk never changes (file relocation remains explicitly out of scope for this change). When the format can't be determined from content, the extension is trusted as a fallback rather than failing outright — matching the same non-error posture as other "not found"/"unavailable" cases in this pipeline.

### Verification reads bypass the tracking store entirely
A `tagged = true` flag only proves the write call returned no error — it says nothing about whether the value written matches what was intended, or whether a subtle library bug wrote something different. Rather than trusting that flag as the only signal, `TagFile` (or a small sibling, `ReadEmbeddedTags(ctx, path) (EmbeddedTags, error)`) calls `taglib.ReadTags`/`ReadImage` fresh against the file itself whenever `GET /api/v1/library/tags` is requested — this is a live read of the current on-disk bytes, not a cached copy, so it stays correct even if the file is edited by something else after tagging. The response includes title/artist/album/album artist/track number/disc number/year as plain values, plus booleans for lyrics-present and cover-art-present (not the lyrics text or image bytes themselves — those are already available via the existing `/lyrics` and `/cover` endpoints and don't need duplicating). The details view renders this next to the resolved metadata so a mismatch is visually obvious rather than requiring the user to open the file in an external tool.

### Write strategy: modify in place, not write-then-rename
Each format library's native "open, mutate in memory, save" API rewrites the file in place (typically via a temp-file-plus-rename internally, e.g. `id3v2`'s `Save()`). This change does not add its own atomic-write wrapper on top — that would double up on temp-file handling the underlying libraries already do correctly, and any bug in a home-grown wrapper is strictly worse than trusting one well-used library's tested save path per format.

## Risks / Trade-offs

- **[Risk] A tagging bug corrupts or truncates a real audio file** → Mitigation: verification (tasks.md) requires testing against real copies of each format before running against the user's actual library, per [[no-local-native-deps]] constraints (the test/build loop stays inside Docker); writing is delegated entirely to TagLib's own save path (via `go-taglib`) rather than a home-grown atomic-write layer, which minimizes custom on-disk-mutation code.
- **[Risk] `go-taglib`'s bundled Wasm binary lags a TagLib fix or has its own bug** → Mitigation: it's a single, actively-maintained dependency (vs. three) to watch/upgrade; pin the version in `go.mod` and note it in verification.
- **[Risk] A file's extension doesn't match its real container format** → Mitigation: content-sniffed format detection (see Decisions, above) routes the TagLib call to the file's real format regardless of extension; caught during real-file verification, not left as a theoretical concern.
- **[Known limitation] Extension-trusting consumers won't see correctly-written tags on a mislabeled file** → Writing into the file's *real* format (per the fix above) is correct for content-sniffing tools (`ffprobe`, `mutagen`, most real players/DAPs), but a consumer that itself trusts the extension — notably macOS Finder/Spotlight, confirmed via `mdls` showing `kMDItemContentType = public.mp3` for a `.mp3`-named file that's actually M4A — will still look for ID3 data, find none, and show no metadata or cover art. No tag-writing behavior can satisfy both an extension-trusting reader and a content-sniffing one at once while the extension itself is wrong; the only complete fix is correcting the file's extension to match its real content, which is file relocation/renaming — explicitly out of scope for this change (see the charter's separate future "Relocate" step). Accepted as a known limitation for this change; a user hitting it must rename the affected file themselves, or wait for the relocation change.
- **[Risk] Overwriting a user's manually-curated tags with MusicBrainz data they didn't want** → Mitigation: tagging stays strictly on-demand and per-selection (never automatic), consistent with identify/enrich; out of scope to add a "diff before writing" preview in this change, but noted as a natural UI follow-up.
- **[Trade-off] No synced-lyrics embedding** → accepted: no standard tag field supports LRC timing across all three formats without a non-standard extension; plain lyrics via the normalized `taglib.Lyrics` key (mapping to `USLT`/`LYRICS`/`©lyr` per format) cover the common case.

## Migration Plan

- New `tagged`/`tag_error` columns added via the existing idempotent `ALTER TABLE ... ADD COLUMN` migration pattern already used for cover art/lyrics columns — no backfill needed; existing rows default to untagged.
- No rollback concern for the schema (additive-only). Rollback of the feature itself is: stop calling the new endpoints; no data migration to reverse.

## Open Questions

- Exact representation of total-tracks/total-discs via `go-taglib`'s normalized keys (e.g. a combined `"3/12"`-style `TRACKNUMBER` value vs. a separate field) — to be confirmed against the library's behavior per format during implementation (task 3.x), since it isn't fully pinned down by the README alone.
- Whether ID3v2 tag version (v2.3 vs v2.4) is something TagLib chooses automatically or something this change needs to control explicitly — default to TagLib's own default unless verification surfaces a real player-compatibility problem.
