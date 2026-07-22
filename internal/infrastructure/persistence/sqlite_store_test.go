package persistence

import (
	"context"
	"testing"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

func TestQueryPage_PathsFilterTakesPriorityOverOtherFields(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{
			{Path: "/music/a.mp3", Format: domain.FormatMP3, Status: domain.StatusNew},
			{Path: "/music/b.mp3", Format: domain.FormatMP3, Status: domain.StatusNew},
			{Path: "/music/c.mp3", Format: domain.FormatMP3, Status: domain.StatusNew},
		},
	}); err != nil {
		t.Fatalf("BulkApply: %v", err)
	}

	// b.mp3 is tagged; a.mp3 and c.mp3 are not. Filtering by Paths={a,b}
	// alongside an unrelated Tagged=false must still return both a and b,
	// ignoring the Tagged field entirely.
	if err := store.RecordTagged(ctx, "/music/b.mp3", true, ""); err != nil {
		t.Fatalf("RecordTagged: %v", err)
	}

	untagged := false
	entries, total, err := store.QueryPage(ctx, usecases.LibraryFilter{
		Paths:  []string{"/music/a.mp3", "/music/b.mp3"},
		Tagged: &untagged,
	}, usecases.LibrarySort{}, 50, 0)
	if err != nil {
		t.Fatalf("QueryPage: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	got := map[string]bool{entries[0].Path: true, entries[1].Path: true}
	if !got["/music/a.mp3"] || !got["/music/b.mp3"] {
		t.Fatalf("entries = %+v, want exactly a.mp3 and b.mp3", entries)
	}
}
