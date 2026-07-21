## Why

A real false positive was found in production: a Daft Punk "Get Lucky (Radio Edit)" file's fingerprint matches AcoustID with a genuinely high confidence score (0.999) — the audio match itself is not in doubt — but that one fingerprint is tied in AcoustID's database to five different MusicBrainz recordings (the official Daft Punk feat. Pharrell Williams single, plus several compilation-series reissues that reuse the same master, e.g. an unrelated "Walt Ribeiro — Every Song!" listing). `IdentifyFile.Identify` blindly takes the first recording AcoustID happens to list for the top result, so it silently accepted the wrong one. The just-implemented `improve-match-quality` change's confidence threshold cannot catch this: it operates on the AcoustID *match* score, which is correctly high here — the ambiguity is about *which of several tied recordings* is the right one, an orthogonal problem `improve-match-quality`'s design explicitly deferred as a non-goal.

## What Changes

- `AcoustIDClient.Lookup` (`internal/infrastructure/gateways/acoustid_client.go`) stops flattening every result's tied recordings into one score-tagged list — it groups each result's recordings together, so a caller can tell "one recording for this result" apart from "several distinct recordings tied to the same audio."
- `IdentifyFile.Identify`: when the top-scoring AcoustID result ties two or more *distinct* recordings, it resolves each recording's candidate metadata via MusicBrainz and records the file with a new `ambiguous` status and its stored candidate list, instead of auto-picking the first one. A single recording per top result (today's normal, unambiguous case) is completely unaffected.
- A new resolve action lets a user pick which stored candidate is correct for an `ambiguous` file; picking one records it exactly like a normal successful identification (same fields, same downstream tagging/relocation eligibility).
- `GET /api/v1/library`'s `status` filter gains `ambiguous` as a new valid value, alongside `new`/`identified`/`not_found`/`missing`.
- The web UI shows a distinct indicator for `ambiguous` rows and a way to open the candidate list and resolve one, mirroring how identified rows already show resolved metadata.
- Tied-recording candidates are drawn from every AcoustID result at or above the confidence threshold, not just the single best-scoring one — a genuinely valid match can land in a lower-scoring result rather than being tied into the top one.
- A new cover-art browsing feature lets a user pick a better front cover than the one automatically chosen: for an identified file, the system lists front-cover images across its release-group's sibling editions (a heavily-reissued album can have many, each potentially uploaded with different art) and lets the user choose one, which is downloaded and recorded exactly like a normal enrichment.
- **BREAKING (internal only)**: `AcoustIDLookup.Lookup`'s return shape changes from a flat `[]AcoustIDMatch` to a form that preserves per-result grouping; `AcoustIDClient` is the only implementation and `IdentifyFile` the only caller, both updated in this change.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `acoustid-lookup`: the lookup result preserves which recordings are tied together under the same matching result, instead of losing that grouping.
- `file-tracking-store`: a new `ambiguous` identification status is recorded when a top AcoustID result ties multiple distinct recordings; candidate metadata is stored per ambiguous file; a resolve action lets a user pick a candidate and records it exactly like a normal successful identification.
- `music-library-scan`: `GET /api/v1/library`'s `status` filter accepts `ambiguous`; new endpoints to list an ambiguous file's stored candidates and to resolve one; new endpoints to list and choose cover-art candidates across a release-group's sibling editions; the web UI surfaces ambiguous files, a candidate picker, and a cover-browsing picker.
- `cover-art-lookup`: gains the ability to list a single release's front-cover image without downloading it, and to download an explicitly-chosen image URL (validated against Cover Art Archive's own host) — used to browse and pick among sibling releases' covers, separate from the existing single automatic choice.
- `musicbrainz-metadata`: gains the ability to resolve a release-group's sibling releases, used to enumerate candidates for cover-art browsing.

## Impact

- Changed code: `internal/infrastructure/gateways/acoustid_client.go` (group tied recordings per result), `internal/usecases/ports.go` (grouped AcoustID result shape, new store methods for recording/resolving ambiguity, release-group/cover-browsing ports), `internal/usecases/identify_file.go` (tied-recording branch across every qualifying result, resolves each candidate's metadata via MusicBrainz before recording), `internal/usecases/browse_cover_art.go` (new usecase for listing/choosing cover-art candidates), `internal/domain/tracking.go` (new `StatusAmbiguous`, candidate storage shape), `internal/infrastructure/persistence/sqlite_store.go` (schema change to store candidates, new store methods), `internal/infrastructure/gateways/musicbrainz_client.go` (new release-group sibling-release lookup), `internal/infrastructure/gateways/coverart_client.go` (new front-image-listing and validated-download methods), `internal/infrastructure/web/v1/` (new handlers for listing/resolving recording candidates and listing/choosing cover candidates, `status` filter validation), `ui/index.html` / `ui/js/app.js` (ambiguous indicator, recording-candidate picker, cover-browsing picker).
- Schema change: yes — a new place to store an ambiguous file's candidate list is needed (exact shape, e.g. a new table vs. a JSON column, is a design decision for `design.md`). No schema change for cover-art browsing — chosen covers are stored via the existing `CoverArtStore`/`cover_art_path` mechanism.
- Resolving a candidate reuses the existing `identified` recording path (same fields written), so tagging/relocation/enrichment need no changes — they already operate on any `identified` file regardless of how it got there. Choosing a browsed cover reuses the existing `RecordCoverArt` path the same way.
- Independent of the already-archived `speed-up-library-scan` and `improve-match-quality` changes; builds on the confidence-threshold and fingerprint-lookup code both of those already touched in `internal/usecases/identify_file.go` and `internal/infrastructure/gateways/acoustid_client.go`.
