package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/domain"
	"music-tagger/internal/infrastructure/persistence"
	"music-tagger/internal/usecases"
)

func newSelectionTestApp(t *testing.T) *fiber.App {
	t.Helper()
	ctx := context.Background()
	store, err := persistence.NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{
			{Path: "/music/a.mp3", Format: domain.FormatMP3, Status: domain.StatusNew},
			{Path: "/music/b.mp3", Format: domain.FormatMP3, Status: domain.StatusIdentified},
			{Path: "/music/c.mp3", Format: domain.FormatMP3, Status: domain.StatusIdentified},
		},
	}); err != nil {
		t.Fatalf("BulkApply: %v", err)
	}

	app := fiber.New()
	app.Get("/api/v1/library", NewLibraryHandler(store).List)
	app.Post("/api/v1/library/selection", NewSelectionHandler(store).List)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, url string, body any) LibraryListResponse {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, url, reqBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	var out LibraryListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestSelectionHandler_Paths(t *testing.T) {
	app := newSelectionTestApp(t)

	got := doJSON(t, app, "POST", "/api/v1/library/selection", SelectionRequest{
		Paths: []string{"/music/a.mp3", "/music/c.mp3"},
	})

	if got.Total != 2 {
		t.Fatalf("total = %d, want 2", got.Total)
	}
	paths := map[string]bool{}
	for _, e := range got.Entries {
		paths[e.Path] = true
	}
	if !paths["/music/a.mp3"] || !paths["/music/c.mp3"] {
		t.Fatalf("entries = %+v, want exactly a.mp3 and c.mp3", got.Entries)
	}
}

func TestSelectionHandler_Filter(t *testing.T) {
	app := newSelectionTestApp(t)

	got := doJSON(t, app, "POST", "/api/v1/library/selection", SelectionRequest{
		Filter: &SelectionFilter{Status: string(domain.StatusIdentified)},
	})

	if got.Total != 2 {
		t.Fatalf("total = %d, want 2", got.Total)
	}
	for _, e := range got.Entries {
		if e.Status != string(domain.StatusIdentified) {
			t.Fatalf("entry %s has status %s, want identified", e.Path, e.Status)
		}
	}
}

func TestSelectionHandler_PaginationAndSortMatchLibraryList(t *testing.T) {
	app := newSelectionTestApp(t)

	selResp := doJSON(t, app, "POST", "/api/v1/library/selection?sort=path&order=desc&limit=2&offset=1", SelectionRequest{
		Filter: &SelectionFilter{},
	})
	libResp := doJSON(t, app, "GET", "/api/v1/library?sort=path&order=desc&limit=2&offset=1", nil)

	if selResp.Total != libResp.Total {
		t.Fatalf("selection total = %d, library total = %d", selResp.Total, libResp.Total)
	}
	if len(selResp.Entries) != len(libResp.Entries) {
		t.Fatalf("selection entries = %d, library entries = %d", len(selResp.Entries), len(libResp.Entries))
	}
	for i := range selResp.Entries {
		if selResp.Entries[i].Path != libResp.Entries[i].Path {
			t.Fatalf("entry %d: selection path = %s, library path = %s", i, selResp.Entries[i].Path, libResp.Entries[i].Path)
		}
	}
}
