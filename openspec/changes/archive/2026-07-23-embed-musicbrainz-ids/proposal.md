## Why

MusicBrainz recording/release/release-group/artist IDs currently live only in the SQLite tracking store — never written to the physical file. That means identification is entirely dependent on the database: losing it (a wiped `/data` volume, a fresh container) means every file goes back to `new` and has to be re-fingerprinted and re-resolved via AcoustID/MusicBrainz from scratch, even though the file itself hasn't changed. Embedding these IDs into the file's own tags — something TagLib (already a dependency) supports out of the box — makes identification durable: the file carries its own identity, survives database loss, and can be recognized instantly without a network round-trip. The tradeoff is a trust-model decision: the file's own tags become authoritative over the database, which is a deliberate choice (confirmed with the user) rather than an incidental one.

## What Changes

- On-demand tagging (`Tag Selected`) now also writes the file's resolved MusicBrainz recording, release, release-group, and artist IDs into its own tags, alongside the metadata it already writes.
- Reading a file's embedded tags back (used today for the Details view's tagged-file comparison) now also returns these four IDs.
- **BREAKING** (new automatic behavior): the background analysis pass — which already runs automatically after every refresh — now also checks every tracked file's own embedded MusicBrainz recording ID on every pass, for every file, regardless of its current tracked status. When a recording ID (plus artist and title) is embedded, the system treats the file's own tags as authoritative: it sets (or resets) that file to `identified` with the embedded metadata, and marks it `tagged` — even overriding an already-`identified` database record if its embedded tags disagree. This only takes effect when the embedded recording ID actually differs from what's already stored; a file whose tags already match its tracked identity is left untouched, so this never resets progress on every pass. Since RecordIdentification's existing invalidation semantics apply regardless of caller, an identity change detected this way clears stale cover art/lyrics/relocated outcome exactly like any other re-identification — so a user who resolves a file differently without re-running "Tag Selected" will see the database revert to what's actually on disk on the next scan. That's the intended trust model: the file is the source of truth, and "Tag Selected" is what keeps it in sync.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `audio-tag-writing`: on-demand tagging also writes the four MusicBrainz IDs into the file's own tags (UFID for the recording ID on MP3, TXXX/Vorbis-comment/MP4-freeform-atom for the rest), and reading embedded tags back also returns them.
- `background-library-analysis`: a new automatic-identification-from-embedded-tags check runs on every analysis pass, trusting a file's own embedded MusicBrainz recording ID as authoritative over the tracking store.

## Impact

- Changed code: `internal/usecases/ports.go` (`TagInput`/`EmbeddedTags` gain four MBID fields), `internal/usecases/tag_file.go` (populate them when tagging), `internal/infrastructure/filestat/taglib_tagger.go` (write/read the four `MUSICBRAINZ_*` TagLib keys), `internal/usecases/analysis_manager.go` (new identify-from-embedded-tags step, ordered before the existing relocated-detection step so a file identified this way is eligible for relocation detection within the same pass).
- No API shape change beyond what's already exposed (recording/release/release-group/artist IDs are already in `GET /api/v1/library`'s response) — this only changes how that data can originate.
- No change to `file-tracking-store`'s specs: its existing "re-identification invalidates prior enrichment/tagged/relocated outcome" rules are implemented in the shared `RecordIdentification` store method, and already apply regardless of which caller triggers a new identification — this change doesn't need to touch that capability's wording.
- No dependency on any other in-progress change.
