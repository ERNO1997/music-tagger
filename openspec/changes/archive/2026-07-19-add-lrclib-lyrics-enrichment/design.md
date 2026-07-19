## Context

`EnrichFile`/`EnrichManager` (from `add-cover-art-enrichment`, archived) already provide the "given an identified file, fetch and store extra data" pipeline for cover art. `project.md`'s original Workflow 1 always described enrichment as one step covering both cover art *and* lyrics — this change fills in the second half using LRCLIB, verified live against the real API during proposal drafting (real request/response shapes, 404 behavior, and duration-matching tolerance all confirmed against actual tracks already in the user's library).

## Goals / Non-Goals

**Goals:**
- Resolve plain and synced (LRC-timed) lyrics via LRCLIB, given a file's already-resolved artist/title/album/duration.
- Fold this into the existing "Enrich Selected" action rather than adding a new job/button — matches `project.md`'s original single-step enrichment framing.
- Keep `GET /api/v1/library` lightweight (an indicator only); serve full lyrics text on demand via a dedicated endpoint, same reasoning as cover art's separate image endpoint.
- Treat "not found" and `instrumental: true` identically as "no lyrics available" — not an error.

**Non-Goals:**
- No fallback to LRCLIB's `/api/search` (fuzzy, multi-candidate) endpoint. Verified live: `/api/search` returns several near-duplicate candidates (same lyrics, different album groupings) for a query that `/api/get` already resolves cleanly using our own precise, already-known artist/title/album/duration. Adding our own candidate-ranking logic on top of `/search` would duplicate complexity for likely-marginal recall gain — a real gap here would look like the release-group-fallback case found for cover art (a specific, reproducible miss), and can be added the same way if/when that actually happens.
- No lyrics deduplication/sharing across files. Cover art dedups by Release MBID because it's a binary file with real download/storage cost; lyrics are plain text stored inline in the row, keyed by recording rather than release — the storage and dedup-complexity trade-off that justified cover art's file-based sharing doesn't apply here.
- No embedding of lyrics into actual audio file tags — future tagging capability.
- No UI for editing lyrics or submitting corrections back to LRCLIB.

## Decisions

- **Use `/api/get`, not `/api/search`.** `/api/get` takes `artist_name`, `track_name`, `album_name`, `duration` and returns a single best match (or 404). We already have all four fields stored on every identified file, so this is a precise lookup, not a fuzzy one — consistent with how `musicbrainz-metadata` and `cover-art-lookup` both prefer precise, already-known identifiers over building our own candidate-selection logic.
- **Pass `duration` even though it's optional.** Verified live: omitting `duration` can return a different (differently-capitalized, in one observed case) database entry than passing it. Duration has some tolerance (a 5-second mismatch still matched in testing; a 30-second mismatch did not), so our stored `duration_seconds` — accurate to the actual file — meaningfully improves match confidence at no extra cost.
- **`instrumental: true` and a 404 are both "no lyrics available."** Both leave `Lyrics`/`SyncedLyrics` empty without being treated as an error, mirroring the cover art capability's 404 handling exactly.
- **`EnrichFile.Enrich`'s growing parameter list becomes a struct.** It already took `(ctx, path, releaseMBID, releaseGroupMBID)` for cover art; adding artist/title/album/duration for lyrics would make an 7-argument positional call error-prone. Introducing an `EnrichmentInput` struct (`Path`, `ReleaseMBID`, `ReleaseGroupMBID`, `Artist`, `Title`, `Album`, `DurationSeconds`) keeps the call site readable as more enrichment sources are added later (this shape already anticipates a lyrics-source count of one now, more later).
- **Cover art and lyrics are attempted independently within one `Enrich` call, not short-circuited by each other.** Matches the existing per-concern-isolation pattern (a fingerprint failure doesn't block identification of other files; one file's identify failure doesn't abort the batch). If cover art lookup hard-errors, lyrics lookup is still attempted for that same file, and vice versa; only "not found" results in each independently leaving that field empty. Errors from both are combined via `errors.Join` so `EnrichManager`'s existing per-path logging still sees an error, without losing information about a possible partial success.
- **Re-identification invalidates lyrics, same as it now invalidates cover art.** If a file resolves to a different recording on re-identify, its previously-fetched lyrics belong to the wrong song. `RecordIdentification` already resets `cover_art_path` on both `identified` and `not_found` outcomes (a fix made during the cover-art change) — extending this to also reset `Lyrics`/`SyncedLyrics` closes the same gap for the new fields before it can occur, rather than after a bug report.
- **New `RecordLyrics` store method and a dedicated `GetLyrics` single-row lookup** — same shape as `RecordCoverArt`/`GetCoverArtPath`, for the same reason: serving lyrics on demand (once per details-view open) shouldn't cost a full-table `LoadAll`.
- **No `has_synced_lyrics` field, just `has_lyrics`.** A track either has usable lyrics data or it doesn't; whether the timing information is present is a detail the lyrics endpoint's response already conveys once fetched (an empty `synced_lyrics` string alongside populated `plain_lyrics` is common and expected — many LRCLIB entries have only one or the other).

## Risks / Trade-offs

- **LRCLIB's crowd-sourced database can have near-duplicate entries with minor inconsistencies** (verified live: capitalization differed between an exact `/api/get` match and a no-duration fallback match) → mitigated by always passing duration for the most precise available match; residual inconsistency is a data-quality property of the source, not something we can control further without much more complexity.
- **A track with no LRCLIB entry permanently shows no lyrics** — acceptable; coverage is smaller than Genius's, which was the whole trade-off accepted when choosing LRCLIB over Genius/Musixmatch (no scraping, no paid tier, at the cost of some coverage).
- **Combining two independent gateway calls into one `Enrich` invocation makes per-file enrichment take roughly twice as long** (two sequential HTTP round-trips instead of one) → acceptable; neither Cover Art Archive nor LRCLIB has a documented rate limit, so this is a latency cost only, not a correctness or rate-limit-compliance concern, and the existing progress-polling UI already communicates an in-flight job clearly regardless of how long each file takes.

## Open Questions

- Should the details view render `synced_lyrics` specially (e.g. as a scrolling, timestamp-highlighted view) versus just showing `plain_lyrics` as a simple text block? Defaulting to a plain scrollable text block of `plain_lyrics` for v1 (falling back to `synced_lyrics` with its timestamps visible as plain text if `plain_lyrics` is empty but `synced_lyrics` isn't) — a proper synced-lyrics player UI is a nice-to-have, not required to satisfy this change's goal of simply showing that lyrics exist and what they say.
