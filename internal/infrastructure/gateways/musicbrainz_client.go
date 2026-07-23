package gateways

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

// musicBrainzBaseURL is a var, not a const, solely so tests can point it at
// an httptest.Server instead of the real MusicBrainz host.
var musicBrainzBaseURL = "https://musicbrainz.org/ws/2"

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
	Title        string           `json:"title"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	Releases     []mbRelease      `json:"releases"`
}

type mbArtistCredit struct {
	Name       string   `json:"name"`
	JoinPhrase string   `json:"joinphrase"`
	Artist     mbArtist `json:"artist"`
}

type mbArtist struct {
	ID string `json:"id"`
}

type mbRelease struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Status       string           `json:"status"`
	Date         string           `json:"date"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	ReleaseGroup *mbReleaseGroup  `json:"release-group"`
	Media        []mbMedium       `json:"media"`
}

type mbReleaseGroup struct {
	ID          string `json:"id"`
	PrimaryType string `json:"primary-type"`
}

type mbMedium struct {
	Position   int       `json:"position"`
	TrackCount int       `json:"track-count"`
	Tracks     []mbTrack `json:"tracks"`
}

type mbTrack struct {
	Position  int             `json:"position"`
	Number    string          `json:"number"`
	Title     string          `json:"title"`
	Recording *mbRecordingRef `json:"recording"`
}

type mbRecordingRef struct {
	ID string `json:"id"`
}

func (c *MusicBrainzClient) Lookup(ctx context.Context, recordingID string) (usecases.RecordingMetadata, error) {
	if c.UserAgent == "" {
		return usecases.RecordingMetadata{}, domain.ErrMusicBrainzNotConfigured
	}
	return c.resolveRecording(ctx, recordingID)
}

// resolveRecording fetches one recording ID's full canonical metadata —
// the shared core of Lookup, also reused by Search to resolve each
// free-text search hit to the same metadata shape.
func (c *MusicBrainzClient) resolveRecording(ctx context.Context, recordingID string) (usecases.RecordingMetadata, error) {
	c.waitForRateGate()

	reqURL := fmt.Sprintf("%s/recording/%s?inc=releases+media+release-groups+artist-credits&fmt=json", musicBrainzBaseURL, recordingID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

	release, medium, track, ok := selectRelease(rec.Releases)
	if !ok {
		return usecases.RecordingMetadata{}, domain.ErrNoMusicBrainzRelease
	}

	var releaseGroupID string
	if release.ReleaseGroup != nil {
		releaseGroupID = release.ReleaseGroup.ID
	}

	return usecases.RecordingMetadata{
		RecordingID:      recordingID,
		Artist:           joinArtistCredit(rec.ArtistCredit),
		Album:            release.Title,
		Title:            rec.Title,
		TrackNumber:      trackNumber(track),
		AlbumArtist:      joinArtistCredit(release.ArtistCredit),
		Year:             parseYear(release.Date),
		DiscNumber:       medium.Position,
		TotalDiscs:       len(release.Media),
		TotalTracks:      medium.TrackCount,
		ReleaseMBID:      release.ID,
		ReleaseGroupMBID: releaseGroupID,
		ArtistMBID:       firstArtistID(rec.ArtistCredit),
	}, nil
}

type mbSearchResponse struct {
	Recordings []mbSearchHit `json:"recordings"`
}

type mbSearchHit struct {
	ID string `json:"id"`
}

// Search resolves a free-text query directly to candidate recordings,
// independent of any AcoustID fingerprint match. It first queries
// MusicBrainz's recording search endpoint for matching recording IDs
// (already ranked by MusicBrainz's own relevance order), then resolves
// each one to full metadata via the same path Lookup uses. A hit with no
// resolvable release (e.g. a bare instrumental entry) is skipped rather
// than aborting the whole search — mirroring how tied-recording
// disambiguation already tolerates individual unresolvable recordings.
func (c *MusicBrainzClient) Search(ctx context.Context, query string, limit int) ([]usecases.RecordingMetadata, error) {
	if c.UserAgent == "" {
		return nil, domain.ErrMusicBrainzNotConfigured
	}

	c.waitForRateGate()

	reqURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=%d", musicBrainzBaseURL, url.QueryEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building MusicBrainz search request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MusicBrainz search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz search error: HTTP %d", resp.StatusCode)
	}

	var body mbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding MusicBrainz search response: %w", err)
	}

	var results []usecases.RecordingMetadata
	for _, hit := range body.Recordings {
		metadata, err := c.resolveRecording(ctx, hit.ID)
		if err != nil {
			if errors.Is(err, domain.ErrNoMusicBrainzRelease) {
				continue
			}
			return nil, err
		}
		results = append(results, metadata)
	}
	return results, nil
}

type mbReleaseGroupReleases struct {
	Releases []mbReleaseGroupReleaseSummary `json:"releases"`
}

type mbReleaseGroupReleaseSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Date   string `json:"date"`
}

// Releases resolves a release-group's sibling releases, for browsing
// alternate cover art across editions.
func (c *MusicBrainzClient) Releases(ctx context.Context, releaseGroupMBID string) ([]usecases.ReleaseGroupRelease, error) {
	if c.UserAgent == "" {
		return nil, domain.ErrMusicBrainzNotConfigured
	}

	c.waitForRateGate()

	url := fmt.Sprintf("%s/release-group/%s?inc=releases&fmt=json", musicBrainzBaseURL, releaseGroupMBID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MusicBrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz error: HTTP %d", resp.StatusCode)
	}

	var body mbReleaseGroupReleases
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding MusicBrainz response: %w", err)
	}

	releases := make([]usecases.ReleaseGroupRelease, 0, len(body.Releases))
	for _, r := range body.Releases {
		releases = append(releases, usecases.ReleaseGroupRelease{
			ReleaseMBID: r.ID,
			Title:       r.Title,
			Status:      r.Status,
			Date:        r.Date,
		})
	}
	return releases, nil
}

type mbArtistLookup struct {
	ReleaseGroups     []mbReleaseGroupFull `json:"release-groups"`
	ReleaseGroupCount int                  `json:"release-group-count"`
}

type mbReleaseGroupFull struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
}

// artistReleaseGroupPageSize is the number of release-groups requested per
// page from MusicBrainz's artist lookup.
const artistReleaseGroupPageSize = 100

// artistReleaseGroupPageLimit bounds how many pages ArtistReleaseGroups will
// follow for a single artist, guarding against unbounded looping rather than
// exhaustively paginating an unusually prolific artist's entire discography.
const artistReleaseGroupPageLimit = 5

// excludedReleaseGroupSecondaryTypes are secondary types excluded from an
// artist's discography completeness check by default, per design.md's
// Album/EP-only default — compilations, live recordings, and similar
// non-original releases would otherwise clutter a "missing albums" list.
var excludedReleaseGroupSecondaryTypes = map[string]bool{
	"Compilation":    true,
	"Live":           true,
	"Remix":          true,
	"Soundtrack":     true,
	"DJ-mix":         true,
	"Mixtape/Street": true,
}

// ArtistReleaseGroups resolves an artist's release-groups (albums), filtered
// to official primary-type Album/EP release-groups excluding the secondary
// types in excludedReleaseGroupSecondaryTypes, for use in comparing the
// artist's MusicBrainz discography against the local library. Follows
// pagination internally (each page subject to the centralized rate limit),
// up to artistReleaseGroupPageLimit pages.
func (c *MusicBrainzClient) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]usecases.ArtistReleaseGroupSummary, error) {
	if c.UserAgent == "" {
		return nil, domain.ErrMusicBrainzNotConfigured
	}

	var all []mbReleaseGroupFull
	offset := 0
	for page := 0; page < artistReleaseGroupPageLimit; page++ {
		c.waitForRateGate()

		reqURL := fmt.Sprintf("%s/artist/%s?inc=release-groups&fmt=json&limit=%d&offset=%d",
			musicBrainzBaseURL, artistMBID, artistReleaseGroupPageSize, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("building MusicBrainz artist request: %w", err)
		}
		req.Header.Set("User-Agent", c.UserAgent)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("MusicBrainz artist request failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("MusicBrainz error: HTTP %d", resp.StatusCode)
		}
		var body mbArtistLookup
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding MusicBrainz artist response: %w", err)
		}

		all = append(all, body.ReleaseGroups...)
		offset += len(body.ReleaseGroups)
		if len(body.ReleaseGroups) == 0 || offset >= body.ReleaseGroupCount {
			break
		}
	}

	summaries := make([]usecases.ArtistReleaseGroupSummary, 0, len(all))
	for _, rg := range all {
		if rg.PrimaryType != "Album" && rg.PrimaryType != "EP" {
			continue
		}
		excluded := false
		for _, st := range rg.SecondaryTypes {
			if excludedReleaseGroupSecondaryTypes[st] {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		summaries = append(summaries, usecases.ArtistReleaseGroupSummary{
			ReleaseGroupMBID: rg.ID,
			Title:            rg.Title,
			Year:             parseYear(rg.FirstReleaseDate),
		})
	}
	return summaries, nil
}

// ReleaseTracklist resolves a release's full tracklist (recording MBID,
// title, and track number for every track across all media), for use in
// comparing a release's tracks against the local library's tracks for that
// album.
func (c *MusicBrainzClient) ReleaseTracklist(ctx context.Context, releaseMBID string) ([]usecases.ReleaseTrackSummary, error) {
	if c.UserAgent == "" {
		return nil, domain.ErrMusicBrainzNotConfigured
	}

	c.waitForRateGate()

	reqURL := fmt.Sprintf("%s/release/%s?inc=recordings&fmt=json", musicBrainzBaseURL, releaseMBID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building MusicBrainz release request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MusicBrainz release request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz error: HTTP %d", resp.StatusCode)
	}

	var release mbRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding MusicBrainz release response: %w", err)
	}

	var tracks []usecases.ReleaseTrackSummary
	for _, medium := range release.Media {
		for _, t := range medium.Tracks {
			recordingID := ""
			if t.Recording != nil {
				recordingID = t.Recording.ID
			}
			tracks = append(tracks, usecases.ReleaseTrackSummary{
				RecordingMBID: recordingID,
				Title:         t.Title,
				TrackNumber:   trackNumber(t),
			})
		}
	}
	return tracks, nil
}

// selectRelease prefers a release whose release-group primary type is
// "Album" and status is "Official", falling back to the first release
// with at least one track, per design.md's release-selection heuristic.
// It returns the medium alongside the release and track because disc
// number and total-tracks-on-that-disc are medium-level fields, while
// total-discs is len(release.Media) computed by the caller.
func selectRelease(releases []mbRelease) (mbRelease, mbMedium, mbTrack, bool) {
	var fallbackRelease mbRelease
	var fallbackMedium mbMedium
	var fallbackTrack mbTrack
	haveFallback := false

	for _, release := range releases {
		medium, track, ok := firstTrack(release)
		if !ok {
			continue
		}
		if !haveFallback {
			fallbackRelease, fallbackMedium, fallbackTrack, haveFallback = release, medium, track, true
		}
		isAlbum := release.ReleaseGroup != nil && release.ReleaseGroup.PrimaryType == "Album"
		if isAlbum && release.Status == "Official" {
			return release, medium, track, true
		}
	}

	return fallbackRelease, fallbackMedium, fallbackTrack, haveFallback
}

func firstTrack(release mbRelease) (mbMedium, mbTrack, bool) {
	for _, medium := range release.Media {
		if len(medium.Tracks) > 0 {
			return medium, medium.Tracks[0], true
		}
	}
	return mbMedium{}, mbTrack{}, false
}

// parseYear extracts the year from a MusicBrainz date string, which may be
// "YYYY", "YYYY-MM", or "YYYY-MM-DD". Returns 0 if empty or unparseable —
// a missing/partial date is common enough in MusicBrainz data that this
// must be a soft failure, not an error.
func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

// firstArtistID returns the first artist credit's MBID, or "" if none.
func firstArtistID(credits []mbArtistCredit) string {
	if len(credits) == 0 {
		return ""
	}
	return credits[0].Artist.ID
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

var (
	_ usecases.MusicBrainzLookup             = (*MusicBrainzClient)(nil)
	_ usecases.MusicBrainzReleaseGroupLookup = (*MusicBrainzClient)(nil)
	_ usecases.MusicBrainzSearch             = (*MusicBrainzClient)(nil)
	_ usecases.MusicBrainzDiscographyLookup  = (*MusicBrainzClient)(nil)
)
