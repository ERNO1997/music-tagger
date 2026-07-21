## Why

Every identification path today requires an audio fingerprint: `fpcalc`/Chromaprint → AcoustID → MusicBrainz. A file AcoustID can't confidently match (below the confidence threshold), a corrupt/unusual file `fpcalc` can't fingerprint at all, or a file whose fingerprint ties to the wrong recording (see `disambiguate-tied-recordings`) currently has no other path to correct identification — the user's only recourse is accepting `not_found`/`ambiguous` or leaving it unidentified. MusicBrainz itself supports free-text search (by artist/title/album, independent of any audio fingerprint), so a user who knows what a file actually is should be able to search for it directly and pick the right result, without AcoustID ever being consulted.

## What Changes

- A new MusicBrainz free-text recording search is added, alongside the existing fingerprint-based lookup, resolving a text query directly to candidate recordings (with the same resolved metadata shape already used everywhere else).
- A new manual search action lets a user submit a free-text query (or separate artist/title/album fields) for any tracked file — regardless of its current status (`new`, `not_found`, `ambiguous`, or even already `identified`, to let a user override a wrong result) — and see the matching candidates.
- Search results are presented and chosen through the exact same candidate-picker UI and resolve action already built for AcoustID tied-recording disambiguation — picking a manual search result records it exactly like resolving an ambiguous candidate (same fields, same downstream tagging/relocation eligibility).
- The web UI gains a manual search control (accessible from any row's details view, not just `ambiguous` ones) with fields for artist/title/album, rendering results in the same candidate list/"Use this" pattern.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `musicbrainz-metadata`: gains free-text recording search, resolving a query directly to candidate recordings without requiring an AcoustID fingerprint match first.
- `file-tracking-store`: candidates may now originate from a manual search in addition to AcoustID tied-recordings; a manual search's candidates are storable and choosable via the same existing candidate/resolve mechanism, for any tracked file regardless of current status.
- `music-library-scan`: new endpoints to submit a manual search for a tracked file and to reuse the existing candidate-resolve endpoint against manual-search results; the web UI gains a manual search control available from any row.

## Impact

- Changed code: `internal/infrastructure/gateways/musicbrainz_client.go` (new free-text recording search method), `internal/usecases/ports.go` (new `MusicBrainzSearch` port), `internal/usecases/manual_search.go` (new usecase: search, store results as candidates via the existing `RecordAmbiguous` mechanism), `internal/infrastructure/persistence/sqlite_store.go` (no schema change — reuses the existing `identification_candidates` table as-is), `internal/infrastructure/web/v1/` (new handler for triggering a manual search), `ui/index.html`/`ui/js/app.js` (manual search control, reusing the existing candidate-picker rendering).
- No schema change: manual search results are stored via the same `identification_candidates` table and `RecordAmbiguous`/`ResolveAmbiguous` methods `disambiguate-tied-recordings` already built — this change adds a second *source* of candidates, not a new storage shape.
- A file's status becomes `ambiguous` once manual search results are stored for it (even if it was previously `identified` or `not_found`), since "here are several candidates, pick one" is the same state regardless of how the candidates arose; resolving one via the existing endpoint returns it to `identified` exactly as today.
- Independent of `improve-library-visibility` and `refactor-frontend-modules`, though it benefits from the latter's module split for where the new search UI control lives; can be built and archived in any order relative to those two.
