package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"music-tagger/internal/usecases"
)

const coverArtArchiveBaseURL = "https://coverartarchive.org"

// CoverArtClient resolves a MusicBrainz Release ID to front-cover image
// bytes via Cover Art Archive — a fully public, unauthenticated API with no
// documented rate limit (unlike MusicBrainz itself), so no rate gate here.
type CoverArtClient struct {
	UserAgent  string
	HTTPClient *http.Client
}

func NewCoverArtClient(userAgent string) *CoverArtClient {
	return &CoverArtClient{UserAgent: userAgent, HTTPClient: http.DefaultClient}
}

type coverArtResponse struct {
	Images []coverArtImage `json:"images"`
}

type coverArtImage struct {
	Front      bool               `json:"front"`
	Image      string             `json:"image"`
	Thumbnails coverArtThumbnails `json:"thumbnails"`
}

type coverArtThumbnails struct {
	Large string `json:"large"`
}

// Lookup returns the front-cover image bytes for a release, falling back
// to the release-group's representative cover if the specific release has
// none (a release-group can have many sibling editions, and not all of
// them have art uploaded). Returns (nil, nil) if no cover art is available
// anywhere in the release-group (both lookups 404) — distinct from a
// non-nil error, which means a lookup itself failed.
func (c *CoverArtClient) Lookup(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]byte, error) {
	images, err := c.fetchImages(ctx, "release", releaseMBID)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 && releaseGroupMBID != "" {
		images, err = c.fetchImages(ctx, "release-group", releaseGroupMBID)
		if err != nil {
			return nil, err
		}
	}

	if len(images) == 0 {
		return nil, nil
	}

	imageURL := selectFrontImageURL(images)
	if imageURL == "" {
		return nil, nil
	}

	return c.download(ctx, imageURL)
}

// fetchImages queries Cover Art Archive for a "release" or "release-group"
// entity and returns its images list, or (nil, nil) on a 404 (no cover art
// available for that entity) — distinct from a non-nil error.
func (c *CoverArtClient) fetchImages(ctx context.Context, entity, mbid string) ([]coverArtImage, error) {
	metaURL := fmt.Sprintf("%s/%s/%s", coverArtArchiveBaseURL, entity, mbid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building Cover Art Archive request: %w", err)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cover Art Archive request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cover Art Archive error: HTTP %d", resp.StatusCode)
	}

	var body coverArtResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding Cover Art Archive response: %w", err)
	}
	return body.Images, nil
}

// selectFrontImageURL prefers the "large" thumbnail of the image marked
// front, falling back to the first image if none is marked front.
func selectFrontImageURL(images []coverArtImage) string {
	if len(images) == 0 {
		return ""
	}
	for _, img := range images {
		if img.Front {
			return img.Thumbnails.Large
		}
	}
	return images[0].Thumbnails.Large
}

func (c *CoverArtClient) download(ctx context.Context, imageURL string) ([]byte, error) {
	// Cover Art Archive's JSON returns http:// URLs; always upgrade to
	// https:// before requesting, since the encrypted path is confirmed to
	// work identically (it redirects through to archive.org either way).
	imageURL = strings.Replace(imageURL, "http://", "https://", 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building cover art download request: %w", err)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cover art download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art download error: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading cover art bytes: %w", err)
	}
	return data, nil
}

var _ usecases.CoverArtLookup = (*CoverArtClient)(nil)
