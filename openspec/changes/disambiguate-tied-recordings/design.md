## Context

`AcoustIDClient.Lookup` (`internal/infrastructure/gateways/acoustid_client.go`) calls AcoustID's `/v2/lookup` with `meta=recordings`. Its raw JSON response is a list of `results`, each with its own `score` and a `recordings` array — AcoustID's fingerprint index maps one acoustic fingerprint to a *result*, and a result can list several MusicBrainz recordings when the exact same audio is catalogued under more than one recording entry (a reissue, a compilation reusing the same master, a various-artists cross-listing, etc.). Today's client flattens every result's recordings into one flat `[]AcoustIDMatch{RecordingID, Score}` slice, so this grouping is lost by the time it reaches `IdentifyFile.Identify`, which — after the `improve-match-quality` change's confidence check — takes `matches[0].RecordingID` unconditionally. A real file (Daft Punk "Get Lucky (Radio Edit)") demonstrated the failure mode directly: AcoustID's top result scores **0.999** (a completely reliable acoustic match) but lists 5 recordings, and `matches[0]` happened to be an unrelated "Walt Ribeiro — Every Song!" compilation-series listing rather than the official single.

## Goals / Non-Goals

**Goals:**
- Preserve, per AcoustID result, which recordings are tied together, instead of losing that grouping during flattening.
- When AcoustID's top result ties two or more recordings that resolve to genuinely different metadata, stop auto-picking one — record the file as `ambiguous` with every candidate's resolved metadata stored, and let a user pick.
- When tied recordings resolve to the *same* artist/title (a harmless MusicBrainz cataloguing duplicate, not a real ambiguity), collapse them automatically and identify the file exactly as before — no new manual step for a case that isn't actually ambiguous.
- Leave the already-working single-recording-per-result path (the overwhelming majority of files) completely unchanged in behavior and performance.

**Non-Goals:**
- A general "search AcoustID/MusicBrainz manually and pick any candidate" feature — this only surfaces the candidates AcoustID's own tied result already returned, nothing beyond that.
- Bulk resolution of multiple ambiguous files at once — one file at a time via the details view, consistent with how the rest of the per-file review UI works today.
- Changing anything about the `improve-match-quality` confidence threshold — that check still runs first, against the top result's own score, before this change's tied-recording handling ever applies.

## Decisions

### `AcoustIDLookup.Lookup` returns grouped results, not a flat match list
`AcoustIDMatch{RecordingID, Score}` is replaced by `AcoustIDResult{Score float64, RecordingIDs []string}`, one entry per AcoustID result, ordered by descending score exactly as today — only the grouping of recording IDs within a result changes. `AcoustIDClient` is the only implementation; it stops flattening `result.Recordings` into individual matches and instead collects each result's recording IDs into one `RecordingIDs` slice. `IdentifyFile` is the only caller, updated in the same change. This is a mechanical, internal-only signature change: no new external dependency, no change to what AcoustID itself returns.

### Tied recordings are resolved and deduplicated by identity before deciding ambiguity
When the top (accepted, above-threshold) result has more than one recording ID, `IdentifyFile.Identify` resolves *each* one via `MusicBrainzLookup.Lookup` (respecting the existing 1 req/sec gate — this is the only case where identifying one file costs more than one MusicBrainz call, and it's rare) and deduplicates the results by `(Artist, Title)`. If they collapse to a single distinct identity, that's not real ambiguity — a MusicBrainz cataloguing quirk mapped the same song to multiple recording MBIDs — so the file is recorded `identified` with that one identity, same as if AcoustID had only returned one recording. Only when ≥2 distinct `(Artist, Title)` identities remain is the file recorded `ambiguous`, with every distinct candidate's full resolved metadata stored for the user to choose from. Alternative considered: treat any >1-recording result as ambiguous without resolving/deduping first — rejected, since the Daft Punk case's own AcoustID response is not the only shape tied-recordings takes, and skipping dedup would surface false "ambiguous" prompts for a large class of harmless MusicBrainz duplicates, defeating the point of a low-friction default path.

### New `ambiguous` status and candidate storage
A new `domain.StatusAmbiguous = "ambiguous"` joins `new`/`identified`/`not_found`/`missing`. An ambiguous file's row itself carries no resolved metadata (same as `not_found` today) — its candidates live in a new `identification_candidates` table (`path`, `recording_mbid`, plus the same resolved-metadata columns already used for `files`, primary keyed on `(path, recording_mbid)`). `TrackingStore` gains:
- `RecordAmbiguous(ctx, path, candidates []RecordingMetadata) error` — replaces any existing candidates for `path`, sets status to `ambiguous`, clears resolved metadata on the file row (mirrors `not_found`'s invalidation), in one transaction.
- `GetCandidates(ctx, path) ([]RecordingMetadata, error)` — reads back the stored candidate list for the details view / resolve picker.
- `ResolveAmbiguous(ctx, path, recordingMBID string) (found bool, err error)` — looks up the matching stored candidate, calls the existing `RecordIdentification` path with it (`Status: StatusIdentified`), and deletes that path's candidate rows in the same transaction. `found=false` (no error) when `recordingMBID` doesn't match any stored candidate for `path`, mirroring the rest of the store's not-found-but-not-an-error convention.

Alternative considered: a JSON blob column on `files` instead of a child table — rejected, since a relational child table keeps the existing per-column resolved-metadata shape consistent with how `files` already stores identified metadata, avoids hand-rolled JSON (de)serialization, and makes "does this file have candidates" a plain indexed lookup rather than a JSON-parse-and-check.

### Resolving a candidate reuses `RecordIdentification` — no new recording path
Once a user picks a candidate, it's written through the exact same `RecordIdentification(ctx, path, IdentificationResult{Status: StatusIdentified, Metadata: ...})` call an unambiguous success already uses. This means tagging, relocation, and enrichment — all of which already operate on any `identified` file — need zero changes; they can't tell an ambiguity-resolved identification apart from a direct one, which is exactly the intended behavior.

### Existing re-identification invalidation extends to candidates
`file-tracking-store`'s "Re-identification invalidates prior enrichment/tagged/relocated outcome" scenarios already fire whenever a file is identified again. The same triggers now also clear that path's stored candidates (via `RecordAmbiguous`/`RecordIdentification`/`ResolveAmbiguous` all touching the candidates table), so a stale candidate list from a previous ambiguous run never lingers once the file's identity changes.

### API and UI surface
- `POST /api/v1/library/identify/candidates` action is unnecessary — candidates are read via a new `GET /api/v1/library/candidates?path=...` (200 with `{"candidates": [...]}`, empty array if none stored, 404 for an untracked path — same convention as `/library/fingerprint`).
- A new `POST /api/v1/library/identify/resolve` with body `{"path": "...", "recording_mbid": "..."}` performs the resolve synchronously (it's a single store write against already-resolved metadata, no external calls, unlike the async identify/enrich/tag/relocate jobs) — `200 OK` on success, `404` if the path/candidate pair doesn't match anything stored.
- `GET /api/v1/library`'s `status` filter and the web UI's status filter `<select>` both gain `ambiguous`.
- The web UI shows a distinct label/color for `ambiguous` rows (parallel to how `not_found` already gets its own color) and, in the details view, a candidate picker listing each stored candidate's artist/album/title/track number with a "Use this" button per candidate, calling the resolve endpoint and refreshing the row on success.

## Risks / Trade-offs

- **[Risk] Resolving N tied recordings costs N MusicBrainz calls (rate-limited to 1/sec) for one file** → Accepted: this only happens when AcoustID's top result actually ties multiple recordings, which is rare; the vast majority of identify jobs are completely unaffected.
- **[Risk] The `(Artist, Title)` dedup key can under- or over-collapse** (e.g. two genuinely different recordings that happen to share artist/title, like a live version vs. studio version credited identically) → Accepted as a bounded, occasional cost: worst case, a real ambiguity gets silently auto-picked as if it were a duplicate. This mirrors the same kind of accepted imprecision as `improve-match-quality`'s closest-duration LRCLIB heuristic — a cheap, good-enough signal, not a perfect one.
- **[Trade-off] `AcoustIDLookup.Lookup`'s return shape changes** (breaking, internal only) → `AcoustIDClient` is the only implementation and `IdentifyFile` the only caller; both updated together in this change, so nothing external observes the old shape.

## Migration Plan

- Schema change: add the `identification_candidates` table and the `ambiguous` status value (no change needed to the `status` column's type — it's already a free-text `TEXT` column, not a constrained enum). No migration needed for existing rows: no existing file is `ambiguous` today, so the new table simply starts empty.
- Rollback: reverting the code leaves the (now-unused) `identification_candidates` table in place, harmlessly empty or populated — no destructive rollback step needed, since the `files` table itself is untouched by this change beyond the new status value already being a valid free-text string.
