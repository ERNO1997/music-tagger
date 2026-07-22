package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"music-tagger/internal/domain"
)

// fakeAnalysisStore is a minimal TrackingStore fake exercising only what
// AnalysisManager touches: LoadAll, RecordFingerprint, RecordCoverArt,
// RecordLyrics, and RecordRelocation.
type fakeAnalysisStore struct {
	TrackingStore
	records map[string]domain.FileRecord
}

func (s *fakeAnalysisStore) LoadAll(ctx context.Context) (map[string]domain.FileRecord, error) {
	out := make(map[string]domain.FileRecord, len(s.records))
	for k, v := range s.records {
		out[k] = v
	}
	return out, nil
}

func (s *fakeAnalysisStore) RecordFingerprint(ctx context.Context, path string, fingerprint string, durationSeconds float64, fingerprintErr string) error {
	rec := s.records[path]
	rec.Fingerprint = fingerprint
	rec.DurationSeconds = durationSeconds
	rec.FingerprintError = fingerprintErr
	s.records[path] = rec
	return nil
}

func (s *fakeAnalysisStore) RecordCoverArt(ctx context.Context, path string, coverArtPath string) error {
	rec := s.records[path]
	rec.CoverArtPath = coverArtPath
	s.records[path] = rec
	return nil
}

func (s *fakeAnalysisStore) RecordLyrics(ctx context.Context, path string, lyrics string, syncedLyrics string) error {
	rec := s.records[path]
	rec.Lyrics = lyrics
	rec.SyncedLyrics = syncedLyrics
	s.records[path] = rec
	return nil
}

func (s *fakeAnalysisStore) RecordRelocation(ctx context.Context, oldPath, newPath string) error {
	rec := s.records[oldPath]
	rec.Path = newPath
	rec.Relocated = true
	s.records[newPath] = rec
	if newPath != oldPath {
		delete(s.records, oldPath)
	}
	return nil
}

// fakeFingerprinter returns a deterministic fingerprint per path, or fails
// for any path listed in failFor.
type fakeFingerprinter struct {
	failFor map[string]bool
}

func (f fakeFingerprinter) Fingerprint(ctx context.Context, path string) (domain.Fingerprint, error) {
	if f.failFor[path] {
		return domain.Fingerprint{}, errors.New("simulated fingerprinting failure")
	}
	return domain.Fingerprint{Chroma: "fp:" + path, Duration: 200 * time.Second}, nil
}

// fakeAnalysisTagger returns preconfigured embedded content per path.
type fakeAnalysisTagger struct {
	content map[string]struct {
		coverArt []byte
		lyrics   string
	}
	failFor map[string]bool
}

func (f fakeAnalysisTagger) Tag(ctx context.Context, path string, meta TagInput) error {
	return nil
}

func (f fakeAnalysisTagger) ReadEmbeddedTags(ctx context.Context, path string) (EmbeddedTags, error) {
	return EmbeddedTags{}, nil
}

func (f fakeAnalysisTagger) ReadEmbeddedContent(ctx context.Context, path string) ([]byte, string, error) {
	if f.failFor[path] {
		return nil, "", errors.New("simulated embedded-read failure")
	}
	c := f.content[path]
	return c.coverArt, c.lyrics, nil
}

// fakeAnalysisCoverStore is an in-memory CoverArtStore fake.
type fakeAnalysisCoverStore struct {
	saved map[string][]byte
}

func newFakeAnalysisCoverStore() *fakeAnalysisCoverStore {
	return &fakeAnalysisCoverStore{saved: map[string][]byte{}}
}

func (s *fakeAnalysisCoverStore) Path(key string) (string, bool) {
	_, exists := s.saved[key]
	return "/covers/" + key + ".jpg", exists
}

func (s *fakeAnalysisCoverStore) Save(key string, data []byte) (string, error) {
	s.saved[key] = data
	return "/covers/" + key + ".jpg", nil
}

// fakeDestinationComputer computes a deterministic destination from the
// input metadata, matching fakeRelocator's shape.
type fakeDestinationComputer struct{}

func (fakeDestinationComputer) ComputeDestination(path string, meta RelocateInput) string {
	return "/music/" + meta.Artist + "/" + meta.Title + ".mp3"
}

func waitForAnalysisIdle(t *testing.T, m *AnalysisManager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !m.Status().Running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("analysis pass did not finish in time")
}

func TestAnalysisManager_FingerprintsFilesLackingOne(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3"},
	}}
	m := NewAnalysisManager(fakeFingerprinter{}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	if got := store.records["/music/a.mp3"].Fingerprint; got != "fp:/music/a.mp3" {
		t.Fatalf("Fingerprint = %q; want it computed and stored", got)
	}
}

func TestAnalysisManager_DoesNotRecomputeExistingFingerprint(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Fingerprint: "already-there"},
	}}
	// A fingerprinter that always fails would surface a recomputation as a
	// stored fingerprint error — proving Start left the existing value alone.
	m := NewAnalysisManager(fakeFingerprinter{failFor: map[string]bool{"/music/a.mp3": true}}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	rec := store.records["/music/a.mp3"]
	if rec.Fingerprint != "already-there" || rec.FingerprintError != "" {
		t.Fatalf("record = %+v; want fingerprint left untouched and no error recorded", rec)
	}
}

func TestAnalysisManager_StoresEmbeddedCoverArtAndLyricsWhenAbsent(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Fingerprint: "fp"},
	}}
	tagger := fakeAnalysisTagger{content: map[string]struct {
		coverArt []byte
		lyrics   string
	}{
		"/music/a.mp3": {coverArt: []byte("cover-bytes"), lyrics: "la la la"},
	}}
	coverStore := newFakeAnalysisCoverStore()
	m := NewAnalysisManager(fakeFingerprinter{}, tagger, coverStore, store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	rec := store.records["/music/a.mp3"]
	if rec.CoverArtPath == "" {
		t.Fatal("CoverArtPath left empty; want the embedded cover art stored")
	}
	if rec.Lyrics != "la la la" {
		t.Fatalf("Lyrics = %q; want the embedded lyrics stored", rec.Lyrics)
	}
}

func TestAnalysisManager_LeavesExistingEnrichedCoverArtAndLyricsUnchanged(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/a.mp3": {Path: "/music/a.mp3", Fingerprint: "fp", CoverArtPath: "/covers/enriched.jpg", Lyrics: "enriched lyrics"},
	}}
	// Different embedded content than what's already stored — must be ignored.
	tagger := fakeAnalysisTagger{content: map[string]struct {
		coverArt []byte
		lyrics   string
	}{
		"/music/a.mp3": {coverArt: []byte("different-cover-bytes"), lyrics: "different embedded lyrics"},
	}}
	m := NewAnalysisManager(fakeFingerprinter{}, tagger, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	rec := store.records["/music/a.mp3"]
	if rec.CoverArtPath != "/covers/enriched.jpg" {
		t.Fatalf("CoverArtPath = %q; want the prior enrichment's value left untouched", rec.CoverArtPath)
	}
	if rec.Lyrics != "enriched lyrics" {
		t.Fatalf("Lyrics = %q; want the prior enrichment's value left untouched", rec.Lyrics)
	}
}

func TestAnalysisManager_MarksAlreadyRelocatedFileWithoutMoving(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/Artist/Song.mp3": {
			Path: "/music/Artist/Song.mp3", Fingerprint: "fp",
			Status: domain.StatusIdentified, Tagged: true,
			Artist: "Artist", Title: "Song",
		},
	}}
	m := NewAnalysisManager(fakeFingerprinter{}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	rec := store.records["/music/Artist/Song.mp3"]
	if !rec.Relocated {
		t.Fatal("Relocated = false; want the file marked relocated since it's already at its canonical destination")
	}
	if rec.Path != "/music/Artist/Song.mp3" {
		t.Fatalf("Path = %q; want it unchanged — the file was never moved", rec.Path)
	}
}

func TestAnalysisManager_LeavesFileNotAtDestinationUnmarked(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/somewhere-else.mp3": {
			Path: "/music/somewhere-else.mp3", Fingerprint: "fp",
			Status: domain.StatusIdentified, Tagged: true,
			Artist: "Artist", Title: "Song",
		},
	}}
	m := NewAnalysisManager(fakeFingerprinter{}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	if store.records["/music/somewhere-else.mp3"].Relocated {
		t.Fatal("Relocated = true; want it left unmarked since the file isn't at its canonical destination")
	}
}

func TestAnalysisManager_SkipsFilesNotBothIdentifiedAndTagged(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{
		"/music/Artist/Song.mp3": {
			Path: "/music/Artist/Song.mp3", Fingerprint: "fp",
			Status: domain.StatusIdentified, Tagged: false, // identified but not tagged
			Artist: "Artist", Title: "Song",
		},
	}}
	m := NewAnalysisManager(fakeFingerprinter{}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})

	if err := m.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	waitForAnalysisIdle(t, m)

	if store.records["/music/Artist/Song.mp3"].Relocated {
		t.Fatal("Relocated = true; want the relocation check skipped for a not-yet-tagged file")
	}
}

func TestAnalysisManager_BlockedByRunningRelocate(t *testing.T) {
	store := &fakeAnalysisStore{records: map[string]domain.FileRecord{}}
	m := NewAnalysisManager(fakeFingerprinter{}, fakeAnalysisTagger{}, newFakeAnalysisCoverStore(), store, fakeDestinationComputer{})
	m.SetRelocateStatus(alwaysRunning{})

	err := m.Start()
	if !errors.Is(err, ErrBlockedByRelocate) {
		t.Fatalf("Start() error = %v; want ErrBlockedByRelocate", err)
	}
}

func TestRelocateManager_BlockedByRunningAnalysis(t *testing.T) {
	store := &fakeRelocateStore{records: map[string]domain.FileRecord{}}
	relocateFile := NewRelocateFile(fakeRelocator{}, store)
	manager := NewRelocateManager(relocateFile, nil)
	manager.SetAnalysisStatus(alwaysRunning{})

	err := manager.Start(nil)
	if !errors.Is(err, ErrBlockedByAnalysis) {
		t.Fatalf("Start() error = %v; want ErrBlockedByAnalysis", err)
	}
}

// alwaysRunning is a StatusChecker fake reporting a job perpetually in
// progress, used to exercise mutual-exclusion guards.
type alwaysRunning struct{}

func (alwaysRunning) Status() JobStatus {
	return JobStatus{Running: true}
}
