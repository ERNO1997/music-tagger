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

// identify upserts a new file and immediately records it as identified with
// meta, for seeding ListArtists/ListAlbums/ListTracks tests with resolved
// MBIDs — BulkApply alone always resets resolved metadata to blank.
func identify(t *testing.T, store *SQLiteStore, ctx context.Context, path string, meta usecases.RecordingMetadata) {
	t.Helper()
	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{{Path: path, Format: domain.FormatMP3, Status: domain.StatusNew}},
	}); err != nil {
		t.Fatalf("BulkApply(%s): %v", path, err)
	}
	if err := store.RecordIdentification(ctx, path, usecases.IdentificationResult{
		Status:   domain.StatusIdentified,
		Metadata: meta,
	}); err != nil {
		t.Fatalf("RecordIdentification(%s): %v", path, err)
	}
}

func TestListArtistsAlbumsTracks_MBIDBasedGroupingAndKeyDrillDown(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Two files sharing an artist name string but with different
	// ArtistMBIDs must stay separate; the ArtistMBID-grouped file's album
	// must be reachable via its ReleaseGroupMBID-based key.
	identify(t, store, ctx, "/music/a1.mp3", usecases.RecordingMetadata{
		Artist: "Overlap", Album: "Album One", Title: "T1", TrackNumber: 1,
		ArtistMBID: "artist-a", ReleaseGroupMBID: "rg-1", RecordingID: "rec-1",
	})
	identify(t, store, ctx, "/music/a2.mp3", usecases.RecordingMetadata{
		Artist: "Overlap", Album: "Album Two", Title: "T2", TrackNumber: 1,
		ArtistMBID: "artist-b", ReleaseGroupMBID: "rg-2", RecordingID: "rec-2",
	})
	// An unidentified file with only a raw tag, unrelated to the above.
	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{{
			Path: "/music/raw.mp3", Format: domain.FormatMP3, Status: domain.StatusNew,
			RawArtist: "Raw Artist", RawAlbum: "Raw Album",
		}},
	}); err != nil {
		t.Fatalf("BulkApply(raw): %v", err)
	}

	artists, err := store.ListArtists(ctx, usecases.LibraryFilter{})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists) != 3 {
		t.Fatalf("len(artists) = %d, want 3 (artist-a, artist-b, Raw Artist): %+v", len(artists), artists)
	}
	var artistA, artistB usecases.ArtistSummary
	for _, a := range artists {
		switch a.Key {
		case "artist-a":
			artistA = a
		case "artist-b":
			artistB = a
		}
	}
	if artistA.Key == "" || artistB.Key == "" {
		t.Fatalf("expected both artist-a and artist-b groupings, got %+v", artists)
	}
	if !artistA.LabelCollision || !artistB.LabelCollision {
		t.Errorf("both Overlap groupings should be flagged LabelCollision: a=%+v b=%+v", artistA, artistB)
	}

	albumsForA, err := store.ListAlbums(ctx, artistA.Key, usecases.LibraryFilter{})
	if err != nil {
		t.Fatalf("ListAlbums(artistA): %v", err)
	}
	if len(albumsForA) != 1 || albumsForA[0].Key != "rg-1" {
		t.Fatalf("albumsForA = %+v, want exactly rg-1", albumsForA)
	}

	tracks, err := store.ListTracks(ctx, artistA.Key, albumsForA[0].Key, usecases.LibraryFilter{})
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 1 || tracks[0].Path != "/music/a1.mp3" {
		t.Fatalf("tracks = %+v, want exactly a1.mp3", tracks)
	}

	// Backward-compatible name-based resolution must reach the same
	// grouping when unambiguous (unrelated raw-tag artist here).
	rawKey, err := store.ResolveArtistKey(ctx, "Raw Artist")
	if err != nil {
		t.Fatalf("ResolveArtistKey: %v", err)
	}
	rawAlbums, err := store.ListAlbums(ctx, rawKey, usecases.LibraryFilter{})
	if err != nil {
		t.Fatalf("ListAlbums(raw): %v", err)
	}
	if len(rawAlbums) != 1 || rawAlbums[0].Album != "Raw Album" {
		t.Fatalf("rawAlbums = %+v, want exactly Raw Album", rawAlbums)
	}
}
