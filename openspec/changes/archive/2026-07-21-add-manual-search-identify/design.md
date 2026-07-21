## Context

`MusicBrainzClient` (`internal/infrastructure/gateways/musicbrainz_client.go`) already calls `musicbrainz.org/ws/2` for two purposes: `Lookup` (recording ID → canonical metadata) and, since `disambiguate-tied-recordings`, `Releases` (release-group ID → sibling releases). MusicBrainz's web service also exposes a search endpoint (`GET /ws/2/recording?query=...&fmt=json`) accepting free-text/Lucene-style queries against fields like `artist`, `recording`, `release` — entirely independent of AcoustID or any audio fingerprint. Separately, `disambiguate-tied-recordings` already built a complete "here are several candidates, pick one" mechanism: `identification_candidates` table, `TrackingStore.RecordAmbiguous`/`GetCandidates`/`ResolveAmbiguous`, a `GET /api/v1/library/candidates` + `POST /api/v1/library/identify/resolve` API pair, and a candidate-picker UI in the details view — today only ever populated by `IdentifyFile.Identify` when AcoustID ties multiple recordings together.

## Goals / Non-Goals

**Goals:**
- Let a user identify (or re-identify) any tracked file by searching MusicBrainz with their own text, with no dependency on that file having a computable or matchable audio fingerprint at all.
- Reuse the existing candidate storage and picker UI wholesale — a manual search result and an AcoustID tied-recording are both just "a candidate metadata set to choose from," and the system shouldn't need two parallel UIs or storage shapes for that.
- Support "wide" search input — at minimum a free-text query, ideally separate artist/title/album fields for a more targeted MusicBrainz query when the user knows more than just a garbled filename.

**Non-Goals:**
- Changing anything about the automatic AcoustID-first identify flow — manual search is a parallel, user-initiated alternative, not a replacement or a fallback automatically triggered when AcoustID fails (that's a real possibility for later, deliberately deferred to keep this change's scope to "give the user a manual override," not "change what automatic identify does").
- Building a general external metadata browser (album art previews, artist bios, etc.) — this is strictly about finding a recording to identify a file with, same fields already shown in the existing candidate picker.
- Any change to how tagging/relocation/enrichment work — resolving a manual-search candidate ends at `identified`, exactly like resolving an ambiguous one; everything downstream is already agnostic to how a file became `identified`.

## Decisions

### New `MusicBrainzSearch` port, implemented by the existing `MusicBrainzClient`
```go
type MusicBrainzSearch interface {
    Search(ctx context.Context, query string, limit int) ([]RecordingMetadata, error)
}
```
Reuses `RecordingMetadata` — the exact shape already used for AcoustID-resolved candidates — so search results slot into the existing candidate-picker UI with zero rendering changes needed. `MusicBrainzClient.Search` calls `GET /ws/2/recording?query=<query>&fmt=json&limit=<limit>`, subject to the same centralized rate gate as `Lookup`/`Releases` (still the same MusicBrainz service, same 1 req/sec budget). Each search hit's `id` is looked up the same way the existing `selectRelease` release-selection heuristic already resolves a recording ID to full metadata — reusing that logic rather than parsing the search response's embedded release data separately, so search and fingerprint-based paths share one metadata-resolution code path. Alternative considered: parse full metadata directly out of the search response instead of doing a Lookup-equivalent per hit — rejected, since the search endpoint's embedded release/media data is a heavier query already available for free from `Lookup`'s own `inc=` parameters, and reusing `selectRelease` avoids maintaining two separate metadata-shaping code paths.

### Manual search results are stored via the existing `RecordAmbiguous`, for any file regardless of current status
A new `ManualSearch` usecase: `Search(ctx, path, query string) (candidates []RecordingMetadata, err error)` calls `MusicBrainzSearch.Search` and, on success, stores the results via the exact same `store.RecordAmbiguous(ctx, path, candidates)` `IdentifyFile` already calls for tied-recording disambiguation — no new store method needed. This means submitting a manual search always sets the file's status to `ambiguous` (even if it was `identified` or `not_found` before), discarding prior resolved metadata exactly like a fresh ambiguous result would. The existing `POST /api/v1/library/identify/resolve` endpoint and candidate-picker UI need no changes at all to handle manual-search-sourced candidates — they already only care that candidates exist for a path, not where they came from. Alternative considered: a separate storage/status path for "manually-searched, pending choice" distinct from `ambiguous` — rejected, since it would require duplicating every piece of the resolve/UI machinery `disambiguate-tied-recordings` already built for a state that is, functionally, identical: several candidates stored, waiting for one to be chosen.

### Search accepts a single free-text query, composed client-side from optional artist/title/album inputs
Rather than a new structured request shape, `ManualSearch.Search`'s `query` parameter is a single string — the caller (the new API handler) composes it from whatever fields the user filled in (e.g. `artist:"Daft Punk" AND recording:"Get Lucky"` when both are given, or a bare string when the user just pastes free text), using MusicBrainz's own Lucene-like query field syntax. This keeps the usecase and port simple (one string in, no query-DSL modeling needed in Go) while still giving the UI "wide" input — separate artist/title/album text boxes — the user asked for. Alternative considered: model structured `{Artist, Title, Album string}` fields through the port and build the Lucene query server-side — rejected as unnecessary complexity; composing the query string is trivial and keeps the port's contract (and any future non-MusicBrainz search backend) simpler.

### API and UI surface
- `POST /api/v1/library/identify/search` with body `{"path": "...", "query": "..."}` — triggers the search, stores results via `RecordAmbiguous`, and responds synchronously with the same shape `GET /api/v1/library/candidates` already returns (so the UI can render results immediately without a second round-trip), `200 OK` with an empty list if nothing matched (not an error — same "no results" convention as everywhere else in this API).
- The web UI's details view gains a "Search manually" control, available for any row (not gated to `ambiguous` like the existing candidate section) — a small form with free-text, or optionally artist/title/album fields, submitting to the new endpoint and rendering results through the exact same candidate-list/"Use this" component the ambiguous-resolution UI already has.

## Risks / Trade-offs

- **[Risk] A manual search on an already-`identified` file discards its resolved metadata immediately upon search (not upon choosing a candidate)**, even if the user backs out without picking anything → Accepted, but worth surfacing clearly in the UI (e.g. a confirmation before searching an already-identified file) — this mirrors `RecordAmbiguous`'s existing invalidation behavior exactly, just newly reachable from a state it wasn't reachable from before.
- **[Risk] MusicBrainz free-text search quality varies** (typos, unusual formatting, an obscure or extremely generic title) → Accepted: this is explicitly a manual, user-judged tool — the user sees real candidates and decides, unlike automatic identification's confidence-threshold gating.
- **[Trade-off] No fallback ranking/scoring shown to the user beyond MusicBrainz's own result order** → Acceptable for a first version; MusicBrainz search results are already relevance-ranked server-side. Revisit if result ordering proves confusing in practice.

## Migration Plan

- No schema changes — reuses `identification_candidates` and every existing candidate-related store method as-is.
- No rollback concern: reverting the code leaves no orphaned data shape, since nothing new is stored beyond what `disambiguate-tied-recordings` already introduced.
