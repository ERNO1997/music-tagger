## Why

We already fetch a MusicBrainz recording response containing far more than the artist/album/title/track number we currently keep — album artist, release year, disc/track counts, and the release/release-group/artist MBIDs are all sitting in the same payload, just discarded. Meanwhile the UI only shows a terse one-line summary per row, with no way to see a file's full resolved picture without digging into the database directly. This change captures the metadata we're already fetching for free and gives it somewhere to be seen.

## What Changes

- Extend `domain.FileRecord` + the SQLite schema (via the same idempotent migration pattern used for the previous metadata columns) with: `AlbumArtist`, `Year`, `DiscNumber`, `TotalDiscs`, `TotalTracks`, `ReleaseMBID`, `ReleaseGroupMBID`, `ArtistMBID`.
- Extend `musicbrainz_client.go`'s parsing to capture these fields from the same API response already being fetched — no new `inc` parameters, no new API calls, no new gateway.
- Extend `GET /api/v1/library`'s entries to include the new fields (omitted when not yet identified).
- Add a details view to the web UI: clicking a row opens a panel/modal showing the file's full metadata — all resolved fields plus path, format, duration, fingerprint, status, and any error. Rendered entirely from data already fetched by `GET /api/v1/library` — no new endpoint needed, since the client already has the full record in hand.

**Explicitly out of scope**: language, ISRC, genre, composer — previously discussed as either needing extra API calls or being niche; can be their own future change.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `musicbrainz-metadata`: resolves additional fields (album artist, year, disc number, total discs, total tracks, release MBID, release-group MBID, artist MBID) from the existing recording lookup.
- `file-tracking-store`: persists the additional resolved metadata fields.
- `music-library-scan`: `GET /api/v1/library` entries include the new fields; web UI gains a per-file details view.

## Impact

- Schema migration: more idempotent `ALTER TABLE` columns, same pattern as the previous metadata migration.
- Extended Go structs (`domain.FileRecord`, `usecases.RecordingMetadata`, `v1.LibraryEntry`) and MusicBrainz JSON parsing in `musicbrainz_client.go`.
- New UI: a details modal/panel + row click handler in `app.js`/`index.html`, no new backend endpoint.
- No new external dependencies, no new gateway calls.
