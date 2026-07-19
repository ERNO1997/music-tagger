package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"music-tagger/internal/usecases"
)

const lrclibBaseURL = "https://lrclib.net/api/get"

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

type lrclibResponse struct {
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
	Instrumental bool   `json:"instrumental"`
}

// Lookup queries LRCLIB's /api/get endpoint, which returns a single best
// match (or 404) given precise artist/title/album/duration — we already
// have all four fields stored on every identified file, so this is a
// precise lookup, not a fuzzy one. Duration is passed even though optional:
// it measurably improves match precision at no extra cost. A 404 or an
// instrumental match are both treated as found=false, err=nil.
func (c *LRCLIBClient) Lookup(ctx context.Context, artist, title, album string, durationSeconds int) (string, string, bool, error) {
	query := url.Values{}
	query.Set("artist_name", artist)
	query.Set("track_name", title)
	query.Set("album_name", album)
	if durationSeconds > 0 {
		query.Set("duration", strconv.Itoa(durationSeconds))
	}

	reqURL := lrclibBaseURL + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", "", false, fmt.Errorf("building LRCLIB request: %w", err)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", false, fmt.Errorf("LRCLIB request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("LRCLIB error: HTTP %d", resp.StatusCode)
	}

	var body lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", false, fmt.Errorf("decoding LRCLIB response: %w", err)
	}

	if body.Instrumental {
		return "", "", false, nil
	}

	return body.PlainLyrics, body.SyncedLyrics, true, nil
}

var _ usecases.LyricsLookup = (*LRCLIBClient)(nil)
