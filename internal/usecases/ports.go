package usecases

import (
	"context"

	"music-tagger/internal/domain"
)

// Fingerprinter computes the acoustic identity of a single audio file.
// Implementations must never derive identity from the file's name or
// any pre-existing embedded tags.
type Fingerprinter interface {
	Fingerprint(ctx context.Context, path string) (domain.Fingerprint, error)
}

// TrackingStore persists per-file discovery/identification state across
// refreshes and restarts.
type TrackingStore interface {
	// LoadAll returns every tracked record, keyed by path, for in-memory
	// diffing against a fresh disk walk.
	LoadAll(ctx context.Context) (map[string]domain.FileRecord, error)

	// BulkApply commits the outcome of one refresh pass in a single
	// transaction: new/changed file upserts, paths to mark missing, and
	// previously-missing paths that reappeared unchanged.
	BulkApply(ctx context.Context, apply BulkApply) error
}

// BulkApply is the batched result of one refresh pass.
type BulkApply struct {
	// Upserts are new or changed files: inserted or updated with a fresh
	// fingerprint, status reset to StatusNew, Missing cleared.
	Upserts []domain.FileRecord

	// MissingPaths are previously tracked paths not found on this pass;
	// their Missing flag is set without altering any other field.
	MissingPaths []string

	// ReappearedPaths are paths previously marked missing that were found
	// again unchanged; their Missing flag is cleared, restoring their
	// prior Status without altering it.
	ReappearedPaths []string
}
