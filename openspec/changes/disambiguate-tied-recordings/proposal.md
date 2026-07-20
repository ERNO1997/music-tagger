## Why

A real false positive was found in production: a Daft Punk "Get Lucky (Radio Edit)" file's fingerprint matches AcoustID with a genuinely high confidence score (0.999) — the audio match itself is not in doubt — but that one fingerprint is tied in AcoustID's database to five different MusicBrainz recordings (the official Daft Punk feat. Pharrell Williams single, plus several compilation-series reissues that reuse the same master, e.g. an unrelated "Walt Ribeiro — Every Song!" listing). `IdentifyFile.Identify` blindly takes the first recording AcoustID happens to list for the top result, so it silently accepted the wrong one. The just-implemented `improve-match-quality` change's confidence threshold cannot catch this: it operates on the AcoustID *match* score, which is correctly high here — the ambiguity is about *which of several tied recordings* is the right one, an orthogonal problem `improve-match-quality`'s design explicitly deferred as a non-goal.

## What Changes

- `AcoustIDClient.Lookup` (`internal/infrastructure/gateways/acoustid_client.go`) stops flattening every result's tied recordings into one score-tagged list — it groups each result's recordings together, so a caller can tell "one recording for this result" apart from "several distinct recordings tied to the same audio."
- `IdentifyFile.Identify`: when the top-scoring AcoustID result ties two or more *distinct* recordings, it resolves each recording's candidate metadata via MusicBrainz and records the file with a new `ambiguous` status and its stored candidate list, instead of auto-picking the first one. A single recording per top result (today's normal, unambiguous case) is completely unaffected.
- A new resolve action lets a user pick which stored candidate is correct for an `ambiguous` file; picking one records it exactly like a normal successful identification (same fields, same downstream tagging/relocation eligibility).
- `GET /api/v1/library`'s `status` filter gains `ambiguous` as a new valid value, alongside `new`/`identified`/`not_found`/`missing`.
- The web UI shows a distinct indicator for `ambiguous` rows and a way to open the candidate list and resolve one, mirroring how identified rows already show resolved metadata.
- **BREAKING (internal only)**: `AcoustIDLookup.Lookup`'s return shape changes from a flat `[]AcoustIDMatch` to a form that preserves per-result grouping; `AcoustIDClient` is the only implementation and `IdentifyFile` the only caller, both updated in this change.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `acoustid-lookup`: the lookup result preserves which recordings are tied together under the same matching result, instead of losing that grouping.
- `file-tracking-store`: a new `ambiguous` identification status is recorded when a top AcoustID result ties multiple distinct recordings; candidate metadata is stored per ambiguous file; a resolve action lets a user pick a candidate and records it exactly like a normal successful identification.
- `music-library-scan`: `GET /api/v1/library`'s `status` filter accepts `ambiguous`; new endpoints to list an ambiguous file's stored candidates and to resolve one; the web UI surfaces ambiguous files and a candidate picker.

## Impact

- Changed code: `internal/infrastructure/gateways/acoustid_client.go` (group tied recordings per result), `internal/usecases/ports.go` (grouped AcoustID result shape, new store methods for recording/resolving ambiguity), `internal/usecases/identify_file.go` (tied-recording branch, resolves each candidate's metadata via MusicBrainz before recording), `internal/domain/tracking.go` (new `StatusAmbiguous`, candidate storage shape), `internal/infrastructure/persistence/sqlite_store.go` (schema change to store candidates, new store methods), `internal/infrastructure/web/v1/` (new handler(s) for listing/resolving candidates, `status` filter validation), `ui/index.html` / `ui/js/app.js` (ambiguous indicator, candidate picker UI).
- Schema change: yes — a new place to store an ambiguous file's candidate list is needed (exact shape, e.g. a new table vs. a JSON column, is a design decision for `design.md`).
- Resolving a candidate reuses the existing `identified` recording path (same fields written), so tagging/relocation/enrichment need no changes — they already operate on any `identified` file regardless of how it got there.
- Independent of the already-archived `speed-up-library-scan` and `improve-match-quality` changes; builds on the confidence-threshold and fingerprint-lookup code both of those already touched in `internal/usecases/identify_file.go` and `internal/infrastructure/gateways/acoustid_client.go`.
