package usecases

import "context"

// CoverArtStore is the on-disk storage EnrichFile needs to check for and
// save downloaded cover art, implemented by internal/infrastructure/covers.Store.
type CoverArtStore interface {
	// Path returns the on-disk path for a release's cover art and whether
	// a file already exists there.
	Path(releaseMBID string) (path string, exists bool)

	// Save writes image bytes to disk for a release and returns the path.
	Save(releaseMBID string, data []byte) (string, error)
}

// EnrichFile resolves and stores cover art for one already-identified
// tracked file, and records the outcome in the tracking store.
type EnrichFile struct {
	coverArt CoverArtLookup
	storage  CoverArtStore
	store    TrackingStore
}

func NewEnrichFile(coverArt CoverArtLookup, storage CoverArtStore, store TrackingStore) *EnrichFile {
	return &EnrichFile{coverArt: coverArt, storage: storage, store: store}
}

// Enrich resolves cover art for one tracked file given its Release MBID
// and Release-Group MBID (the latter used as a fallback when the specific
// release has no art). If a cover for that release is already stored on
// disk (from enriching another track on the same release), it's reused
// without a redundant Cover Art Archive call. If no cover art is available
// anywhere in the release-group, the file's cover art path is simply left
// unset — not an error.
func (u *EnrichFile) Enrich(ctx context.Context, path, releaseMBID, releaseGroupMBID string) error {
	if existingPath, exists := u.storage.Path(releaseMBID); exists {
		return u.store.RecordCoverArt(ctx, path, existingPath)
	}

	data, err := u.coverArt.Lookup(ctx, releaseMBID, releaseGroupMBID)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	savedPath, err := u.storage.Save(releaseMBID, data)
	if err != nil {
		return err
	}

	return u.store.RecordCoverArt(ctx, path, savedPath)
}
