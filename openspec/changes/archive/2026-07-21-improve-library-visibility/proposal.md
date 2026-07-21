## Why

Over a real ~2,600-file library, most files are still `new` — unidentified — and today the library list shows nothing for them beyond path, format, and duration. A file named `1647274517772140579I_WANNA_BE_YOUR_SLAVE-140_-_audio_only_me.m4a` gives no hint what it actually is, even though the file's own embedded tags (already readable via the existing `audio-tag-writing` capability's tag-reading path) often do — many downloaded/ripped files already carry a real title/artist/album in their ID3v2/Vorbis/MP4 tags regardless of whether this system has identified them yet. Separately, `has_lyrics`/`tagged`/`relocated` filters already exist but there's no way to filter to files missing cover art, the equivalent gap for the one remaining enrichment outcome.

## What Changes

- During a scan refresh, alongside the existing cheap TagLib duration read, the system also reads each new/changed file's own embedded title/artist/album/album-artist tags (no audio decode, same cost class as the duration read already performed) and persists them as a distinct "raw tag" snapshot, independent of resolved (AcoustID/MusicBrainz) metadata.
- `GET /api/v1/library`'s response includes each file's raw title/artist/album/album-artist when present, so an unidentified file's list row can show what it actually is instead of just its path.
- Free-text search (`q`) additionally matches these raw tag fields, so a poorly-named but already-tagged file becomes findable by its real title/artist even before identification.
- `GET /api/v1/library` gains a `has_cover_art` boolean filter, mirroring `tagged`/`relocated`/`has_lyrics` exactly.
- The web UI's table row and details view show the raw tag snapshot for unidentified files (visually distinguished from resolved metadata, since a raw tag can be wrong/incomplete in ways resolved metadata already isn't), and gain a "Cover: any/yes/no" filter control.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `file-tracking-store`: persists a per-file raw tag snapshot (title/artist/album/album-artist) captured during scan, independent of resolved metadata; gains `has_cover_art` as a filterable dimension.
- `music-library-scan`: `GET /api/v1/library` response and free-text search include raw tag fields; `has_cover_art` becomes a valid filter/query parameter; the web UI displays raw tags and gains a cover-art filter control.

## Impact

- Changed code: `internal/domain/tracking.go` (new `RawTitle`/`RawArtist`/`RawAlbum`/`RawAlbumArtist` fields), `internal/usecases/ports.go` (new lightweight raw-tag-reading port, `HasCoverArt` filter field), `internal/infrastructure/filestat/` (new or extended TagLib-backed raw tag reader), `internal/usecases/scan_local_volume.go` (capture raw tags alongside duration for new/changed files), `internal/infrastructure/persistence/sqlite_store.go` (new columns, WHERE-clause dimension, search clause extension), `internal/infrastructure/web/v1/library_handler.go` (response fields, query parameter), `ui/index.html`/`ui/js/app.js` (raw tag display, cover filter control).
- Schema change: yes — new columns on `files` for the raw tag snapshot, additive only (existing rows simply have them empty until next scan; no migration of historical data needed since nothing is lost, only newly captured going forward).
- No change to identification, tagging, or resolved-metadata behavior — this only adds a second, independent, pre-identification data source for display/search/filter, never written by or consulted during AcoustID/MusicBrainz resolution.
- Independent of the already-archived `speed-up-library-scan`, `improve-match-quality`, and `disambiguate-tied-recordings` changes; extends the same scan pass `speed-up-library-scan` already made cheap (TagLib-only, no `fpcalc`).
