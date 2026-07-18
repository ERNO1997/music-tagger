package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

const musicBrainzBaseURL = "https://musicbrainz.org/ws/2"

// musicBrainzMinInterval is the minimum time between requests to
// MusicBrainz, enforced centrally inside this client regardless of caller,
// per project.md §4.2's 1 req/sec limit.
const musicBrainzMinInterval = 1 * time.Second

// MusicBrainzClient resolves a Recording ID to canonical artist/release/
// track metadata via the MusicBrainz web service. A single instance's rate
// gate is shared across every call made through it — construct one
// instance and reuse it for every caller.
type MusicBrainzClient struct {
	UserAgent  string
	HTTPClient *http.Client

	rateMu   sync.Mutex
	nextCall time.Time
}

func NewMusicBrainzClient(userAgent string) *MusicBrainzClient {
	return &MusicBrainzClient{UserAgent: userAgent, HTTPClient: http.DefaultClient}
}

type mbRecording struct {
	Title        string          `json:"title"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	Releases     []mbRelease      `json:"releases"`
}

type mbArtistCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
}

type mbRelease struct {
	Title        string          `json:"title"`
	Status       string          `json:"status"`
	ReleaseGroup *mbReleaseGroup `json:"release-group"`
	Media        []mbMedium      `json:"media"`
}

type mbReleaseGroup struct {
	PrimaryType string `json:"primary-type"`
}

type mbMedium struct {
	Tracks []mbTrack `json:"tracks"`
}

type mbTrack struct {
	Position int    `json:"position"`
	Number   string `json:"number"`
}

func (c *MusicBrainzClient) Lookup(ctx context.Context, recordingID string) (usecases.RecordingMetadata, error) {
	if c.UserAgent == "" {
		return usecases.RecordingMetadata{}, domain.ErrMusicBrainzNotConfigured
	}

	c.waitForRateGate()

	url := fmt.Sprintf("%s/recording/%s?inc=releases+media+release-groups+artist-credits&fmt=json", musicBrainzBaseURL, recordingID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return usecases.RecordingMetadata{}, fmt.Errorf("building MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return usecases.RecordingMetadata{}, fmt.Errorf("MusicBrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return usecases.RecordingMetadata{}, fmt.Errorf("MusicBrainz error: HTTP %d", resp.StatusCode)
	}

	var rec mbRecording
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return usecases.RecordingMetadata{}, fmt.Errorf("decoding MusicBrainz response: %w", err)
	}

	release, track, ok := selectRelease(rec.Releases)
	if !ok {
		return usecases.RecordingMetadata{}, domain.ErrNoMusicBrainzRelease
	}

	return usecases.RecordingMetadata{
		RecordingID: recordingID,
		Artist:      joinArtistCredit(rec.ArtistCredit),
		Album:       release.Title,
		Title:       rec.Title,
		TrackNumber: trackNumber(track),
	}, nil
}

// selectRelease prefers a release whose release-group primary type is
// "Album" and status is "Official", falling back to the first release
// with at least one track, per design.md's release-selection heuristic.
func selectRelease(releases []mbRelease) (mbRelease, mbTrack, bool) {
	var fallbackRelease mbRelease
	var fallbackTrack mbTrack
	haveFallback := false

	for _, release := range releases {
		track, ok := firstTrack(release)
		if !ok {
			continue
		}
		if !haveFallback {
			fallbackRelease, fallbackTrack, haveFallback = release, track, true
		}
		isAlbum := release.ReleaseGroup != nil && release.ReleaseGroup.PrimaryType == "Album"
		if isAlbum && release.Status == "Official" {
			return release, track, true
		}
	}

	return fallbackRelease, fallbackTrack, haveFallback
}

func firstTrack(release mbRelease) (mbTrack, bool) {
	for _, medium := range release.Media {
		if len(medium.Tracks) > 0 {
			return medium.Tracks[0], true
		}
	}
	return mbTrack{}, false
}

func trackNumber(track mbTrack) int {
	if track.Position > 0 {
		return track.Position
	}
	if n, err := strconv.Atoi(track.Number); err == nil {
		return n
	}
	return 0
}

func joinArtistCredit(credits []mbArtistCredit) string {
	artist := ""
	for _, credit := range credits {
		artist += credit.Name + credit.JoinPhrase
	}
	return artist
}

// waitForRateGate blocks until at least musicBrainzMinInterval has elapsed
// since the last call through this client instance, then reserves the next
// slot. This is the centralized rate limit required by project.md §4.2 —
// every caller through this shared instance is paced identically.
func (c *MusicBrainzClient) waitForRateGate() {
	c.rateMu.Lock()
	now := time.Now()
	var wait time.Duration
	if now.Before(c.nextCall) {
		wait = c.nextCall.Sub(now)
	}
	c.nextCall = now.Add(wait).Add(musicBrainzMinInterval)
	c.rateMu.Unlock()

	if wait > 0 {
		time.Sleep(wait)
	}
}
