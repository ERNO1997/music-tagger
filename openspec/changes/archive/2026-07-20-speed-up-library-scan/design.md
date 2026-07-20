## Context

`ScanLocalVolume.Refresh` (`internal/usecases/scan_local_volume.go`) walks `/music`, classifies each file as new/changed/unchanged/missing by comparing size+mtime against the tracking store, and — for every new or changed file — calls `Fingerprinter.Fingerprint(ctx, path)` (backed by `FpcalcRunner`, which shells out to `fpcalc`) to get both a Chromaprint acoustic fingerprint and the file's duration in one call. That fingerprint is stored but not used again until the user triggers Identify, which reads it back out of the tracking store and sends it to AcoustID. `IdentifyManager.Start` (`internal/usecases/identify_manager.go:50`) currently skips any path whose stored fingerprint is empty ("has no usable fingerprint, skipping") — under today's design this branch is unreachable in practice once a scan has run, since scan always fingerprints. `taglib.ReadProperties` (already used by `TagLibTagger.ReadEmbeddedTags` for cover-art presence) returns a `Properties.Length time.Duration` field sourced from the file's own container headers — a cheap read, nothing like a full Chromaprint decode.

## Goals / Non-Goals

**Goals:**
- A scan refresh no longer runs `fpcalc` at all; duration comes from a cheap TagLib header read instead.
- Fingerprinting happens lazily, exactly once per file's current content, the first time that file is actually submitted to Identify.
- A file whose content changes never has a stale fingerprint (from its old content) reused against AcoustID.
- Per-file failure tolerance is preserved in both directions: a scan's duration-read failure doesn't abort the refresh; an identify job's fingerprint-computation failure doesn't abort the batch.

**Non-Goals:**
- Changing anything about the AcoustID/MusicBrainz lookup flow itself, or the confidence-threshold work being done in the separate `improve-match-quality` change.
- Changing the `files` table schema — `fingerprint`, `fingerprint_error`, `duration_seconds` all already exist; this only changes when/how they're populated.
- Caching or pre-warming fingerprints in the background ahead of an explicit Identify request — deliberately lazy, computed only when needed.

## Decisions

### Duration comes from `taglib.ReadProperties`, not `fpcalc`
`Properties.Length` is populated from the file's own container headers (MP3 Xing/VBRI frame, FLAC `STREAMINFO`, MP4 `moov` atom) — no audio decode, no acoustic fingerprint computation. This is the same `go.senan.xyz/taglib` dependency already used for tag reading/writing, just a different call on it. Alternative considered: keep computing duration from `fpcalc` but run it in more parallel workers to speed up scan — rejected, since it still pays the actual expensive cost (decoding audio) for every file on every scan, just distributed across more goroutines; the real fix is to stop needing that work at scan time at all.

Like `TagLibTagger.Tag`/`ReadEmbeddedTags`, this new duration read goes through the existing `withCorrectExtension` helper (`internal/infrastructure/filestat/format_detect.go`) so a mislabeled-extension file (the bug fixed during the tag-writing change) gets its duration read against its real, content-sniffed format rather than a wrong one.

### Fingerprinting moves into `IdentifyFile.Identify`, mirroring `TagFile.Tag`'s self-loading pattern
`IdentifyFile.Identify` changes from `Identify(ctx, path, fingerprint string, durationSeconds float64) error` to `Identify(ctx, path string) (skipped bool, err error)` — it loads the tracked record itself via `store.Get(ctx, path)` (same shape as `TagFile.Tag`), checks whether a fingerprint is already stored, and:
- if present, uses it directly (no re-fingerprinting — the whole point of persisting it);
- if absent, calls `Fingerprinter.Fingerprint(ctx, path)` itself, and on success persists the fingerprint+duration via a new store method before proceeding to AcoustID;
- if fingerprinting fails, persists the failure reason and returns `skipped=true` rather than an error, so `IdentifyManager` logs and moves on exactly as it already does for an unknown path — no new error-handling shape needed in the manager.

`IdentifyFile` gains a `fingerprinter Fingerprinter` constructor dependency (the same port `ScanLocalVolume` used to hold). `IdentifyManager.Start` is simplified: it drops its own `rec.Fingerprint == ""` branch entirely (that check moves inside `Identify`) and just calls `m.identify.Identify(ctx, path)` for every path — the "unknown path" and "no usable fingerprint" cases both become the same kind of self-detected skip inside `Identify`.

Alternative considered: keep fingerprinting inside `IdentifyManager.Start` (which already does one `LoadAll` per job) rather than inside `IdentifyFile.Identify`. Rejected — it would duplicate the skip/error-recording logic that `Identify` already needs for its own AcoustID-not-found and gateway-error paths, and would break the existing convention (established by `TagFile`/`RelocateFile`) that each `*File` usecase owns its own record-loading and skip logic rather than relying on its manager to pre-filter.

### A changed file's fingerprint is invalidated the same way its identification is
`ScanLocalVolume`'s existing "changed file" branch already resets a file's status to `new` and (via `RecordIdentification`'s existing re-identification-invalidation behavior) clears prior resolved metadata. This change extends that: a changed file's `fingerprint` and `fingerprint_error` are cleared too, in the same `BulkApply` commit, so the next Identify attempt is forced to recompute against the file's new content — never silently reusing a fingerprint computed against the old bytes at that path.

### New store method: `RecordFingerprint`
Mirrors `RecordTagged(ctx, path, tagged bool, tagErr string)`'s dual-outcome shape: `RecordFingerprint(ctx, path string, fingerprint string, durationSeconds float64, fingerprintErr string) error`, a single `UPDATE` touching only `fingerprint`, `duration_seconds`, and `fingerprint_error`, called once by `IdentifyFile.Identify` after it computes (or fails to compute) a fingerprint on demand. `BulkApply`'s existing per-file upsert (used by scan) is left untouched aside from no longer setting `fingerprint`/`duration_seconds` from a `Fingerprinter` result — it now takes duration from the new TagLib-backed reader and leaves `fingerprint` blank on insert (cleared on update, per the change above).

### `GET /api/v1/library/fingerprint` for a tracked-but-not-yet-fingerprinted file
Returns `200 OK` with an empty `fingerprint` string — the path is genuinely tracked, it just hasn't been fingerprinted yet (identical in spirit to `GetLyrics`/`GetCoverArtPath`'s existing `found=false`-but-still-200-for-a-tracked-path convention elsewhere in this API). Only a truly untracked path gets `404`. Alternative considered: `404` for "not yet fingerprinted" too — rejected, since that would make the endpoint indistinguishable from "this path doesn't exist in the tracking store at all," which is a materially different situation for a client to react to.

## Risks / Trade-offs

- **[Risk] A corrupt file's fingerprint failure no longer surfaces immediately after a scan — only when the user tries to Identify it** → Accepted: the user already has to explicitly trigger Identify to do anything with a file, so the failure surfaces at the point it actually matters, just later than before. The `GET /api/v1/library`'s `error` field still surfaces a *duration*-read failure at scan time, which already catches most "this file is unreadable/corrupt" cases — a file that reads fine for TagLib properties but somehow fails Chromaprint specifically is a narrow edge case.
- **[Risk] Identify jobs get marginally slower per file** (fingerprinting now happens inline, where it used to already be done) → Accepted: MusicBrainz's 1 req/sec gate already dominates identify's pace (project.md §4.2); a local fingerprint computation taking a fraction of a second is negligible against that existing floor.
- **[Trade-off] `IdentifyFile.Identify`'s signature changes** (drops the `fingerprint`/`durationSeconds` parameters) → Internal-only break, no external API contract; `IdentifyManager` is the only caller and is updated in the same change.

## Migration Plan

- No schema migration needed — `fingerprint`, `fingerprint_error`, `duration_seconds` are already columns.
- Existing tracked files that already have a stored fingerprint (from a scan under the old behavior) are unaffected — `Identify` only computes a fresh one when the stored value is empty, so no work is redone for already-fingerprinted files.
- No rollback concern beyond reverting the code: nothing about the stored data becomes invalid under the old code path if this change is reverted.
