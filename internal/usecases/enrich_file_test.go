package usecases

import (
	"context"
	"testing"
)

// fakeEnrichStore is a minimal TrackingStore fake recording RecordCoverArt/
// RecordLyrics calls, to assert Enrich does (or doesn't) write them.
type fakeEnrichStore struct {
	TrackingStore
	coverArtCalls int
	lyricsCalls   int
}

func (s *fakeEnrichStore) RecordCoverArt(ctx context.Context, path string, coverArtPath string) error {
	s.coverArtCalls++
	return nil
}

func (s *fakeEnrichStore) RecordLyrics(ctx context.Context, path string, lyrics string, syncedLyrics string) error {
	s.lyricsCalls++
	return nil
}

type fakeCoverArtStore struct{}

func (fakeCoverArtStore) Path(releaseMBID string) (string, bool) { return "", false }
func (fakeCoverArtStore) Save(releaseMBID string, data []byte) (string, error) {
	return "/covers/" + releaseMBID + ".jpg", nil
}

type fakeCoverArtLookup struct{ data []byte }

func (f fakeCoverArtLookup) Lookup(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]byte, error) {
	return f.data, nil
}

type fakeLyricsLookup struct{ plain string }

func (f fakeLyricsLookup) Lookup(ctx context.Context, artist, title, album string, durationSeconds int) (string, string, bool, error) {
	return f.plain, "", f.plain != "", nil
}

func TestEnrichFile_DoesNotOverwriteExistingCoverArtOrLyrics(t *testing.T) {
	store := &fakeEnrichStore{}
	enrich := NewEnrichFile(fakeCoverArtLookup{data: []byte("new-cover")}, fakeCoverArtStore{}, fakeLyricsLookup{plain: "new lyrics"}, store)

	err := enrich.Enrich(context.Background(), EnrichmentInput{
		Path:                 "/music/a.mp3",
		ReleaseMBID:          "release-1",
		ExistingCoverArtPath: "/covers/already-there.jpg",
		ExistingLyrics:       "already there lyrics",
	})
	if err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}
	if store.coverArtCalls != 0 {
		t.Fatalf("RecordCoverArt called %d times; want 0 since a value is already stored", store.coverArtCalls)
	}
	if store.lyricsCalls != 0 {
		t.Fatalf("RecordLyrics called %d times; want 0 since a value is already stored", store.lyricsCalls)
	}
}

func TestEnrichFile_WritesCoverArtAndLyricsWhenAbsent(t *testing.T) {
	store := &fakeEnrichStore{}
	enrich := NewEnrichFile(fakeCoverArtLookup{data: []byte("new-cover")}, fakeCoverArtStore{}, fakeLyricsLookup{plain: "new lyrics"}, store)

	err := enrich.Enrich(context.Background(), EnrichmentInput{
		Path:        "/music/a.mp3",
		ReleaseMBID: "release-1",
	})
	if err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}
	if store.coverArtCalls != 1 {
		t.Fatalf("RecordCoverArt called %d times; want 1", store.coverArtCalls)
	}
	if store.lyricsCalls != 1 {
		t.Fatalf("RecordLyrics called %d times; want 1", store.lyricsCalls)
	}
}
