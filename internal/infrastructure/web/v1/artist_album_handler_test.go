package v1

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/domain"
	"music-tagger/internal/infrastructure/persistence"
	"music-tagger/internal/usecases"
)

// fakeDiscographyLookup returns canned responses for the ArtistAlbumHandler
// completeness endpoint tests, without hitting MusicBrainz.
type fakeDiscographyLookup struct {
	releaseGroups map[string][]usecases.ArtistReleaseGroupSummary
	tracklists    map[string][]usecases.ReleaseTrackSummary
}

func (f *fakeDiscographyLookup) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]usecases.ArtistReleaseGroupSummary, error) {
	return f.releaseGroups[artistMBID], nil
}

func (f *fakeDiscographyLookup) ReleaseTracklist(ctx context.Context, releaseMBID string) ([]usecases.ReleaseTrackSummary, error) {
	return f.tracklists[releaseMBID], nil
}

func newTestArtistAlbumHandler(t *testing.T, lookup *fakeDiscographyLookup) (*ArtistAlbumHandler, *persistence.SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	store, err := persistence.NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if lookup == nil {
		lookup = &fakeDiscographyLookup{}
	}
	checker := usecases.NewCompletenessChecker(store, lookup)
	return NewArtistAlbumHandler(store, checker), store
}

func identifyForTest(t *testing.T, store *persistence.SQLiteStore, path string, meta usecases.RecordingMetadata) {
	t.Helper()
	ctx := context.Background()
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

func TestArtistAlbumHandler_KeyAndNameBasedDrillDown(t *testing.T) {
	handler, store := newTestArtistAlbumHandler(t, nil)
	identifyForTest(t, store, "/music/a.mp3", usecases.RecordingMetadata{
		Artist: "Test Artist", Album: "Test Album", Title: "Song", TrackNumber: 1,
		ArtistMBID: "artist-1", ReleaseGroupMBID: "rg-1", RecordingID: "rec-1",
	})

	app := fiber.New()
	app.Get("/api/v1/library/artists", handler.Artists)
	app.Get("/api/v1/library/albums", handler.Albums)
	app.Get("/api/v1/library/tracks", handler.Tracks)

	// Discover the artist's key via the artists listing.
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/library/artists", nil))
	if err != nil {
		t.Fatalf("app.Test(artists): %v", err)
	}
	var artistsResp ArtistsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&artistsResp); err != nil {
		t.Fatalf("decode artists: %v", err)
	}
	resp.Body.Close()
	if len(artistsResp.Artists) != 1 || artistsResp.Artists[0].ArtistKey != "artist-1" {
		t.Fatalf("artists = %+v, want exactly artist-1", artistsResp.Artists)
	}

	// Key-based album drill-down.
	resp, err = app.Test(httptest.NewRequest("GET", "/api/v1/library/albums?artist_key=artist-1", nil))
	if err != nil {
		t.Fatalf("app.Test(albums by key): %v", err)
	}
	var albumsResp AlbumsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&albumsResp); err != nil {
		t.Fatalf("decode albums: %v", err)
	}
	resp.Body.Close()
	if len(albumsResp.Albums) != 1 || albumsResp.Albums[0].AlbumKey != "rg-1" {
		t.Fatalf("albums = %+v, want exactly rg-1", albumsResp.Albums)
	}

	// Name-based album drill-down (backward compatibility).
	resp, err = app.Test(httptest.NewRequest("GET", "/api/v1/library/albums?artist=Test+Artist", nil))
	if err != nil {
		t.Fatalf("app.Test(albums by name): %v", err)
	}
	var albumsByName AlbumsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&albumsByName); err != nil {
		t.Fatalf("decode albums by name: %v", err)
	}
	resp.Body.Close()
	if len(albumsByName.Albums) != 1 || albumsByName.Albums[0].AlbumKey != "rg-1" {
		t.Fatalf("albums by name = %+v, want exactly rg-1", albumsByName.Albums)
	}

	// Key-based track drill-down.
	resp, err = app.Test(httptest.NewRequest("GET", "/api/v1/library/tracks?artist_key=artist-1&album_key=rg-1", nil))
	if err != nil {
		t.Fatalf("app.Test(tracks by key): %v", err)
	}
	var tracksResp TracksListResponse
	if err := json.NewDecoder(resp.Body).Decode(&tracksResp); err != nil {
		t.Fatalf("decode tracks: %v", err)
	}
	resp.Body.Close()
	if len(tracksResp.Entries) != 1 || tracksResp.Entries[0].Path != "/music/a.mp3" {
		t.Fatalf("tracks = %+v, want exactly a.mp3", tracksResp.Entries)
	}

	// Name-based track drill-down (backward compatibility).
	resp, err = app.Test(httptest.NewRequest("GET", "/api/v1/library/tracks?artist=Test+Artist&album=Test+Album", nil))
	if err != nil {
		t.Fatalf("app.Test(tracks by name): %v", err)
	}
	var tracksByName TracksListResponse
	if err := json.NewDecoder(resp.Body).Decode(&tracksByName); err != nil {
		t.Fatalf("decode tracks by name: %v", err)
	}
	resp.Body.Close()
	if len(tracksByName.Entries) != 1 || tracksByName.Entries[0].Path != "/music/a.mp3" {
		t.Fatalf("tracks by name = %+v, want exactly a.mp3", tracksByName.Entries)
	}
}

func TestArtistAlbumHandler_ArtistCompletenessCheck(t *testing.T) {
	lookup := &fakeDiscographyLookup{
		releaseGroups: map[string][]usecases.ArtistReleaseGroupSummary{
			"artist-1": {
				{ReleaseGroupMBID: "rg-1", Title: "Owned Album"},
				{ReleaseGroupMBID: "rg-2", Title: "Missing Album", Year: 2010},
			},
		},
	}
	handler, store := newTestArtistAlbumHandler(t, lookup)
	identifyForTest(t, store, "/music/a.mp3", usecases.RecordingMetadata{
		Artist: "Test Artist", Album: "Owned Album", Title: "Song", TrackNumber: 1,
		ArtistMBID: "artist-1", ReleaseGroupMBID: "rg-1", RecordingID: "rec-1",
	})

	app := fiber.New()
	app.Get("/api/v1/library/artists/completeness", handler.ArtistCompletenessCheck)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/library/artists/completeness?artist_key=artist-1", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got ArtistCompletenessResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.OwnedAlbums != 1 || got.TotalAlbums != 2 {
		t.Fatalf("got = %+v, want owned=1 total=2", got)
	}
	if len(got.Missing) != 1 || got.Missing[0].Title != "Missing Album" || got.Missing[0].Year != 2010 {
		t.Fatalf("Missing = %+v, want exactly Missing Album (2010)", got.Missing)
	}
}

func TestArtistAlbumHandler_CompletenessUnavailableForUnidentifiedGroup(t *testing.T) {
	handler, store := newTestArtistAlbumHandler(t, nil)
	ctx := context.Background()
	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{{
			Path: "/music/raw.mp3", Format: domain.FormatMP3, Status: domain.StatusNew,
			RawArtist: "Raw Artist",
		}},
	}); err != nil {
		t.Fatalf("BulkApply: %v", err)
	}

	app := fiber.New()
	app.Get("/api/v1/library/artists/completeness", handler.ArtistCompletenessCheck)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/library/artists/completeness?artist=Raw+Artist", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (completeness unavailable for a name-derived group)", resp.StatusCode)
	}
}
