package usecases

import (
	"context"
	"testing"

	"music-tagger/internal/domain"
)

// fakeTrackingStore is a minimal TrackingStore fake exercising only what
// IdentifyFile.Identify/ResolveAmbiguous/ManualSearch.Search touch: Get
// (self-loading the record), RecordIdentification, RecordAmbiguous, and
// ResolveAmbiguous. Every other method panics if called, since this test
// never exercises them.
type fakeTrackingStore struct {
	TrackingStore
	record     domain.FileRecord
	candidates []RecordingMetadata // seeded stored candidates, for ResolveAmbiguous tests
	notFound   bool                // set true to make Get report an untracked path

	recordedResult     *IdentificationResult
	recordedCandidates []RecordingMetadata
	resolvedTo         *RecordingMetadata
}

func (s *fakeTrackingStore) Get(ctx context.Context, path string) (domain.FileRecord, bool, error) {
	return s.record, !s.notFound, nil
}

func (s *fakeTrackingStore) RecordIdentification(ctx context.Context, path string, result IdentificationResult) error {
	s.recordedResult = &result
	return nil
}

func (s *fakeTrackingStore) RecordAmbiguous(ctx context.Context, path string, candidates []RecordingMetadata) error {
	s.recordedCandidates = candidates
	return nil
}

func (s *fakeTrackingStore) ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (bool, error) {
	for _, c := range s.candidates {
		if c.RecordingID == recordingMBID {
			s.resolvedTo = &c
			return true, nil
		}
	}
	return false, nil
}

// fakeAcoustIDLookup returns a single top result at the given score, tied to
// however many recording IDs are given (1 for the unambiguous case, ≥2 to
// exercise the tied-recording path). extraResults, if set, are appended
// after the top result (in descending-score order, as AcoustID guarantees)
// to exercise candidates drawn from beyond the single best-scoring result.
type fakeAcoustIDLookup struct {
	score        float64
	recordingIDs []string
	extraResults []AcoustIDResult
}

func (f fakeAcoustIDLookup) Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDResult, error) {
	ids := f.recordingIDs
	if ids == nil {
		ids = []string{"some-recording-id"}
	}
	results := []AcoustIDResult{{Score: f.score, RecordingIDs: ids}}
	return append(results, f.extraResults...), nil
}

// fakeMusicBrainzLookup resolves each recording ID via byRecording (falling
// back to a default identity keyed by the ID itself), fails with
// domain.ErrNoMusicBrainzRelease for any ID listed in noRelease, and counts
// calls.
type fakeMusicBrainzLookup struct {
	calls       *int
	byRecording map[string]RecordingMetadata
	noRelease   map[string]bool
}

func (f fakeMusicBrainzLookup) Lookup(ctx context.Context, recordingID string) (RecordingMetadata, error) {
	*f.calls++
	if f.noRelease[recordingID] {
		return RecordingMetadata{}, domain.ErrNoMusicBrainzRelease
	}
	if meta, ok := f.byRecording[recordingID]; ok {
		return meta, nil
	}
	return RecordingMetadata{RecordingID: recordingID, Artist: "Some Artist", Title: "Some Title"}, nil
}

func TestIdentify_BelowConfidenceThreshold_RecordsNotFoundWithoutCallingMusicBrainz(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0

	identify := NewIdentifyFile(fakeAcoustIDLookup{score: minAcoustIDConfidence - 0.01}, fakeMusicBrainzLookup{calls: &mbCalls}, nil, store)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true for a below-threshold match; want skipped=false (it's a recorded not_found, not a skip)")
	}
	if mbCalls != 0 {
		t.Fatalf("MusicBrainz was called %d times; want 0 for a below-threshold match", mbCalls)
	}
	if store.recordedResult == nil || store.recordedResult.Status != domain.StatusNotFound {
		t.Fatalf("recorded result = %+v; want Status=StatusNotFound", store.recordedResult)
	}
}

func TestIdentify_AtOrAboveConfidenceThreshold_ResolvesNormally(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0

	identify := NewIdentifyFile(fakeAcoustIDLookup{score: minAcoustIDConfidence}, fakeMusicBrainzLookup{calls: &mbCalls}, nil, store)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true for an at-threshold match")
	}
	if mbCalls != 1 {
		t.Fatalf("MusicBrainz was called %d times; want 1 for an at-or-above-threshold match", mbCalls)
	}
	if store.recordedResult == nil || store.recordedResult.Status != domain.StatusIdentified {
		t.Fatalf("recorded result = %+v; want Status=StatusIdentified", store.recordedResult)
	}
}

func TestIdentify_TiedRecordingsWithDistinctIdentities_RecordsAmbiguous(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0
	mb := fakeMusicBrainzLookup{calls: &mbCalls, byRecording: map[string]RecordingMetadata{
		"recording-official":    {RecordingID: "recording-official", Artist: "Daft Punk", Title: "Get Lucky"},
		"recording-compilation": {RecordingID: "recording-compilation", Artist: "Walt Ribeiro", Title: "Daft Punk 'Get Lucky'"},
	}}

	identify := NewIdentifyFile(
		fakeAcoustIDLookup{score: minAcoustIDConfidence, recordingIDs: []string{"recording-official", "recording-compilation"}},
		mb, nil, store,
	)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true for a tied-recording match")
	}
	if store.recordedResult != nil {
		t.Fatalf("RecordIdentification was called (result=%+v); want only RecordAmbiguous for distinct tied identities", store.recordedResult)
	}
	if len(store.recordedCandidates) != 2 {
		t.Fatalf("recorded %d candidates; want 2 distinct candidates", len(store.recordedCandidates))
	}
}

func TestIdentify_TiedRecordingsWithSameIdentity_RecordsIdentifiedNormally(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0
	mb := fakeMusicBrainzLookup{calls: &mbCalls, byRecording: map[string]RecordingMetadata{
		"recording-a": {RecordingID: "recording-a", Artist: "Daft Punk", Title: "Get Lucky"},
		"recording-b": {RecordingID: "recording-b", Artist: "Daft Punk", Title: "Get Lucky"},
	}}

	identify := NewIdentifyFile(
		fakeAcoustIDLookup{score: minAcoustIDConfidence, recordingIDs: []string{"recording-a", "recording-b"}},
		mb, nil, store,
	)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true for tied recordings collapsing to one identity")
	}
	if store.recordedCandidates != nil {
		t.Fatalf("RecordAmbiguous was called (candidates=%+v); want a normal identified outcome since recordings collapse to one identity", store.recordedCandidates)
	}
	if store.recordedResult == nil || store.recordedResult.Status != domain.StatusIdentified {
		t.Fatalf("recorded result = %+v; want Status=StatusIdentified", store.recordedResult)
	}
	if store.recordedResult.Metadata.Artist != "Daft Punk" || store.recordedResult.Metadata.Title != "Get Lucky" {
		t.Fatalf("recorded metadata = %+v; want the shared Daft Punk/Get Lucky identity", store.recordedResult.Metadata)
	}
}

func TestIdentify_SecondQualifyingResult_ContributesCandidates(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0
	mb := fakeMusicBrainzLookup{calls: &mbCalls, byRecording: map[string]RecordingMetadata{
		"recording-top":    {RecordingID: "recording-top", Artist: "Artist One", Title: "Song One"},
		"recording-second": {RecordingID: "recording-second", Artist: "Artist Two", Title: "Song Two"},
	}}

	identify := NewIdentifyFile(
		fakeAcoustIDLookup{
			score:        0.95,
			recordingIDs: []string{"recording-top"},
			extraResults: []AcoustIDResult{{Score: minAcoustIDConfidence, RecordingIDs: []string{"recording-second"}}},
		},
		mb, nil, store,
	)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true")
	}
	if len(store.recordedCandidates) != 2 {
		t.Fatalf("recorded %d candidates; want 2, one from each qualifying result", len(store.recordedCandidates))
	}
}

func TestIdentify_BelowThresholdSecondResult_DoesNotContributeCandidates(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0
	mb := fakeMusicBrainzLookup{calls: &mbCalls}

	identify := NewIdentifyFile(
		fakeAcoustIDLookup{
			score:        0.95,
			recordingIDs: []string{"recording-top"},
			extraResults: []AcoustIDResult{{Score: minAcoustIDConfidence - 0.01, RecordingIDs: []string{"recording-below-threshold"}}},
		},
		mb, nil, store,
	)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true")
	}
	if mbCalls != 1 {
		t.Fatalf("MusicBrainz was called %d times; want 1 (the below-threshold second result must not be looked up at all)", mbCalls)
	}
	if store.recordedResult == nil || store.recordedResult.Status != domain.StatusIdentified {
		t.Fatalf("recorded result = %+v; want a normal Status=StatusIdentified from the single qualifying recording", store.recordedResult)
	}
}

func TestIdentify_TiedRecordingWithNoRelease_IsSkippedNotAborted(t *testing.T) {
	store := &fakeTrackingStore{record: domain.FileRecord{Path: "/music/a.mp3", Fingerprint: "already-computed"}}
	mbCalls := 0
	mb := fakeMusicBrainzLookup{
		calls: &mbCalls,
		byRecording: map[string]RecordingMetadata{
			"recording-a": {RecordingID: "recording-a", Artist: "Pigs Parlament", Title: "Legends Never Die"},
			"recording-b": {RecordingID: "recording-b", Artist: "Against the Current", Title: "Legends Never Die"},
		},
		noRelease: map[string]bool{"recording-c": true}, // e.g. a bare instrumental entry with no release attached
	}

	identify := NewIdentifyFile(
		fakeAcoustIDLookup{score: minAcoustIDConfidence, recordingIDs: []string{"recording-a", "recording-b", "recording-c"}},
		mb, nil, store,
	)

	skipped, err := identify.Identify(context.Background(), "/music/a.mp3")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if skipped {
		t.Fatalf("Identify reported skipped=true; want the two resolvable candidates to still be recorded ambiguous")
	}
	if len(store.recordedCandidates) != 2 {
		t.Fatalf("recorded %d candidates; want 2 (the unresolvable third recording skipped, not aborting the attempt)", len(store.recordedCandidates))
	}
}

func TestResolveAmbiguous_ValidCandidate_RecordsIdentified(t *testing.T) {
	store := &fakeTrackingStore{
		record: domain.FileRecord{Path: "/music/a.mp3", Status: domain.StatusAmbiguous},
		candidates: []RecordingMetadata{
			{RecordingID: "recording-official", Artist: "Daft Punk", Title: "Get Lucky"},
			{RecordingID: "recording-compilation", Artist: "Walt Ribeiro", Title: "Daft Punk 'Get Lucky'"},
		},
	}
	identify := NewIdentifyFile(nil, nil, nil, store)

	found, err := identify.ResolveAmbiguous(context.Background(), "/music/a.mp3", "recording-official")
	if err != nil {
		t.Fatalf("ResolveAmbiguous returned error: %v", err)
	}
	if !found {
		t.Fatalf("ResolveAmbiguous reported found=false for a valid candidate")
	}
	if store.resolvedTo == nil || store.resolvedTo.RecordingID != "recording-official" {
		t.Fatalf("resolved to %+v; want recording-official", store.resolvedTo)
	}
}

func TestResolveAmbiguous_UnrecognizedCandidate_ReturnsNotFound(t *testing.T) {
	store := &fakeTrackingStore{
		record: domain.FileRecord{Path: "/music/a.mp3", Status: domain.StatusAmbiguous},
		candidates: []RecordingMetadata{
			{RecordingID: "recording-official", Artist: "Daft Punk", Title: "Get Lucky"},
		},
	}
	identify := NewIdentifyFile(nil, nil, nil, store)

	found, err := identify.ResolveAmbiguous(context.Background(), "/music/a.mp3", "recording-unknown")
	if err != nil {
		t.Fatalf("ResolveAmbiguous returned error: %v", err)
	}
	if found {
		t.Fatalf("ResolveAmbiguous reported found=true for an unrecognized candidate")
	}
	if store.resolvedTo != nil {
		t.Fatalf("resolved to %+v; want nothing resolved for an unrecognized candidate", store.resolvedTo)
	}
}
