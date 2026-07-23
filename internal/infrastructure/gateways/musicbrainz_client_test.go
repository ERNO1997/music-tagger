package gateways

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestMusicBrainzClient points musicBrainzBaseURL at server for the
// duration of the calling test, restoring it on cleanup, and returns a
// client wired to the server's own HTTP client.
func newTestMusicBrainzClient(t *testing.T, server *httptest.Server) *MusicBrainzClient {
	t.Helper()
	original := musicBrainzBaseURL
	musicBrainzBaseURL = server.URL
	t.Cleanup(func() { musicBrainzBaseURL = original })
	return &MusicBrainzClient{UserAgent: "test-agent/1.0", HTTPClient: server.Client()}
}

func TestArtistReleaseGroups_FiltersToAlbumAndEP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artist/artist-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{
			"release-group-count": 4,
			"release-groups": [
				{"id": "rg-album", "title": "A Real Album", "primary-type": "Album", "first-release-date": "1999-05-01"},
				{"id": "rg-ep", "title": "A Short EP", "primary-type": "EP", "first-release-date": "2001"},
				{"id": "rg-live", "title": "Live In Concert", "primary-type": "Album", "secondary-types": ["Live"], "first-release-date": "2005"},
				{"id": "rg-single", "title": "A Single", "primary-type": "Single", "first-release-date": "2002"}
			]
		}`)
	}))
	defer server.Close()

	client := newTestMusicBrainzClient(t, server)
	got, err := client.ArtistReleaseGroups(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("ArtistReleaseGroups: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (got %+v)", len(got), got)
	}
	byID := map[string]int{}
	for _, rg := range got {
		byID[rg.ReleaseGroupMBID] = rg.Year
	}
	if year, ok := byID["rg-album"]; !ok || year != 1999 {
		t.Errorf("rg-album missing or wrong year: %+v", got)
	}
	if year, ok := byID["rg-ep"]; !ok || year != 2001 {
		t.Errorf("rg-ep missing or wrong year: %+v", got)
	}
	if _, ok := byID["rg-live"]; ok {
		t.Errorf("rg-live (Live secondary type) should have been excluded: %+v", got)
	}
	if _, ok := byID["rg-single"]; ok {
		t.Errorf("rg-single (Single primary type) should have been excluded: %+v", got)
	}
}

func TestArtistReleaseGroups_FollowsPagination(t *testing.T) {
	var requestedOffsets []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		requestedOffsets = append(requestedOffsets, mustAtoi(t, offset))
		switch offset {
		case "0":
			fmt.Fprint(w, `{"release-group-count": 3, "release-groups": [
				{"id": "rg-1", "title": "One", "primary-type": "Album"},
				{"id": "rg-2", "title": "Two", "primary-type": "Album"}
			]}`)
		case "2":
			fmt.Fprint(w, `{"release-group-count": 3, "release-groups": [
				{"id": "rg-3", "title": "Three", "primary-type": "Album"}
			]}`)
		default:
			t.Fatalf("unexpected offset requested: %s", offset)
		}
	}))
	defer server.Close()

	client := newTestMusicBrainzClient(t, server)
	got, err := client.ArtistReleaseGroups(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("ArtistReleaseGroups: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if len(requestedOffsets) != 2 {
		t.Fatalf("requestedOffsets = %v, want 2 pages fetched", requestedOffsets)
	}
}

func TestArtistReleaseGroups_RequestFailureIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestMusicBrainzClient(t, server)
	if _, err := client.ArtistReleaseGroups(context.Background(), "artist-1"); err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestReleaseTracklist_ReturnsAllTracks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release/release-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{
			"media": [
				{"position": 1, "tracks": [
					{"position": 1, "title": "Track One", "recording": {"id": "rec-1"}},
					{"position": 2, "title": "Track Two", "recording": {"id": "rec-2"}}
				]}
			]
		}`)
	}))
	defer server.Close()

	client := newTestMusicBrainzClient(t, server)
	got, err := client.ReleaseTracklist(context.Background(), "release-1")
	if err != nil {
		t.Fatalf("ReleaseTracklist: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].RecordingMBID != "rec-1" || got[0].Title != "Track One" || got[0].TrackNumber != 1 {
		t.Errorf("got[0] = %+v, unexpected", got[0])
	}
	if got[1].RecordingMBID != "rec-2" || got[1].Title != "Track Two" || got[1].TrackNumber != 2 {
		t.Errorf("got[1] = %+v, unexpected", got[1])
	}
}

func TestReleaseTracklist_RequestFailureIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestMusicBrainzClient(t, server)
	if _, err := client.ReleaseTracklist(context.Background(), "release-1"); err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("not a number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
