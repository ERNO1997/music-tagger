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

	// RecordIdentification updates one file's status and (when identified)
	// resolved metadata in a single commit, without altering its
	// fingerprint, size, or modification time.
	RecordIdentification(ctx context.Context, path string, result IdentificationResult) error
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

// AcoustIDMatch is one candidate MusicBrainz recording resolved from a
// fingerprint, ranked by match confidence.
type AcoustIDMatch struct {
	RecordingID string
	Score       float64
}

// AcoustIDLookup resolves a fingerprint + duration to candidate MusicBrainz
// Recording IDs. An empty, nil-error result means AcoustID found no match —
// distinct from a returned error, which means the lookup itself failed.
type AcoustIDLookup interface {
	Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDMatch, error)
}

// RecordingMetadata is the canonical metadata MusicBrainz resolves for a
// given Recording ID.
type RecordingMetadata struct {
	RecordingID string
	Artist      string
	Album       string
	Title       string
	TrackNumber int

	// Extended fields, all derived from the same recording lookup — no
	// additional MusicBrainz request. Year/DiscNumber/TotalDiscs/
	// TotalTracks are 0 when not derivable (e.g. no usable release date).
	AlbumArtist      string
	Year             int
	DiscNumber       int
	TotalDiscs       int
	TotalTracks      int
	ReleaseMBID      string
	ReleaseGroupMBID string
	ArtistMBID       string
}

// MusicBrainzLookup resolves a MusicBrainz Recording ID to canonical
// artist/release/track data. Implementations must enforce the 1 req/sec
// rate limit centrally, regardless of caller.
type MusicBrainzLookup interface {
	Lookup(ctx context.Context, recordingID string) (RecordingMetadata, error)
}

// IdentificationResult is the outcome of attempting to identify one file.
type IdentificationResult struct {
	Status   domain.TrackingStatus // StatusIdentified or StatusNotFound
	Metadata RecordingMetadata     // populated only when Status is StatusIdentified
}
