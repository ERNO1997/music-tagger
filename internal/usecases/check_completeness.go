package usecases

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrCompletenessUnavailable is returned when a completeness check is
// requested for a grouping with no MusicBrainz ID (a name-derived,
// unidentified grouping) — there is nothing to look up on MusicBrainz.
var ErrCompletenessUnavailable = errors.New("completeness check unavailable: grouping has no MusicBrainz ID")

// completenessCacheTTL bounds how long a completeness result is reused
// across repeat navigation before a fresh MusicBrainz lookup is made again,
// per design.md decision 7.
const completenessCacheTTL = 15 * time.Minute

// MissingAlbum is one MusicBrainz release-group not present in the local
// library, as surfaced by an artist completeness check.
type MissingAlbum struct {
	Title string
	Year  int
}

// ArtistCompletenessResult is the outcome of comparing an artist's local
// albums against its MusicBrainz discography.
type ArtistCompletenessResult struct {
	OwnedAlbums int
	TotalAlbums int
	Missing     []MissingAlbum
}

// MissingTrack is one MusicBrainz recording not present in the local
// library, as surfaced by an album completeness check.
type MissingTrack struct {
	Title       string
	TrackNumber int
}

// AlbumCompletenessResult is the outcome of comparing an album's local
// tracks against its MusicBrainz release tracklist.
type AlbumCompletenessResult struct {
	OwnedTracks int
	TotalTracks int
	Missing     []MissingTrack

	// ReleaseMismatch is true when the album's own identified tracks
	// disagree on ReleaseMBID — the tracklist comparison necessarily used
	// only the most-frequent one, so a "missing" track here might really
	// just be on a different edition. Surfaced rather than hidden, per the
	// same "don't silently resolve a mismatch" rule grouping follows.
	ReleaseMismatch bool
}

type completenessCacheEntry struct {
	value   any
	expires time.Time
}

// CompletenessChecker compares the local library against MusicBrainz's own
// catalog for a given artist or album grouping, cached briefly in-process
// to absorb repeat navigation without re-hitting MusicBrainz's rate-gated
// API on every visit.
type CompletenessChecker struct {
	store  TrackingStore
	lookup MusicBrainzDiscographyLookup

	mu    sync.Mutex
	cache map[string]completenessCacheEntry
}

func NewCompletenessChecker(store TrackingStore, lookup MusicBrainzDiscographyLookup) *CompletenessChecker {
	return &CompletenessChecker{store: store, lookup: lookup, cache: map[string]completenessCacheEntry{}}
}

// isMBIDKey reports whether key (as produced by GroupArtists/GroupAlbums)
// is a real MusicBrainz ID rather than a "name:"-prefixed fallback key for
// an unidentified grouping.
func isMBIDKey(key string) bool {
	return key != "" && !strings.HasPrefix(key, "name:")
}

// ArtistCompleteness compares artistKey's locally-owned albums against its
// full MusicBrainz discography. refresh bypasses the cache, for a
// user-triggered manual recheck.
func (c *CompletenessChecker) ArtistCompleteness(ctx context.Context, artistKey string, refresh bool) (ArtistCompletenessResult, error) {
	if !isMBIDKey(artistKey) {
		return ArtistCompletenessResult{}, ErrCompletenessUnavailable
	}

	cacheKey := "artist:" + artistKey
	if !refresh {
		if cached, ok := c.getCached(cacheKey); ok {
			return cached.(ArtistCompletenessResult), nil
		}
	}

	albums, err := c.store.ListAlbums(ctx, artistKey, LibraryFilter{})
	if err != nil {
		return ArtistCompletenessResult{}, fmt.Errorf("loading local albums: %w", err)
	}
	owned := map[string]bool{}
	for _, a := range albums {
		if isMBIDKey(a.Key) {
			owned[a.Key] = true
		}
	}

	discography, err := c.lookup.ArtistReleaseGroups(ctx, artistKey)
	if err != nil {
		return ArtistCompletenessResult{}, fmt.Errorf("fetching MusicBrainz discography: %w", err)
	}

	result := ArtistCompletenessResult{TotalAlbums: len(discography)}
	for _, rg := range discography {
		if owned[rg.ReleaseGroupMBID] {
			result.OwnedAlbums++
		} else {
			result.Missing = append(result.Missing, MissingAlbum{Title: rg.Title, Year: rg.Year})
		}
	}
	sort.Slice(result.Missing, func(i, j int) bool { return result.Missing[i].Title < result.Missing[j].Title })

	c.setCached(cacheKey, result)
	return result, nil
}

// AlbumCompleteness compares albumKey's (within artistKey) locally-owned
// tracks against its MusicBrainz release tracklist. refresh bypasses the
// cache, for a user-triggered manual recheck.
func (c *CompletenessChecker) AlbumCompleteness(ctx context.Context, artistKey, albumKey string, refresh bool) (AlbumCompletenessResult, error) {
	if !isMBIDKey(albumKey) {
		return AlbumCompletenessResult{}, ErrCompletenessUnavailable
	}

	cacheKey := "album:" + albumKey
	if !refresh {
		if cached, ok := c.getCached(cacheKey); ok {
			return cached.(AlbumCompletenessResult), nil
		}
	}

	tracks, err := c.store.ListTracks(ctx, artistKey, albumKey, LibraryFilter{})
	if err != nil {
		return AlbumCompletenessResult{}, fmt.Errorf("loading local tracks: %w", err)
	}

	owned := map[string]bool{}
	releaseCounts := map[string]int{}
	for _, t := range tracks {
		if t.RecordingMBID != "" {
			owned[t.RecordingMBID] = true
		}
		if t.ReleaseMBID != "" {
			releaseCounts[t.ReleaseMBID]++
		}
	}
	releaseMBID, distinctReleases := mostFrequentKey(releaseCounts)

	var tracklist []ReleaseTrackSummary
	if releaseMBID != "" {
		tracklist, err = c.lookup.ReleaseTracklist(ctx, releaseMBID)
		if err != nil {
			return AlbumCompletenessResult{}, fmt.Errorf("fetching MusicBrainz tracklist: %w", err)
		}
	}

	result := AlbumCompletenessResult{
		TotalTracks:     len(tracklist),
		ReleaseMismatch: distinctReleases > 1,
	}
	for _, tr := range tracklist {
		if owned[tr.RecordingMBID] {
			result.OwnedTracks++
		} else {
			result.Missing = append(result.Missing, MissingTrack{Title: tr.Title, TrackNumber: tr.TrackNumber})
		}
	}
	sort.Slice(result.Missing, func(i, j int) bool { return result.Missing[i].TrackNumber < result.Missing[j].TrackNumber })

	c.setCached(cacheKey, result)
	return result, nil
}

// mostFrequentKey returns the most-frequent key in counts (ties broken by
// map iteration order, which is fine here — any tied choice is equally
// arbitrary) alongside the number of distinct keys observed.
func mostFrequentKey(counts map[string]int) (string, int) {
	best := ""
	bestCount := -1
	for k, n := range counts {
		if n > bestCount {
			best, bestCount = k, n
		}
	}
	return best, len(counts)
}

func (c *CompletenessChecker) getCached(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.value, true
}

func (c *CompletenessChecker) setCached(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = completenessCacheEntry{value: value, expires: time.Now().Add(completenessCacheTTL)}
}
