package v1

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/domain"
	"music-tagger/internal/infrastructure/persistence"
	"music-tagger/internal/usecases"
)

// fakeRelocator computes a deterministic destination and never fails,
// except for a designated skip path handled by the store setup below
// (status/tagged conditions), which RelocateFile itself skips before ever
// calling this.
type fakeRelocator struct{}

func (fakeRelocator) Relocate(ctx context.Context, path string, meta usecases.RelocateInput) (string, error) {
	return "/music/" + meta.Artist + "/" + meta.Title + ".mp3", nil
}

func (fakeRelocator) Undo(ctx context.Context, currentPath, originalPath string) error {
	return nil
}

func waitForRelocateIdle(t *testing.T, m *usecases.RelocateManager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !m.Status().Running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("relocate job did not finish in time")
}

func TestRelocateHandler_StatusReportsOldToNewPaths(t *testing.T) {
	ctx := context.Background()
	store, err := persistence.NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	if err := store.BulkApply(ctx, usecases.BulkApply{
		Upserts: []domain.FileRecord{
			{Path: "/music/a.mp3", Format: domain.FormatMP3, Status: domain.StatusNew},
			{Path: "/music/b.mp3", Format: domain.FormatMP3, Status: domain.StatusNew}, // stays new/untagged -> skipped
		},
	}); err != nil {
		t.Fatalf("BulkApply: %v", err)
	}
	if err := store.RecordIdentification(ctx, "/music/a.mp3", usecases.IdentificationResult{
		Status:   domain.StatusIdentified,
		Metadata: usecases.RecordingMetadata{Artist: "Artist", Title: "Song"},
	}); err != nil {
		t.Fatalf("RecordIdentification: %v", err)
	}
	if err := store.RecordTagged(ctx, "/music/a.mp3", true, ""); err != nil {
		t.Fatalf("RecordTagged: %v", err)
	}

	relocateFile := usecases.NewRelocateFile(fakeRelocator{}, store)
	manager := usecases.NewRelocateManager(relocateFile, nil)
	handler := NewRelocateHandler(manager, store)

	if err := manager.Start([]string{"/music/a.mp3", "/music/b.mp3"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRelocateIdle(t, manager)

	app := fiber.New()
	app.Get("/api/v1/library/relocate/status", handler.Status)

	req := httptest.NewRequest("GET", "/api/v1/library/relocate/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	var got RelocateStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got.Relocations) != 1 {
		t.Fatalf("Relocations = %+v; want exactly 1 (the skipped file must not appear)", got.Relocations)
	}
	if got.Relocations[0].OldPath != "/music/a.mp3" || got.Relocations[0].NewPath != "/music/Artist/Song.mp3" {
		t.Fatalf("relocation = %+v; want old=/music/a.mp3 new=/music/Artist/Song.mp3", got.Relocations[0])
	}
}
