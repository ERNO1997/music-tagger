package usecases

import (
	"context"
	"fmt"

	"music-tagger/internal/domain"
)

// RelocateFile physically moves one already-identified-and-tagged tracked
// file into the canonical Artist/Album/Track hierarchy, and records the
// outcome in the tracking store. Like TagFile, it loads the file's tracked
// record itself rather than requiring its caller to assemble an input
// struct.
type RelocateFile struct {
	relocator Relocator
	store     TrackingStore
}

func NewRelocateFile(relocator Relocator, store TrackingStore) *RelocateFile {
	return &RelocateFile{relocator: relocator, store: store}
}

// Relocate moves the tracked file at path. skipped is true (with a nil
// error) when the path is unknown, not yet identified, or identified but
// not yet tagged — distinct from a relocation failure, so the caller can
// log the two cases differently without treating a skip as an error.
// newPath is populated only on success, so a caller (e.g. RelocateManager)
// can report the old-to-new mapping to clients tracking a selection by path.
func (r *RelocateFile) Relocate(ctx context.Context, path string) (newPath string, skipped bool, err error) {
	rec, found, err := r.store.Get(ctx, path)
	if err != nil {
		return "", false, fmt.Errorf("loading tracked record for %s: %w", path, err)
	}
	if !found || rec.Status != domain.StatusIdentified || !rec.Tagged {
		return "", true, nil
	}

	input := RelocateInput{
		Artist:      rec.Artist,
		Album:       rec.Album,
		Title:       rec.Title,
		TrackNumber: rec.TrackNumber,
		Year:        rec.Year,
	}

	newPath, relocErr := r.relocator.Relocate(ctx, path, input)
	if relocErr != nil {
		if recErr := r.store.RecordRelocationFailure(ctx, path, relocErr.Error()); recErr != nil {
			return "", false, fmt.Errorf("relocating %s: %w (and recording the failure: %v)", path, relocErr, recErr)
		}
		return "", false, fmt.Errorf("relocating %s: %w", path, relocErr)
	}

	if recErr := r.store.RecordRelocation(ctx, path, newPath); recErr != nil {
		// The physical move succeeded but recording it failed — move the
		// file back so a failure here leaves nothing changed, same as a
		// failure in Relocate itself.
		if undoErr := r.relocator.Undo(ctx, newPath, path); undoErr != nil {
			return "", false, fmt.Errorf("relocated %s to %s but failed to record it (%v), and failed to move it back (%v) — the file is now at %s but tracked at %s",
				path, newPath, recErr, undoErr, newPath, path)
		}
		return "", false, fmt.Errorf("relocating %s: recording the outcome failed, moved the file back: %w", path, recErr)
	}

	return newPath, false, nil
}
