## Why

Scanning a real library (2,800 files) took roughly 10 minutes, because `ScanLocalVolume` invokes the Chromaprint fingerprinter (`fpcalc`, a full audio decode) on every new or changed file during the disk walk — even though the only reason to look at the library list is to see path/format/duration/status, and a fingerprint is only actually needed once the user triggers Identify on that file. The fix is to stop paying that cost during scan and only pay it when it's actually needed.

## What Changes

- `ScanLocalVolume` no longer computes a Chromaprint fingerprint for any file. It still needs each file's duration (shown in the library list and used to sanity-check identification later), which it now gets from a cheap TagLib audio-properties read (already a dependency, already used for tag reading/writing) instead of as a byproduct of fingerprinting.
- Fingerprinting moves to identify-time: when `POST /api/v1/library/identify` processes a path with no stored fingerprint yet, it computes one on the spot (persisting it so future identify attempts don't recompute it), then proceeds to the AcoustID/MusicBrainz lookup as before. **BREAKING (internal only)**: `IdentifyManager` currently skips any path with no stored fingerprint ("has no usable fingerprint, skipping") — that skip is removed; those paths are now processed instead.
- A file whose content changes (detected the same way as today, via size/mtime) has its previously stored fingerprint invalidated along with its identification status, since a stale fingerprint from the old content must never be reused against the new content.
- If a file can't be fingerprinted at identify-time (e.g. corrupt audio), that one file is skipped with a stored error, exactly mirroring today's per-file-failure tolerance during scan — the rest of the batch continues.
- Per-file duration-read failure during scan takes over the tolerance role a fingerprint failure used to play (skip that file, report an error, keep scanning the rest) — same shape, different underlying operation.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `music-library-scan`: the background refresh no longer fingerprints; `GET /api/v1/library`'s per-row `error` field now reflects a duration-read failure instead of a fingerprint failure; `POST /api/v1/library/identify` gains lazy, on-demand fingerprinting for paths that don't have one yet; `GET /api/v1/library/fingerprint` gains a defined response for a tracked file that hasn't been fingerprinted yet (distinct from an untracked path).
- `file-tracking-store`: change detection during a refresh derives duration instead of a fingerprint for new/changed files, and clears a changed file's stale fingerprint; a new requirement covers fingerprinting happening lazily as part of identification instead of as part of a refresh.

## Impact

- Changed code: `internal/usecases/scan_local_volume.go` (drop `Fingerprinter` dependency, add a duration-reading dependency), `internal/usecases/identify_file.go` and `identify_manager.go` (compute-on-demand fingerprinting, mirroring `TagFile.Tag`'s self-loading pattern), `internal/infrastructure/persistence/sqlite_store.go` (a new store method to persist a just-computed fingerprint/duration/error independently of a full scan's `BulkApply`), `internal/infrastructure/filestat/` (a new lightweight duration-reader backed by `taglib.ReadProperties`, reusing the existing `withCorrectExtension` mislabeled-extension protection).
- `cmd/server/main.go`: wiring changes for the above (scan no longer takes a `Fingerprinter`; identify does).
- No schema changes — `fingerprint`, `fingerprint_error`, and `duration_seconds` are all existing columns; this only changes when/how they get populated.
- No API contract changes to `GET /api/v1/library`'s shape (still the same fields), only to what causes its `error` field to be set and what triggers `POST /api/v1/library/identify` to do more work per file than before.
