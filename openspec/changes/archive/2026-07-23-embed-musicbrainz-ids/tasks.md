## 1. Write MusicBrainz IDs into file tags

- [x] 1.1 Add `RecordingMBID`, `ReleaseMBID`, `ReleaseGroupMBID`, `ArtistMBID` string fields to `TagInput` in `internal/usecases/ports.go`
- [x] 1.2 Populate those four fields from the tracked record in `TagFile.Tag` (`internal/usecases/tag_file.go`), alongside the fields it already copies
- [x] 1.3 In `TagLibTagger.Tag` (`internal/infrastructure/filestat/taglib_tagger.go`), add `setIfNonEmpty` calls writing `taglib.MusicBrainzTrackID`, `taglib.MusicBrainzAlbumID`, `taglib.MusicBrainzReleaseGroupID`, `taglib.MusicBrainzArtistID` from the four new `TagInput` fields

## 2. Read MusicBrainz IDs back from file tags

- [x] 2.1 Add the same four MBID fields to `EmbeddedTags` in `internal/usecases/ports.go`
- [x] 2.2 In `TagLibTagger.ReadEmbeddedTags`, read `taglib.MusicBrainzTrackID`/`MusicBrainzAlbumID`/`MusicBrainzReleaseGroupID`/`MusicBrainzArtistID` back into the new `EmbeddedTags` fields
- [x] 2.3 (found during verification) `EmbeddedTagsResponse` (`internal/infrastructure/web/v1/embedded_tags_handler.go`) and `EMBEDDED_TAG_FIELD_LABELS` (`ui/src/format.js`) had their own separate field lists that didn't include the four MBIDs — the spec's "visually verified" language means the API/Details view, not just the internal struct. Added the four fields to both.

## 3. Identify from embedded tags during the analysis pass

- [x] 3.1 Add a `detectIdentificationFromTags(ctx, path, rec) (changed bool)` method to `AnalysisManager` (`internal/usecases/analysis_manager.go`) that calls `m.tagger.ReadEmbeddedTags`, and returns false (no-op) unless the embedded recording MBID is non-empty, artist and title are non-empty, and the embedded recording MBID differs from `rec.RecordingMBID`
- [x] 3.2 When those conditions hold, build an `IdentificationResult` from the embedded tags and call `m.store.RecordIdentification`, then call `m.store.RecordTagged(ctx, path, true, "")`, and return true
- [x] 3.3 In `AnalysisManager.analyzeOne`, call `detectIdentificationFromTags` first; if it returns true, re-fetch the tracked record from the store before running `fingerprint`, `detectEmbeddedContent`, and `detectRelocated`, so they see the file's freshly-identified state within the same pass
- [x] 3.4 Confirm (by reading `RecordIdentification`'s implementation, not assuming) that this conditional-call approach is the only path that reaches it from the analysis pass — i.e. the pass never calls `RecordIdentification` when the embedded recording ID is absent or already matches, so `tagged`/`relocated` are never reset on a pass where nothing changed

## 4. Verification

- [x] 4.1 Tag an identified file (with resolved recording/release/release-group/artist MBIDs) via "Tag Selected"; confirm via the Details view's embedded-tags section (or a raw tag read) that all four IDs are present, using the correct representation per format (MP3: `UFID` for recording ID, `TXXX` for the rest; FLAC: `MUSICBRAINZ_*` Vorbis comments; M4A: `----` freeform atoms)
- [x] 4.2 Wipe (or fabricate an untracked-equivalent state for) a previously-tagged file's tracking record, trigger a refresh, and confirm the automatic analysis pass re-identifies it from its own embedded tags without any AcoustID/MusicBrainz calls, and marks it `tagged`
- [x] 4.3 Resolve an identified file to a different candidate without re-tagging it, trigger a refresh, and confirm the next analysis pass reverts the tracking store back to what's actually embedded in the file (the intended, documented trust-model behavior — not a bug)
- [x] 4.4 Confirm a file whose embedded recording ID already matches its tracked state is left completely unchanged by repeated analysis passes — `tagged`/`relocated`/cover art/lyrics do not reset or flicker across passes
- [x] 4.5 Confirm a file identified from its own tags mid-pass, that also happens to already sit at its canonical relocation destination, gets marked `relocated` within that same pass (not requiring a second refresh)
- [x] 4.6 Confirm a file with no embedded MusicBrainz recording ID (the common case for anything not yet touched by this app or Picard) is completely unaffected by this change
