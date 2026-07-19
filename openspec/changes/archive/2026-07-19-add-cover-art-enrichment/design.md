## Context

Files now carry a Release MBID once identified (from `add-extended-metadata-and-details-view`, archived). Cover Art Archive resolves that exact ID to front-cover artwork via a fully public, unauthenticated API — no key, no documented rate limit (unlike MusicBrainz). This change adds that lookup, storage for the downloaded image, and an on-demand "Enrich Selected" action mirroring the existing Identify pattern (its own background job, its own concurrency guard, same progress-polling UX already proven in the UI).

## Goals / Non-Goals

**Goals:**
- Resolve a Release MBID to its front-cover image via Cover Art Archive and download the actual bytes.
- Persist the image as a file under `/data/covers/`, with just a path reference in the tracking store — keep the SQLite database itself small.
- Add enrichment as its own on-demand, background, rate-limit-free job — independent of both the scan refresh and the identify job.
- Serve the stored image back to the browser so the UI can show a thumbnail.

**Non-Goals:**
- Lyrics/Genius — explicitly deferred to a separate future decision (ToS/scraping concerns are a different risk profile than Cover Art Archive's sanctioned API).
- Embedding the downloaded art into the actual audio file's tags — that's the future tagging capability, not this one.
- Any artificial rate limiting for Cover Art Archive calls — `project.md` §2.3 documents no rate constraint for this API, unlike MusicBrainz's explicit 1 req/sec.
- Choosing among sibling releases ourselves (iterating MusicBrainz's list of a release-group's editions) — Cover Art Archive's own release-group endpoint already picks a representative release for us (see Decisions below), so there's no need to reimplement that selection.

## Decisions

- **Fetch the `large` thumbnail (500px), not the full-resolution original.** Verified directly against the real API: a release's `images[].thumbnails.large` key points to the same 500px JPEG as the numbered `"500"` key (Cover Art Archive treats them as aliases). This resolution is large enough for future tag-embedding and comfortably large enough for a UI thumbnail — fetching the (often much larger) full `image` URL would just be wasted bandwidth/disk for both current uses.
- **Always request over HTTPS, even though the API's JSON returns `http://` URLs.** Confirmed live that swapping the scheme to `https://` on the same path works identically (Cover Art Archive redirects through to `archive.org`'s HTTPS-hosted file either way) — there's no reason to make an unencrypted request when the encrypted path is already proven to work.
- **Front-cover selection: prefer `images[].front == true`; fall back to the first image if none is explicitly marked front.** Matches the same "prefer the clearly-correct signal, fall back to first available" shape already used for release selection in `musicbrainz-metadata`.
- **A 404 from Cover Art Archive means "no cover art available," not an error.** Confirmed directly against real releases (some have art, some don't — 404 is a normal, expected response for a real release with no uploaded art). The `CoverArtPath` field stays empty in that case; this is not treated as a fingerprint/gateway failure requiring the file's status to change.
- **Store images as files under `/data/covers/<release-mbid>.jpg`, not as SQLite BLOBs.** Consistent with the project's existing preference for keeping the SQLite file itself small and letting the filesystem hold larger artifacts (mirrors why the tracking database itself is a plain file rather than something heavier). Keying by Release MBID means multiple tracks on the same release naturally share one downloaded image file rather than duplicating it per-track.
- **`EnrichManager` is a third independent `JobManager` instance, alongside `RefreshManager` and `IdentifyManager`.** Same rationale as when `IdentifyManager` was introduced: enrichment touches a different resource (Cover Art Archive) than either scanning (local `fpcalc`) or identification (AcoustID/MusicBrainz), so there's no correctness reason to serialize it against the other two — only against itself (no two enrich jobs at once).
- **Enrichment requires a file to already be `identified`.** Cover art lookup needs a Release MBID, which only exists post-identification. A path submitted to `POST /api/v1/library/enrich` that isn't yet identified is skipped (logged, not fatal to the rest of the batch) — same per-item-failure-doesn't-abort-the-job pattern already used for identify.
- **New `GET /api/v1/library/cover?path=...` endpoint serves the stored image file directly** (reads the path recorded for that file's tracking record, serves the bytes with the correct `Content-Type: image/jpeg`), rather than exposing `/data/covers/` as a static directory — keeps the tracking-store-to-file mapping as the single source of truth and avoids exposing internal storage layout as a public path structure.
- **Fall back to Cover Art Archive's release-group endpoint when the specific release 404s.** Found during real-world verification: a release-group can have dozens of "Official" sibling releases (regional/format editions of the same album, all released the same day) — our release-selection heuristic (first Album+Official match) has no way to know in advance which sibling has art uploaded, and in a confirmed real case, the specific release we'd picked had none while most siblings did. Rather than fetching and iterating the release-group's full release list ourselves (an extra MusicBrainz-side call plus our own selection logic duplicated), `GET https://coverartarchive.org/release-group/{release-group-mbid}` already does exactly this — it returns the same `images[]` shape and automatically resolves to whichever sibling actually has art. So `CoverArtLookup.Lookup` now takes both the Release MBID and Release-Group MBID, tries the release-level endpoint first (exact match), and falls back to the release-group endpoint only if that 404s.

## Risks / Trade-offs

- **A release-group with no cover art on any of its releases will still permanently show no thumbnail** — acceptable; Cover Art Archive coverage isn't universal even across an entire release-group. The release-group fallback substantially narrows this gap (confirmed live: most siblings of a real 25-edition release-group had art even though our selected release didn't) but doesn't eliminate it entirely.
- **Storing images under `/data/covers/` grows the `/data` volume independent of the SQLite file** → acceptable; images are the expected bulk of storage for a "fully managed library" feature, and this keeps the database itself fast and small. No cleanup/pruning story yet (e.g. an orphaned image if a release is somehow untracked) — acceptable for v1, revisit if it becomes an actual problem.
- **Keying cover art files by Release MBID (not by individual track path) means re-enriching a second track on the same already-covered release is a wasted, redundant API call and download** unless the enrich job checks for an existing file first → mitigated by checking whether `/data/covers/<release-mbid>.jpg` already exists before calling Cover Art Archive at all, skipping straight to recording the existing path.

## Open Questions

- Should the UI thumbnail be shown at table-row scale (small) and full-size in the details view, using the same single stored 500px file scaled by CSS, or would two stored sizes ever be worth it? Defaulting to one stored size + CSS scaling for simplicity; revisit only if 500px proves visibly blurry when scaled up in the details view.
