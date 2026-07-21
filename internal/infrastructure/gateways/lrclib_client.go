package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"music-tagger/internal/usecases"
)

const (
	lrclibGetURL    = "https://lrclib.net/api/get"
	lrclibSearchURL = "https://lrclib.net/api/search"
)

// LRCLIBClient resolves an already-known artist/title/album/duration to
// plain and synced lyrics via LRCLIB — a fully public, unauthenticated API
// with no documented rate limit, so no rate gate here.
type LRCLIBClient struct {
	UserAgent  string
	HTTPClient *http.Client
}

func NewLRCLIBClient(userAgent string) *LRCLIBClient {
	return &LRCLIBClient{UserAgent: userAgent, HTTPClient: http.DefaultClient}
}

// lrclibTrack is the shape of one track as returned by both /api/get (a
// single object) and /api/search (an array of these).
type lrclibTrack struct {
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// Lookup queries LRCLIB's /api/get endpoint, which returns a single best
// match (or 404) given precise artist/title/album/duration — we already
// have all four fields stored on every identified file, so this is a
// precise lookup, not a fuzzy one. Duration is passed even though optional:
// it measurably improves match precision at no extra cost. If the exact
// lookup 404s, Lookup falls back to /api/search (fuzzy, by artist/title
// only) and picks the closest-duration candidate. A fuzzy miss or an
// instrumental match (from either endpoint) are both treated as
// found=false, err=nil.
func (c *LRCLIBClient) Lookup(ctx context.Context, artist, title, album string, durationSeconds int) (string, string, bool, error) {
	query := url.Values{}
	query.Set("artist_name", artist)
	query.Set("track_name", title)
	query.Set("album_name", album)
	if durationSeconds > 0 {
		query.Set("duration", strconv.Itoa(durationSeconds))
	}

	var track lrclibTrack
	found, err := c.get(ctx, lrclibGetURL+"?"+query.Encode(), &track)
	if err != nil {
		return "", "", false, err
	}

	if !found {
		track, found, err = c.searchByClosestDuration(ctx, artist, title, durationSeconds)
		if err != nil {
			return "", "", false, err
		}
	}

	if !found || track.Instrumental {
		return "", "", false, nil
	}

	return track.PlainLyrics, track.SyncedLyrics, true, nil
}

// get performs a GET against reqURL, decoding a single JSON object into out.
// found is false (with a nil error) for a 404; other non-2xx responses or
// decode failures return a distinguishable error.
func (c *LRCLIBClient) get(ctx context.Context, reqURL string, out any) (found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("building LRCLIB request: %w", err)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("LRCLIB request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("LRCLIB error: HTTP %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, fmt.Errorf("decoding LRCLIB response: %w", err)
	}
	return true, nil
}

// searchByClosestDuration queries /api/search by artist and title only (no
// album — LRCLIB's cross-album duplicates all carry correct lyrics
// regardless of album match) and, among the returned candidates, picks the
// one whose duration is closest to durationSeconds. It falls back to the
// first result when durationSeconds is 0 (unknown) or no candidate has a
// usable duration. found is false when the search returns zero results.
func (c *LRCLIBClient) searchByClosestDuration(ctx context.Context, artist, title string, durationSeconds int) (lrclibTrack, bool, error) {
	query := url.Values{}
	query.Set("artist_name", artist)
	query.Set("track_name", title)

	var results []lrclibTrack
	found, err := c.get(ctx, lrclibSearchURL+"?"+query.Encode(), &results)
	if err != nil {
		return lrclibTrack{}, false, err
	}
	if !found || len(results) == 0 {
		return lrclibTrack{}, false, nil
	}

	if durationSeconds <= 0 {
		return results[0], true, nil
	}

	best := results[0]
	bestDiff := math.Abs(best.Duration - float64(durationSeconds))
	for _, r := range results[1:] {
		diff := math.Abs(r.Duration - float64(durationSeconds))
		if diff < bestDiff {
			best, bestDiff = r, diff
		}
	}
	return best, true, nil
}

var _ usecases.LyricsLookup = (*LRCLIBClient)(nil)
