## Why

The Artist-Album view groups purely by string equality on artist/album name, even though identified files already carry `ArtistMBID`/`ReleaseGroupMBID`. This causes two real problems: different artists that happen to share a name string get merged into one group, and the same artist under slightly different tag spellings/aliases gets split into separate groups. Separately, now that MBIDs are available, the library can compare itself against MusicBrainz's actual catalog to tell the user what's missing from an artist's discography or an album's tracklist — something it has no way to do today.

## What Changes

- Artist grouping keys on `artist_mbid` when present (falling back to name for unidentified files); album grouping keys on `release_group_mbid` scoped within the artist group, same fallback rule.
- Since an MBID has no display string, each group picks a representative label (most-frequent non-blank name observed in that group).
- **New**: mismatch detection — if a group's members disagree (same MBID but different name strings, or same name string but different MBIDs), the group is flagged with a visible warning indicator rather than silently merged or resolved by picking one value. Detail of the disagreement is available on inspection.
- **New**: `MusicBrainzClient` gains an artist discography lookup (all release-groups for an artist MBID) and a release tracklist lookup (full track list for a release, reusing the existing release-selection heuristic).
- **New**: a completeness check comparing the local library's tracks/albums against MusicBrainz's for the current artist or album — "M/N tracks" on an album, "M/N albums" on an artist, with the missing ones listed. Runs automatically when a user drills into an artist or album, and is also available as a manual re-check action.
- Completeness checks are strictly on-demand per artist/album (never eager across a full artist/album list), respecting the existing 1 req/sec MusicBrainz rate gate.
- Completeness only considers official Album/EP release-groups by default (singles, live albums, compilations, and bootlegs excluded from "missing" counts) to avoid noise.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `library-browsing`: Artist/album grouping changes from name-string equality to MBID-first grouping with mismatch flagging; new completeness-check requirement added for the Artist-Album view.
- `musicbrainz-metadata`: Two new lookups added — artist discography (release-groups for an artist) and release tracklist (recordings for a release) — both subject to the existing centralized rate limit.

## Impact

- `internal/infrastructure/persistence/sqlite_store.go`: `ListArtists`/`ListAlbums`/`ListTracks` grouping logic.
- `internal/infrastructure/gateways/musicbrainz_client.go`: new `ArtistReleaseGroups` and `ReleaseTracklist` (naming TBD in design) methods.
- `internal/usecases/`: new port(s) for completeness checking, likely a new use case orchestrating the gateway + store comparison.
- `internal/infrastructure/web/v1/artist_album_handler.go` and `router.go`: mismatch flags in existing responses, new endpoint(s) for completeness checks.
- `ui/src/components/views/ArtistAlbumGroupingView.vue`: mismatch warning indicators, completeness UI (progress/missing list), manual re-check action.
- No changes to the folder tree view or other browsing modes.
