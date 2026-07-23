package usecases

import (
	"context"
	"errors"
	"testing"

	"music-tagger/internal/domain"
)

// fakeCompletenessStore is a minimal TrackingStore fake exercising only
// ListAlbums/ListTracks, as used by CompletenessChecker.
type fakeCompletenessStore struct {
	TrackingStore
	albumsByArtistKey map[string][]AlbumSummary
	tracksByAlbumKey  map[string][]domain.FileRecord
}

func (s *fakeCompletenessStore) ListAlbums(ctx context.Context, artistKey string, filter LibraryFilter) ([]AlbumSummary, error) {
	return s.albumsByArtistKey[artistKey], nil
}

func (s *fakeCompletenessStore) ListTracks(ctx context.Context, artistKey, albumKey string, filter LibraryFilter) ([]domain.FileRecord, error) {
	return s.tracksByAlbumKey[albumKey], nil
}

// fakeDiscographyLookup returns canned responses, counting calls per MBID
// so tests can assert the cache avoids redundant lookups.
type fakeDiscographyLookup struct {
	releaseGroups map[string][]ArtistReleaseGroupSummary
	tracklists    map[string][]ReleaseTrackSummary
	artistCalls   map[string]int
	releaseCalls  map[string]int
	failArtist    map[string]bool
	failRelease   map[string]bool
}

func newFakeDiscographyLookup() *fakeDiscographyLookup {
	return &fakeDiscographyLookup{
		releaseGroups: map[string][]ArtistReleaseGroupSummary{},
		tracklists:    map[string][]ReleaseTrackSummary{},
		artistCalls:   map[string]int{},
		releaseCalls:  map[string]int{},
		failArtist:    map[string]bool{},
		failRelease:   map[string]bool{},
	}
}

func (f *fakeDiscographyLookup) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]ArtistReleaseGroupSummary, error) {
	f.artistCalls[artistMBID]++
	if f.failArtist[artistMBID] {
		return nil, errors.New("simulated MusicBrainz failure")
	}
	return f.releaseGroups[artistMBID], nil
}

func (f *fakeDiscographyLookup) ReleaseTracklist(ctx context.Context, releaseMBID string) ([]ReleaseTrackSummary, error) {
	f.releaseCalls[releaseMBID]++
	if f.failRelease[releaseMBID] {
		return nil, errors.New("simulated MusicBrainz failure")
	}
	return f.tracklists[releaseMBID], nil
}

func TestArtistCompleteness_PartialAndFull(t *testing.T) {
	store := &fakeCompletenessStore{
		albumsByArtistKey: map[string][]AlbumSummary{
			"artist-1": {{Key: "rg-owned", Album: "Owned Album"}},
		},
	}
	lookup := newFakeDiscographyLookup()
	lookup.releaseGroups["artist-1"] = []ArtistReleaseGroupSummary{
		{ReleaseGroupMBID: "rg-owned", Title: "Owned Album", Year: 2001},
		{ReleaseGroupMBID: "rg-missing", Title: "Missing Album", Year: 2005},
	}

	checker := NewCompletenessChecker(store, lookup)
	result, err := checker.ArtistCompleteness(context.Background(), "artist-1", false)
	if err != nil {
		t.Fatalf("ArtistCompleteness: %v", err)
	}
	if result.OwnedAlbums != 1 || result.TotalAlbums != 2 {
		t.Fatalf("result = %+v, want owned=1 total=2", result)
	}
	if len(result.Missing) != 1 || result.Missing[0].Title != "Missing Album" {
		t.Fatalf("Missing = %+v, want exactly Missing Album", result.Missing)
	}

	// Full completeness: owning everything on MusicBrainz.
	store.albumsByArtistKey["artist-2"] = []AlbumSummary{{Key: "rg-only"}}
	lookup.releaseGroups["artist-2"] = []ArtistReleaseGroupSummary{{ReleaseGroupMBID: "rg-only", Title: "Only Album"}}
	full, err := checker.ArtistCompleteness(context.Background(), "artist-2", false)
	if err != nil {
		t.Fatalf("ArtistCompleteness (full): %v", err)
	}
	if full.OwnedAlbums != 1 || full.TotalAlbums != 1 || len(full.Missing) != 0 {
		t.Fatalf("full = %+v, want nothing missing", full)
	}
}

func TestArtistCompleteness_UnavailableForNameDerivedGroup(t *testing.T) {
	checker := NewCompletenessChecker(&fakeCompletenessStore{}, newFakeDiscographyLookup())
	_, err := checker.ArtistCompleteness(context.Background(), "name:Some Artist", false)
	if !errors.Is(err, ErrCompletenessUnavailable) {
		t.Fatalf("err = %v, want ErrCompletenessUnavailable", err)
	}
}

func TestArtistCompleteness_GatewayFailureIsDistinctError(t *testing.T) {
	lookup := newFakeDiscographyLookup()
	lookup.failArtist["artist-1"] = true
	checker := NewCompletenessChecker(&fakeCompletenessStore{}, lookup)

	_, err := checker.ArtistCompleteness(context.Background(), "artist-1", false)
	if err == nil {
		t.Fatal("expected an error from a failing gateway, got nil")
	}
	if errors.Is(err, ErrCompletenessUnavailable) {
		t.Fatal("a gateway failure must not look like ErrCompletenessUnavailable")
	}
}

func TestArtistCompleteness_CacheAvoidsSecondGatewayCall(t *testing.T) {
	store := &fakeCompletenessStore{albumsByArtistKey: map[string][]AlbumSummary{"artist-1": nil}}
	lookup := newFakeDiscographyLookup()
	lookup.releaseGroups["artist-1"] = []ArtistReleaseGroupSummary{{ReleaseGroupMBID: "rg-1", Title: "Album"}}
	checker := NewCompletenessChecker(store, lookup)

	if _, err := checker.ArtistCompleteness(context.Background(), "artist-1", false); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := checker.ArtistCompleteness(context.Background(), "artist-1", false); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if lookup.artistCalls["artist-1"] != 1 {
		t.Fatalf("artistCalls = %d, want 1 (second call should hit the cache)", lookup.artistCalls["artist-1"])
	}

	// A manual recheck (refresh=true) must bypass the cache.
	if _, err := checker.ArtistCompleteness(context.Background(), "artist-1", true); err != nil {
		t.Fatalf("refresh call: %v", err)
	}
	if lookup.artistCalls["artist-1"] != 2 {
		t.Fatalf("artistCalls = %d, want 2 after a manual refresh", lookup.artistCalls["artist-1"])
	}
}

func TestAlbumCompleteness_PartialAndReleaseMismatch(t *testing.T) {
	store := &fakeCompletenessStore{
		tracksByAlbumKey: map[string][]domain.FileRecord{
			"rg-1": {
				{RecordingMBID: "rec-owned", ReleaseMBID: "release-1"},
				{RecordingMBID: "rec-owned-2", ReleaseMBID: "release-1"},
				{RecordingMBID: "rec-other-edition", ReleaseMBID: "release-2"},
			},
		},
	}
	lookup := newFakeDiscographyLookup()
	lookup.tracklists["release-1"] = []ReleaseTrackSummary{
		{RecordingMBID: "rec-owned", Title: "Track One", TrackNumber: 1},
		{RecordingMBID: "rec-owned-2", Title: "Track Two", TrackNumber: 2},
		{RecordingMBID: "rec-missing", Title: "Track Three", TrackNumber: 3},
	}

	checker := NewCompletenessChecker(store, lookup)
	result, err := checker.AlbumCompleteness(context.Background(), "artist-1", "rg-1", false)
	if err != nil {
		t.Fatalf("AlbumCompleteness: %v", err)
	}
	if result.OwnedTracks != 2 || result.TotalTracks != 3 {
		t.Fatalf("result = %+v, want owned=2 total=3", result)
	}
	if len(result.Missing) != 1 || result.Missing[0].Title != "Track Three" {
		t.Fatalf("Missing = %+v, want exactly Track Three", result.Missing)
	}
	if !result.ReleaseMismatch {
		t.Errorf("ReleaseMismatch should be true: two distinct ReleaseMBIDs observed among local tracks")
	}
}

func TestAlbumCompleteness_UnavailableForNameDerivedGroup(t *testing.T) {
	checker := NewCompletenessChecker(&fakeCompletenessStore{}, newFakeDiscographyLookup())
	_, err := checker.AlbumCompleteness(context.Background(), "artist-1", "name:Some Album", false)
	if !errors.Is(err, ErrCompletenessUnavailable) {
		t.Fatalf("err = %v, want ErrCompletenessUnavailable", err)
	}
}
