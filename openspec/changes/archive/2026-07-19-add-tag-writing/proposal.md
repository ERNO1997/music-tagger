## Why

Identification (AcoustID/MusicBrainz), cover art, and lyrics are all resolved and stored in the tracking database, but none of it ever reaches the actual audio files — the on-disk MP3/FLAC/M4A tags are untouched. Per the project charter (§1.3 Phase C), "Tag" is the next unimplemented step in the core pipeline, and it's a prerequisite for the later "Relocate" step, since relocation depends on trusting the file's own embedded metadata as the final, portable record once it leaves this system's tracking store.

## What Changes

- Add a `Tagger` usecase port with one implementation per format, writing resolved metadata, cover art, and lyrics directly into the audio file's own tag format:
  - MP3: ID3v2 `TIT2`/`TPE1`/`TALB`/`TPE2`/`TRCK`/`TPOS` text frames, `APIC` for cover art, `USLT` for plain lyrics.
  - FLAC: Vorbis comments (`TITLE`/`ARTIST`/`ALBUM`/`ALBUMARTIST`/`TRACKNUMBER`/`DISCNUMBER`/`DATE`) and a `PICTURE` metadata block, plus a `LYRICS` comment.
  - M4A/MP4: iTunes atoms (`©nam`/`©ART`/`©alb`/`aART`/`trkn`/`disk`/`©day`), the `covr` atom, and the `©lyr` atom.
- Add an on-demand `TagFile` usecase that, given an already-identified and (optionally) enriched tracked file, reads its resolved metadata/cover art/lyrics from the tracking store and writes them into the physical file in place, following the same background-job pattern as `EnrichManager`/`IdentifyManager` (`POST`/`GET /api/v1/library/tag`), triggered per-selection from the web UI — never automatically.
- Persist a per-file "tagged" outcome (status + optional error) in the tracking store so the UI can show which files have been tagged and re-tagging isn't silently redone or lost on refresh.
- Add a "Tag Selected" bulk action to the web UI, parallel to the existing "Identify Selected"/"Enrich Selected" actions, with a per-row tagged indicator.
- Writing tags is destructive to the file's existing embedded tags for the fields listed above (they are overwritten with MusicBrainz-resolved values); all other frames/atoms/comments not covered above are preserved as-is.
- Add a `GET /api/v1/library/tags` endpoint and a corresponding details-view section that read a tagged file's *actual* embedded tags directly from disk — independent of the resolved metadata cached in the tracking store — so a user can visually confirm what was really written, rather than trusting a "write didn't error" flag alone.
- Determine which tag format to write/read (ID3v2, Vorbis, or MP4 atom) by sniffing the file's real content, not by trusting its `.mp3`/`.flac`/`.m4a` extension — caught during real-file verification, where a user file with a `.mp3` extension turned out to actually be an MP4 container, which would otherwise have silently received tags no real player would ever see.

**Explicitly out of scope**: physically relocating/renaming files into the `Artist/Album/Track - Title` directory structure (charter §4.3/§1.3 Phase C step 7) — that remains a separate future change. This change only writes tags to files at their current path.

**Known limitation, accepted for this change**: for a file whose extension doesn't match its real container format (e.g. an M4A saved with a `.mp3` name), tagging correctly writes into the file's *real* format — but an extension-trusting consumer (notably macOS Finder/Spotlight, confirmed via `mdls`) will still look for tags matching the file's extension and find nothing, showing no metadata or cover art for that file. The only complete fix is renaming the file's extension to match its real content, which is out of scope here (see above). A user hitting this must rename the file themselves for now.

## Capabilities

### New Capabilities
- `audio-tag-writing`: On-demand writing of resolved artist/album/title/track metadata, cover art, and lyrics into the physical ID3v2 (MP3), Vorbis comment (FLAC), or MP4 atom (M4A) tags of an already-identified tracked file, in place at its current path; and reading a file's actual embedded tags back, independent of the tracking store, for verification.

### Modified Capabilities
- `file-tracking-store`: New persisted per-file tagging outcome (tagged/failed + error), alongside the existing identification/enrichment fields, following the same "record outcome without disturbing other fields" pattern as `RecordCoverArt`/`RecordLyrics`.
- `music-library-scan`: `GET /api/v1/library` gains a tagged indicator; new `POST`/`GET /api/v1/library/tag` and `GET /api/v1/library/tags` endpoints; web UI gains a "Tag Selected" bulk action, per-row tagged indicator, and an embedded-tags section in the details view. The capability's purpose statement drops "no tagging" (file relocation remains out of scope).

## Impact

- New code: `internal/infrastructure/filestat/taglib_tagger.go`; `internal/usecases/tag_file.go`, `tag_manager.go`; a new `Tagger` port in `internal/usecases/ports.go`; `internal/infrastructure/web/v1/tag_handler.go`.
- New dependency: `go.senan.xyz/taglib` (`github.com/sentriz/go-taglib`), a Wasm-embedded binding to TagLib run via `github.com/tetratelabs/wazero` — no CGO, `CGO_ENABLED=0` is preserved (see design.md).
- Schema: new `tagged`/`tag_error` columns on the tracking store, via the existing idempotent migration pattern.
- Three new API endpoints (`POST`/`GET /api/v1/library/tag`, same shape as the existing enrich trigger/status pair, plus `GET /api/v1/library/tags`); no changes to identify/enrich/upload endpoints.
- Filesystem: this change writes to files under the mounted `/music` volume for the first time (previously read-only) — every write path must be exercised against real files as part of verification, with a backup/dry-run consideration flagged for design.
