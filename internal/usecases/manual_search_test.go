package usecases

import (
	"context"
	"testing"

	"music-tagger/internal/domain"
)

type fakeMusicBrainzSearch struct {
	results []RecordingMetadata
}

func (f fakeMusicBrainzSearch) Search(ctx context.Context, query string, limit int) ([]RecordingMetadata, error) {
	return f.results, nil
}

func TestManualSearch_WithResults_RecordsAmbiguous(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Status: domain.StatusNotFound}}
	search := fakeMusicBrainzSearch{results: []RecordingMetadata{
		{RecordingID: "rec-1", Artist: "Artist One", Title: "Song One"},
		{RecordingID: "rec-2", Artist: "Artist Two", Title: "Song Two"},
	}}

	manualSearch := NewManualSearch(search, store)

	candidates, found, err := manualSearch.Search(context.Background(), "/music/a.mp3", "artist:\"Artist One\"")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if !found {
		t.Fatalf("Search reported found=false for a tracked path")
	}
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates; want 2", len(candidates))
	}
	if len(store.recordedCandidates) != 2 {
		t.Fatalf("RecordAmbiguous was called with %d candidates; want 2", len(store.recordedCandidates))
	}
}

func TestManualSearch_NoResults_DoesNotRecordAmbiguous(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Status: domain.StatusIdentified, Artist: "Existing Artist"}}
	search := fakeMusicBrainzSearch{results: nil}

	manualSearch := NewManualSearch(search, store)

	candidates, found, err := manualSearch.Search(context.Background(), "/music/a.mp3", "nonsense query")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if !found {
		t.Fatalf("Search reported found=false for a tracked path")
	}
	if len(candidates) != 0 {
		t.Fatalf("got %d candidates; want 0", len(candidates))
	}
	if store.recordedCandidates != nil {
		t.Fatalf("RecordAmbiguous was called (candidates=%+v); want no call for a zero-result search", store.recordedCandidates)
	}
}

func TestManualSearch_UntrackedPath_ReturnsNotFoundWithoutSearching(t *testing.T) {
	store := &fakeTrackingStore{notFound: true}
	searchCalls := 0
	search := countingMusicBrainzSearch{calls: &searchCalls}

	manualSearch := NewManualSearch(search, store)

	candidates, found, err := manualSearch.Search(context.Background(), "/music/unknown.mp3", "some query")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if found {
		t.Fatalf("Search reported found=true for an untracked path")
	}
	if candidates != nil {
		t.Fatalf("got candidates %+v; want nil for an untracked path", candidates)
	}
	if searchCalls != 0 {
		t.Fatalf("MusicBrainzSearch.Search was called %d times; want 0 for an untracked path", searchCalls)
	}
}

type countingMusicBrainzSearch struct {
	calls *int
}

func (f countingMusicBrainzSearch) Search(ctx context.Context, query string, limit int) ([]RecordingMetadata, error) {
	*f.calls++
	return nil, nil
}
