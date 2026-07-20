package usecases

import (
	"context"
	"fmt"

	"music-tagger/internal/domain"
)

// DeleteOutcome distinguishes why a delete did or didn't happen, so the
// caller (an HTTP handler) can map each case to a distinct response without
// guessing from an error string.
type DeleteOutcome int

const (
	DeleteOutcomeDeleted DeleteOutcome = iota
	DeleteOutcomeNotFound
	DeleteOutcomeNotMissing
)

// DeleteMissingFile removes a tracked record, but only when the file is
// confirmed missing from disk — never for a file that might still exist,
// to avoid orphaning a real file's tracking state. Never touches cover art
// files on disk: deletion is a pure tracking-store row removal, and cover
// art can be shared across multiple tracks on the same release.
type DeleteMissingFile struct {
	store TrackingStore
}

func NewDeleteMissingFile(store TrackingStore) *DeleteMissingFile {
	return &DeleteMissingFile{store: store}
}

func (d *DeleteMissingFile) Delete(ctx context.Context, path string) (DeleteOutcome, error) {
	rec, found, err := d.store.Get(ctx, path)
	if err != nil {
		return DeleteOutcomeNotFound, fmt.Errorf("loading tracked record for %s: %w", path, err)
	}
	if !found {
		return DeleteOutcomeNotFound, nil
	}
	if rec.EffectiveStatus() != domain.StatusMissing {
		return DeleteOutcomeNotMissing, nil
	}

	if err := d.store.Delete(ctx, path); err != nil {
		return DeleteOutcomeNotFound, fmt.Errorf("deleting tracked record for %s: %w", path, err)
	}
	return DeleteOutcomeDeleted, nil
}
