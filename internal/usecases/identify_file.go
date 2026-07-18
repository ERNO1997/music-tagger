package usecases

import (
	"context"

	"music-tagger/internal/domain"
)

// IdentifyFile resolves one tracked file's canonical metadata via AcoustID
// then MusicBrainz, and records the outcome in the tracking store. It
// never touches the file's fingerprint, size, or modification time.
type IdentifyFile struct {
	acoustID    AcoustIDLookup
	musicBrainz MusicBrainzLookup
	store       TrackingStore
}

func NewIdentifyFile(acoustID AcoustIDLookup, musicBrainz MusicBrainzLookup, store TrackingStore) *IdentifyFile {
	return &IdentifyFile{acoustID: acoustID, musicBrainz: musicBrainz, store: store}
}

// Identify resolves and records metadata for one tracked file given its
// already-known fingerprint and duration. On a gateway/network error, the
// file's tracked state is left unchanged and the error is returned to the
// caller rather than being recorded as a `not_found` outcome.
func (u *IdentifyFile) Identify(ctx context.Context, path, fingerprint string, durationSeconds float64) error {
	matches, err := u.acoustID.Lookup(ctx, fingerprint, durationSeconds)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return u.store.RecordIdentification(ctx, path, IdentificationResult{Status: domain.StatusNotFound})
	}

	metadata, err := u.musicBrainz.Lookup(ctx, matches[0].RecordingID)
	if err != nil {
		return err
	}

	return u.store.RecordIdentification(ctx, path, IdentificationResult{
		Status:   domain.StatusIdentified,
		Metadata: metadata,
	})
}
