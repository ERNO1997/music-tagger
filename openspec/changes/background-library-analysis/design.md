## Context

Three existing capabilities already do the underlying work this change needs, just only on explicit user request:
- `audio-fingerprinting`: computes a Chromaprint fingerprint for a local file via `fpcalc`. Currently invoked lazily, once, the first time a file is submitted for identification (see `file-tracking-store`'s "Fingerprint computed lazily during identification").
- The details view's embedded-tag read path (`TagFile.GetEmbeddedTags`, served via `GET /api/v1/library/tags`) already reads a file's *own* tags directly from disk via TagLib, independent of the tracking store — but today it only returns two booleans (`HasLyrics`, `HasCoverArt`) for display, not the actual lyrics text or cover image bytes.
- `file-relocation`'s `PathRelocator.Relocate` computes a file's canonical destination (`{musicRoot}/{sanitized artist}/{year - }{sanitized album}/{track} - {sanitized title}{ext}`) and, as part of the same function, performs the move — including an existing check that treats "already at that path" as a successful no-op rather than a collision. That destination computation is not currently reusable independently of performing a move.

`RefreshManager` already runs one background job automatically at server startup and chains to a relocate-status check (`SetRelocateStatus`) — this change adds a fourth automatic job to that same family, chained to run after every refresh (startup or manually triggered) rather than only at startup.

## Goals / Non-Goals

**Goals:**
- No user action required: fingerprinting, embedded-content detection, and passive relocation detection all happen automatically after a refresh.
- Never overwrite something a user (or a prior automatic pass) already established: embedded-content detection only fills in a currently-empty cover art path or lyrics field; passive relocation detection only marks a file relocated when it's genuinely already at its canonical path.
- Reuse existing mechanisms (fingerprinting, destination computation, tag reading) rather than re-implementing them.
- Observable progress, consistent with how scan/identify/enrich/tag/relocate already report progress.

**Non-Goals:**
- Automatically running identify or enrich themselves (the AcoustID/MusicBrainz/Cover-Art-Archive/LRCLIB lookups) — those remain on-demand, user-triggered actions; this change only fingerprints (a prerequisite identify already needs) and looks at what's already embedded in the file itself, not external services.
- Automatically tagging or relocating a file — this change only *marks* an already-relocated file as such; it never moves a file itself. Tagging remains on-demand.
- Continuous/real-time analysis (e.g. a filesystem watcher) — this is triggered once per refresh, same as the rest of the existing pipeline.

## Decisions

### Fingerprinting: proactive, but the lazy-during-identify path is unchanged
The background pass computes a fingerprint for any tracked file that doesn't have one, using the exact same `audio-fingerprinting` mechanism identify already calls. This doesn't change identify's own behavior or its spec requirement ("computed lazily... if no fingerprint already stored") — that remains true and becomes the fallback path for a file added between one background pass and the next (e.g. a scan just found it, background analysis for this refresh hasn't reached it yet, and a user selects it and hits Identify right away). Whichever process gets there first wins; the other reuses the stored value. No new locking is introduced for this — recomputing the same fingerprint twice in a rare race is a harmless, idempotent overwrite with the same value (see Risks).

### Embedded cover art/lyrics extraction reuses and extends the existing tag-reading path
`TagFile.GetEmbeddedTags`'s underlying TagLib read is extended to also return the actual embedded picture bytes and lyrics text when present (today it only returns booleans, since that's all the details view needed). The background pass calls this for every tracked file whose `CoverArtPath`/`Lyrics` are currently empty, and stores whatever it finds using the exact same tracking-store write path enrichment already uses (so cover art goes through the same `covers.Store` dedup-by-release mechanism enrichment relies on — though here keyed by the file's own embedded image rather than a MusicBrainz release, since an unidentified file has no release to key by). If a file is already identified and multiple tracks share a cover, no cross-track dedup is attempted for embedded extraction (unlike enrichment's "shared cover art across tracks on the same release" behavior) — each file's own embedded image is used independently, since this path doesn't have a release ID to dedup by. Alternative considered: skip storage entirely and just set a boolean "has embedded cover/lyrics" flag distinct from enrichment's fields — rejected, since that would mean the existing `has_cover_art`/`has_lyrics` filters and the details view's cover/lyrics display would need to check two different sources everywhere they're used today; reusing the same fields keeps every existing consumer working unchanged.

### Passive relocation detection reuses an extracted, filesystem-free destination computation
`PathRelocator.Relocate`'s destination-path computation (currently inline: sanitize segments, build `dir`/`filename`/`dest`) is extracted into its own function taking the same `RelocateInput` and returning just the computed path, with no filesystem access. `Relocate` itself calls this function and then proceeds exactly as it does today (existing behavior unchanged). The background pass calls the same function for every `identified`+`tagged`, not-yet-`relocated` file, compares it against the file's current tracked path, and marks the record `relocated` (clearing any stored relocation error) when they match — reusing the exact same "already at destination" semantics `file-relocation`'s spec already documents for the on-demand action, just triggered passively instead of requiring the user to click Relocate.

### A new status endpoint, following the existing convention
`GET /api/v1/library/analyze/status` returns `{running, processed, total}`, identical in shape to the existing scan/identify/enrich/tag/relocate status endpoints. The web UI's existing status line gains a fifth "Analyzing… X/Y" state, polled the same way the other four already are. No trigger endpoint is needed (unlike identify/enrich/tag/relocate) since this job is never user-initiated — it starts automatically when a refresh completes.

### Serialized with refresh and relocate, like the existing scan/relocate mutual exclusion
The background pass starts only after a refresh fully completes (chained the same way `RefreshManager` already chains to `RelocateManager.SetRelocateStatus`), and is treated as mutually exclusive with an in-progress relocate job for the same reason scan and relocate already exclude each other: relocation moves files and updates their tracked path, and a concurrent analysis pass reading paths could race with that. The exact primitive is left to implementation — likely extending the same mutual-exclusion mechanism `RefreshManager`/`RelocateManager` already share, rather than inventing a new one.

## Risks / Trade-offs

- **[Risk] Fingerprinting every file automatically, for a very large library, could take a long time in the background** → Accepted: fingerprinting is already required before identification works at all; this just moves the same unavoidable cost earlier and off the critical path of a user explicitly waiting on an identify job. Progress is observable via the new status endpoint, same as any other background job here.
- **[Risk] A race between this pass and a user-triggered identify computing the same file's fingerprint concurrently** → Accepted: both paths write the same deterministic value for the same file content; a duplicate computation is wasted work, not a correctness issue.
- **[Trade-off] Embedded cover art extracted this way isn't deduped across tracks of the same release the way enrichment's Cover Art Archive lookups are** → Accepted: without a resolved release ID (many files processed by this pass aren't identified yet), there's nothing to dedup by; storing each file's own embedded image independently is simple and correct, just not storage-optimal for a large multi-track embedded-cover album. Optimizing that is a separate concern from *whether* embedded content is detected at all.
- **[Risk] Extending the TagLib wrapper to return actual image bytes/lyrics text (not just booleans) touches code shared with tagging/embedded-tags-display** → Mitigated by keeping the boolean-only `GetEmbeddedTags`/`GET /api/v1/library/tags` response contract unchanged (additive internal capability only, not a response-shape change) — the existing details-view consumer is unaffected.

## Migration Plan

- No schema migration for existing fields (`cover_art_path`, lyrics, `relocated`) — this pass writes into them using the same shape enrichment/relocation already use.
- Rollout: on first startup after this ships, the very next automatic refresh (already scheduled to run at startup) will trigger a first analysis pass over the entire existing library, which may take a while for a large, previously-unanalyzed library. This is expected, one-time (for already-processed files, subsequent passes are no-ops), and observable via the new status endpoint.
- Rollback: reverting this change stops the automatic pass; nothing it wrote (fingerprints, extracted cover art/lyrics, relocated flags) needs to be undone — it's all data the corresponding on-demand actions would have produced anyway.

## Open Questions

- Exact mutual-exclusion primitive between this pass and an in-progress relocate job — left to implementation, likely reusing whatever `RefreshManager`/`RelocateManager` already share for scan-vs-relocate exclusion.
- Whether TagLib's existing Go wrapper in this codebase already exposes raw embedded picture bytes and lyrics text, or only presence booleans — needs a quick implementation-time check before scoping the "extend the embedded-tag reader" task precisely.
