package usecases

import (
	"context"
	"errors"
)

// CoverArtStore is the on-disk storage EnrichFile needs to check for and
// save downloaded cover art, implemented by internal/infrastructure/covers.Store.
type CoverArtStore interface {
	// Path returns the on-disk path for a release's cover art and whether
	// a file already exists there.
	Path(releaseMBID string) (path string, exists bool)

	// Save writes image bytes to disk for a release and returns the path.
	Save(releaseMBID string, data []byte) (string, error)
}

// EnrichmentInput is one already-identified tracked file's data needed to
// resolve its cover art and lyrics. Kept as a struct rather than growing
// Enrich's positional parameter list further as more enrichment sources
// are added.
type EnrichmentInput struct {
	Path             string
	ReleaseMBID      string
	ReleaseGroupMBID string
	Artist           string
	Title            string
	Album            string
	DurationSeconds  int

	// ExistingCoverArtPath and ExistingLyrics are the file's tracking
	// record's already-stored values, if any — from a prior enrichment or
	// from the background-library-analysis capability's automatic
	// embedded-content detection. Non-empty means Enrich SHALL leave that
	// field untouched rather than replacing it.
	ExistingCoverArtPath string
	ExistingLyrics       string
}

// EnrichFile resolves and stores cover art and lyrics for one
// already-identified tracked file, and records the outcome in the tracking
// store.
type EnrichFile struct {
	coverArt CoverArtLookup
	storage  CoverArtStore
	lyrics   LyricsLookup
	store    TrackingStore
}

func NewEnrichFile(coverArt CoverArtLookup, storage CoverArtStore, lyrics LyricsLookup, store TrackingStore) *EnrichFile {
	return &EnrichFile{coverArt: coverArt, storage: storage, lyrics: lyrics, store: store}
}

// Enrich resolves cover art and lyrics for one tracked file. The two are
// attempted independently — a failure or "not found" in one does not skip
// the other — and any errors from both are combined via errors.Join so the
// caller still sees a failure without losing information about a possible
// partial success.
func (u *EnrichFile) Enrich(ctx context.Context, input EnrichmentInput) error {
	coverErr := u.enrichCoverArt(ctx, input)
	lyricsErr := u.enrichLyrics(ctx, input)
	return errors.Join(coverErr, lyricsErr)
}

// enrichCoverArt resolves cover art given the file's Release MBID and
// Release-Group MBID (the latter used as a fallback when the specific
// release has no art). If a cover for that release is already stored on
// disk (from enriching another track on the same release), it's reused
// without a redundant Cover Art Archive call. If no cover art is available
// anywhere in the release-group, the file's cover art path is simply left
// unset — not an error. If the file's tracking record already has a cover
// art path stored (from a prior enrichment or automatic embedded-content
// detection), it's left untouched rather than replaced.
func (u *EnrichFile) enrichCoverArt(ctx context.Context, input EnrichmentInput) error {
	if input.ExistingCoverArtPath != "" {
		return nil
	}

	if existingPath, exists := u.storage.Path(input.ReleaseMBID); exists {
		return u.store.RecordCoverArt(ctx, input.Path, existingPath)
	}

	data, err := u.coverArt.Lookup(ctx, input.ReleaseMBID, input.ReleaseGroupMBID)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	savedPath, err := u.storage.Save(input.ReleaseMBID, data)
	if err != nil {
		return err
	}

	return u.store.RecordCoverArt(ctx, input.Path, savedPath)
}

// enrichLyrics resolves lyrics via LRCLIB given the file's resolved
// artist/title/album/duration. If LRCLIB has no entry, or marks the track
// instrumental, the file's lyrics fields are simply left unset — not an
// error. If the file's tracking record already has lyrics stored (from a
// prior enrichment or automatic embedded-content detection), they're left
// untouched rather than replaced.
func (u *EnrichFile) enrichLyrics(ctx context.Context, input EnrichmentInput) error {
	if input.ExistingLyrics != "" {
		return nil
	}

	plainLyrics, syncedLyrics, found, err := u.lyrics.Lookup(ctx, input.Artist, input.Title, input.Album, input.DurationSeconds)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	return u.store.RecordLyrics(ctx, input.Path, plainLyrics, syncedLyrics)
}
