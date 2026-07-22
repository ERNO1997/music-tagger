package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"music-tagger/internal/domain"
)

// fakeRelocateStore is a minimal TrackingStore fake exercising only what
// RelocateFile.Relocate touches: Get, RecordRelocation, and
// RecordRelocationFailure.
type fakeRelocateStore struct {
	TrackingStore
	records map[string]domain.FileRecord
}

func (s *fakeRelocateStore) Get(ctx context.Context, path string) (domain.FileRecord, bool, error) {
	rec, ok := s.records[path]
	return rec, ok, nil
}

func (s *fakeRelocateStore) RecordRelocation(ctx context.Context, oldPath, newPath string) error {
	rec := s.records[oldPath]
	delete(s.records, oldPath)
	rec.Path = newPath
	rec.Relocated = true
	s.records[newPath] = rec
	return nil
}

func (s *fakeRelocateStore) RecordRelocationFailure(ctx context.Context, path string, relocateErr string) error {
	rec := s.records[path]
	rec.RelocateError = relocateErr
	s.records[path] = rec
	return nil
}

// fakeRelocator computes a deterministic destination from the input
// metadata, and fails for any path listed in failFor.
type fakeRelocator struct {
	failFor map[string]bool
}

func (f fakeRelocator) Relocate(ctx context.Context, path string, meta RelocateInput) (string, error) {
	if f.failFor[path] {
		return "", errors.New("simulated relocation failure")
	}
	return "/music/" + meta.Artist + "/" + meta.Title + ".mp3", nil
}

func (f fakeRelocator) Undo(ctx context.Context, currentPath, originalPath string) error {
	return nil
}

func waitForRelocateIdle(t *testing.T, m *RelocateManager) {
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

func TestRelocateManager_ReportsOldToNewPathsForSuccessfulRelocationsOnly(t *testing.T) {
	store := &fakeRelocateStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Status: domain.StatusIdentified, Tagged: true, Artist: "Artist", Title: "Song"},
		"/music/b.mp3": {Path: "/music/b.mp3", Status: domain.StatusNew}, // not identified/tagged -> skipped
	}}
	relocateFile := NewRelocateFile(fakeRelocator{}, store)
	manager := NewRelocateManager(relocateFile, nil)

	if err := manager.Start([]string{"/music/a.mp3", "/music/b.mp3"}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForRelocateIdle(t, manager)

	relocations := manager.Relocations()
	if len(relocations) != 1 {
		t.Fatalf("Relocations() = %+v; want exactly 1 (the skipped file must not appear)", relocations)
	}
	if relocations[0].OldPath != "/music/a.mp3" || relocations[0].NewPath != "/music/Artist/Song.mp3" {
		t.Fatalf("relocation = %+v; want old=/music/a.mp3 new=/music/Artist/Song.mp3", relocations[0])
	}
}

func TestRelocateManager_FailedRelocationDoesNotAppearInRelocations(t *testing.T) {
	store := &fakeRelocateStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Status: domain.StatusIdentified, Tagged: true, Artist: "Artist", Title: "Song"},
	}}
	relocateFile := NewRelocateFile(fakeRelocator{failFor: map[string]bool{"/music/a.mp3": true}}, store)
	manager := NewRelocateManager(relocateFile, nil)

	if err := manager.Start([]string{"/music/a.mp3"}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForRelocateIdle(t, manager)

	if relocations := manager.Relocations(); len(relocations) != 0 {
		t.Fatalf("Relocations() = %+v; want empty since the only relocation failed", relocations)
	}
}

func TestRelocateManager_RelocationsResetOnNextStart(t *testing.T) {
	store := &fakeRelocateStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Status: domain.StatusIdentified, Tagged: true, Artist: "Artist", Title: "Song"},
		"/music/c.mp3": {Path: "/music/c.mp3", Status: domain.StatusNew},
	}}
	relocateFile := NewRelocateFile(fakeRelocator{}, store)
	manager := NewRelocateManager(relocateFile, nil)

	if err := manager.Start([]string{"/music/a.mp3"}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForRelocateIdle(t, manager)
	if len(manager.Relocations()) != 1 {
		t.Fatalf("expected 1 relocation after the first job")
	}

	if err := manager.Start([]string{"/music/c.mp3"}); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	waitForRelocateIdle(t, manager)

	if relocations := manager.Relocations(); len(relocations) != 0 {
		t.Fatalf("Relocations() = %+v; want empty — a new job SHALL reset the accumulated list rather than appending to the previous job's", relocations)
	}
}
