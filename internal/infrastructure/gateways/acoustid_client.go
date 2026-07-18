package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

const acoustIDLookupURL = "https://api.acoustid.org/v2/lookup"

// AcoustIDClient resolves a fingerprint + duration to candidate MusicBrainz
// Recording IDs via the AcoustID API.
type AcoustIDClient struct {
	APIKey     string
	HTTPClient *http.Client
}

func NewAcoustIDClient(apiKey string) *AcoustIDClient {
	return &AcoustIDClient{APIKey: apiKey, HTTPClient: http.DefaultClient}
}

type acoustIDResponse struct {
	Status  string             `json:"status"`
	Results []acoustIDResult   `json:"results"`
	Error   *acoustIDErrorBody `json:"error,omitempty"`
}

type acoustIDErrorBody struct {
	Message string `json:"message"`
}

type acoustIDResult struct {
	ID         string              `json:"id"`
	Score      float64             `json:"score"`
	Recordings []acoustIDRecording `json:"recordings"`
}

type acoustIDRecording struct {
	ID string `json:"id"`
}

func (c *AcoustIDClient) Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]usecases.AcoustIDMatch, error) {
	if c.APIKey == "" {
		return nil, domain.ErrAcoustIDNotConfigured
	}

	query := url.Values{}
	query.Set("client", c.APIKey)
	query.Set("meta", "recordings")
	query.Set("fingerprint", fingerprint)
	query.Set("duration", strconv.Itoa(int(durationSeconds+0.5)))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, acoustIDLookupURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("building AcoustID request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AcoustID request failed: %w", err)
	}
	defer resp.Body.Close()

	var body acoustIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding AcoustID response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || body.Status != "ok" {
		if body.Error != nil {
			return nil, fmt.Errorf("AcoustID error (HTTP %d): %s", resp.StatusCode, body.Error.Message)
		}
		return nil, fmt.Errorf("AcoustID error: HTTP %d", resp.StatusCode)
	}

	var matches []usecases.AcoustIDMatch
	for _, result := range body.Results {
		for _, recording := range result.Recordings {
			if recording.ID == "" {
				continue
			}
			matches = append(matches, usecases.AcoustIDMatch{
				RecordingID: recording.ID,
				Score:       result.Score,
			})
		}
	}

	return matches, nil
}
