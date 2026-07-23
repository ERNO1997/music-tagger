## Context

Today, `recording_mbid`/`release_mbid`/`release_group_mbid`/`artist_mbid` exist only as SQLite columns (`internal/infrastructure/persistence/sqlite_store.go`), populated by `RecordIdentification` once AcoustID/MusicBrainz resolve a file. `TagLibTagger.Tag` (`internal/infrastructure/filestat/taglib_tagger.go`) never writes them — it writes title/artist/album/album-artist/track/disc/year/lyrics/cover art only. The Go TagLib binding this project already depends on (`go.senan.xyz/taglib`) exposes ready-made normalized keys for exactly this: `MusicBrainzTrackID` (recording), `MusicBrainzAlbumID` (release), `MusicBrainzReleaseGroupID`, `MusicBrainzArtistID`. Confirmed by actually writing them to real MP3/FLAC/M4A fixtures and inspecting the raw bytes (not just reading the binding's source):

- **MP3**: recording ID → a `UFID` frame with owner `http://musicbrainz.org` (the same frame Picard uses — not a `TXXX` frame, since ID3v2 has a dedicated identifier frame for exactly this). The other three IDs → `TXXX` frames (e.g. `TXXX:MUSICBRAINZ ALBUM ID`, `TXXX:MUSICBRAINZ ARTIST ID`).
- **FLAC**: plain Vorbis comment fields — `MUSICBRAINZ_TRACKID`, `MUSICBRAINZ_ALBUMID`, `MUSICBRAINZ_RELEASEGROUPID`, `MUSICBRAINZ_ARTISTID`.
- **M4A**: freeform `----` atoms under the `com.apple.iTunes` mean, named atoms like `MusicBrainz Album Id` — the same scheme Picard uses for MP4.

All three round-trip correctly through `taglib.WriteTags`/`taglib.ReadTags` with no new dependency.

The other load-bearing fact: `SQLiteStore.RecordIdentification` (`sqlite_store.go:490-547`) **unconditionally** resets `cover_art_path`/`lyrics`/`synced_lyrics`/`tagged`/`relocate_error`/`relocated` to blank on every call where `result.Status == identified` — it doesn't compare against the record's current values first. That's correct and safe for the existing caller (the on-demand identify use case, which is only ever invoked to deliberately (re-)identify a file), but it means a new caller that invokes `RecordIdentification` on every analysis pass — as this change's "every scan, for every file" requirement calls for — **must not call it unconditionally**, or it will wipe `tagged`/`relocated` back to false on every single pass even when nothing changed. This is the central implementation risk this design addresses.

## Goals / Non-Goals

**Goals:**
- Make identification durable: a file's own tags carry enough to reconstruct its `identified` state without AcoustID/MusicBrainz, surviving a full tracking-store loss.
- Treat the file's embedded tags as the source of truth once a recording ID is present — deliberately, per the user's explicit decision — overriding the database if the two disagree, on every analysis pass, for every file.
- Reuse the existing `RecordIdentification`/`RecordTagged` store methods and the existing `background-library-analysis` pass rather than introducing new mechanisms.

**Non-Goals:**
- Verifying an embedded recording ID against AcoustID/MusicBrainz before trusting it — explicitly rejected in favor of unconditional trust (see prior conversation).
- Any UI surfacing of a "DB and file tags disagree" state — out of scope here; re-tagging after any resolve/re-identify is the expected corrective action, not something the UI needs to flag (though a future change could add a mismatch indicator).
- Writing `album_artist` MBID or `work` MBID — the domain model doesn't currently resolve or store those, so there's nothing to write; only the four IDs already in `domain.FileRecord` are in scope.

## Decisions

### Extend `TagInput`/`EmbeddedTags` with the four MBID fields, not a new struct
`internal/usecases/ports.go`'s `TagInput` (write) and `EmbeddedTags` (read-back) both gain `RecordingMBID`, `ReleaseMBID`, `ReleaseGroupMBID`, `ArtistMBID` string fields, mirroring the naming already used on `domain.FileRecord`. `TagFile.Tag` (`tag_file.go`) populates them from the tracked record exactly like every other field it already copies across. `TagLibTagger.Tag`/`ReadEmbeddedTags` gain four more `setIfNonEmpty`/read calls using the TagLib constants above. No new port method, no new struct — this is the same shape of change as every other field already flowing through this path.

### The identify-from-tags check reuses `ReadEmbeddedTags`, not a new read path
`AnalysisManager` already holds a `tagger Tagger` field and already calls `m.tagger.ReadEmbeddedContent` for cover/lyrics detection. The new step calls the same (now-extended) `m.tagger.ReadEmbeddedTags(ctx, path)` already used for the Details view's tagged-file comparison — one more field read off a struct that already exists, not a new capability on the `Tagger` interface.

### Minimum trust bar: recording MBID + artist + title
A file's embedded tags are only trusted enough to (re-)identify it when all three of `RecordingMBID`, `Artist`, and `Title` are non-empty — recording MBID because it's the field that actually identifies a specific recording (and the one MusicBrainz/Picard convention centers this scheme on), artist and title because an "identified" record with neither would be useless everywhere else in the app (metadata-completeness display, relocation's destination path, etc.). Album, track number, disc/year/the other three MBIDs are read and stored when present but don't gate whether the file is trusted — consistent with how AcoustID-driven identification already tolerates a missing release year.

### Only call `RecordIdentification` when the embedded recording ID actually differs from what's stored
This is the fix for the idempotency risk described in Context. The new analysis step:
1. Reads the file's embedded tags.
2. If no recording MBID (or no artist/title) is present, does nothing — leaves the tracked record exactly as-is, regardless of current status.
3. If a recording MBID is present and equals the tracked record's already-stored `RecordingMBID` (both non-empty and matching), does nothing — the file already reflects its own tags, no need to touch tagged/relocated/enrichment state.
4. Only when the embedded recording MBID is non-empty and *differs* from the currently-stored one (including the case where the record isn't `identified` at all yet, i.e. currently empty) does it call `store.RecordIdentification` with an `IdentificationResult` built from the embedded values — at which point the existing, unmodified `RecordIdentification` invalidation behavior (clearing stale cover art/lyrics/tagged/relocated/candidates) is exactly correct and desired, since the identity genuinely changed.

This means "every scan, for every file" is accurate for step 1 (the read always happens) without meaning "every scan force-resets every file" (the write is conditional on an actual disagreement) — reconciling the user's stated requirement with the store method's real semantics.

### A file identified from its own tags is marked `tagged`, via a separate `RecordTagged` call
Because `RecordIdentification` always writes `tagged = 0`, immediately after step 4 above succeeds, the analysis step calls `store.RecordTagged(ctx, path, true, "")`. This is accurate, not a workaround: the file's own tags already fully describe this identity (that's precisely why it was trusted), so there's nothing left for an on-demand "Tag Selected" to write that isn't already there. This also means such a file becomes immediately eligible for this same pass's existing relocated-detection step (which requires `identified && tagged`) — see ordering below.

### Ordering within `analyzeOne`: identify-from-tags runs first, and refreshes `rec` before the remaining steps
`AnalysisManager.analyzeOne` currently runs `fingerprint` → `detectEmbeddedContent` → `detectRelocated` against one in-memory `domain.FileRecord` snapshot per file. The new `detectIdentificationFromTags` step is inserted first, and — only when it actually wrote a change (step 4 above) — `analyzeOne` re-fetches the record from the store before running the remaining three steps, so a file identified this way is correctly seen as `identified`/`tagged` (and therefore checked for `detectEmbeddedContent`'s cover/lyrics backfill and `detectRelocated`'s canonical-path check) within the very same pass, rather than only catching up on the next refresh cycle.

### No change to `file-tracking-store`'s spec
`RecordIdentification`'s invalidation behavior is already implemented once and shared by every caller. Its existing scenarios ("Re-identification invalidates prior enrichment/tagged/relocated outcome") are worded generically around "identified again," not "identified again via the on-demand identify action" — so they already, correctly, describe what happens when this new caller triggers a change. Editing that spec would be re-describing existing, unchanged behavior.

## Risks / Trade-offs

- **[Risk] A hand-edited or miscopied embedded MusicBrainz tag silently misidentifies a file, with no verification against AcoustID** → Accepted, per the user's explicit decision: the file's own tags are the deliberate source of truth here, not a hint to be double-checked.
- **[Trade-off] A resolve/re-identify that isn't followed by re-tagging will be silently reverted by the next analysis pass** → Accepted, per the user's explicit decision: "Tag Selected" is the corrective action; this is presented as the expected workflow, not a bug, and is called out in the proposal as a **BREAKING** behavior change.
- **[Risk] Forgetting the conditional check in Decision 4 and calling `RecordIdentification` unconditionally every pass** → Mitigated by making the comparison an explicit, named step in this design rather than leaving it to be discovered during implementation; this is the single most important detail to get right.
