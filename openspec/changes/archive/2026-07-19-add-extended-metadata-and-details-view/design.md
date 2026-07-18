## Context

`musicbrainz_client.go` (archived under `openspec/changes/archive/2026-07-18-add-acoustid-musicbrainz-identification/`) already fetches a recording's `releases+media+release-groups+artist-credits` and parses only the four fields the first identification change needed (artist, album, title, track number). The same JSON response already contains the release's own artist-credit (for Album Artist), release date (for Year), medium position/track-count (for disc/track counts), and the release/release-group/artist MBIDs — none of it currently captured. Separately, the web UI's table only shows a condensed one-line metadata summary per row, with no way to see a file's full resolved record without querying the database directly.

## Goals / Non-Goals

**Goals:**
- Capture Album Artist, Year, Disc Number, Total Discs, Total Tracks, Release MBID, Release-Group MBID, and Artist MBID — all derivable from the existing MusicBrainz response, no new API calls.
- Persist these alongside the existing resolved metadata, surviving restarts like everything else in the tracking store.
- Give the web UI a way to show a file's complete resolved record on demand, without adding a new backend endpoint.

**Non-Goals:**
- Language, ISRC, genre, composer — each needs either an additional `inc` parameter (extra API call) or a separate relation lookup; deferred to a future change if wanted.
- Any change to which release is selected (the existing "Album + Official, else first" heuristic is unchanged) — this change only extracts more fields from the same already-selected release/medium/track.
- No change to the identification flow itself (still on-demand, single/bulk, rate-limited) — purely additive data capture and a read-only UI addition.

## Decisions

- **Extend the existing MusicBrainz JSON structs rather than re-fetching or adding a second call.** `mbRelease` gains `ID`, `Date`, and `ArtistCredit` fields; `mbReleaseGroup` gains `ID`; `mbMedium` gains `Position` and `TrackCount`; `mbArtistCredit` gains a nested `Artist.ID`. All of these are already present in the response `selectRelease` already receives — this is purely "parse more of what we already have," not a new integration.
- **`selectRelease` returns the medium alongside the release and track.** Disc number and total-tracks-on-that-disc are medium-level fields (`position`, `track-count`), while total-discs is `len(release.Media)` — computed once at the release level, independent of which medium was selected. The function's signature changes from `(release, track, ok)` to `(release, medium, track, ok)` to carry this through without a second pass over the release list.
- **Year parsing takes the leading 4 digits of MusicBrainz's `date` field and tolerates any granularity.** MusicBrainz dates can be `"2018"`, `"2018-05"`, or `"2018-05-14"` — all three cases just need the year. If the first 4 characters aren't parseable as a number (empty date, unusual format), `Year` stays `0` rather than erroring; a missing year is common enough in MusicBrainz data that this must be a soft failure, not a fatal one.
- **Album Artist is sourced from the release's artist-credit, not the recording's.** These can legitimately differ (a variousartists compilation has one Artist per track but "Various Artists" as Album Artist) — this is the standard tagging convention and the reason the two fields exist separately at all. The existing `Artist` field's sourcing (recording-level) is unchanged.
- **No new API endpoint for the details view.** The web UI already receives every field on every tracked file via `GET /api/v1/library`. The details view is purely a client-side rendering concern: keep the last-fetched entries array in memory in `app.js`, and on a row click, look up that path's entry and render it into a modal/panel. This avoids a redundant per-file fetch and keeps the read path exactly as read-only and cheap as it already is.
- **Row click opens details; the row's checkbox stops propagation.** Clicking anywhere on a row except the checkbox opens its details panel; clicking the checkbox only toggles selection (`stopPropagation` prevents the click from also opening details). This keeps bulk-select and "view details" from fighting over the same click target.

## Risks / Trade-offs

- **`Year` can be `0` for a meaningful fraction of releases** with missing/unparseable dates → surfaced in the UI as blank rather than "0", so it doesn't read as a real value.
- **Album Artist can differ from Artist in ways that surprise a user unfamiliar with the convention** (e.g. seeing "Various Artists" as Album Artist on a track by a specific performer) → mitigated by labeling both fields clearly in the details view rather than only showing one ambiguous "artist" line.
- **Keeping the last-fetched entries array in client-side JS memory means the details view can show slightly stale data** if a background refresh/identify job updates that file between the last `GET /api/v1/library` poll and the user opening its details → acceptable; the existing polling loops already refresh this array every second while a job is running, and the modal can simply be closed and reopened after a refresh if needed. Not worth a live-updating modal for this v1.

## Open Questions

- Should the details view also expose a shortcut to trigger identification for that single file (equivalent to selecting just that row and clicking "Identify Selected")? Not required for this change — the existing row-selection flow already covers it — but worth considering as a small UX polish later if opening details becomes a common precursor to identifying a file.
