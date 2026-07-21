package usecases

import "context"

// manualSearchLimit bounds how many MusicBrainz search hits are resolved to
// full metadata per manual search — each resolution is its own rate-gated
// MusicBrainz request, so this keeps a single manual search's worst-case
// latency bounded rather than unbounded on a very broad query.
const manualSearchLimit = 8

// ManualSearch lets a user identify (or re-identify) any tracked file by
// searching MusicBrainz with their own free text, independent of any
// audio fingerprint. Results are stored as that file's candidates via the
// same mechanism AcoustID tied-recording disambiguation already uses, so
// they're resolved through the existing candidate-picker/resolve path.
type ManualSearch struct {
	search MusicBrainzSearch
	store  TrackingStore
}

func NewManualSearch(search MusicBrainzSearch, store TrackingStore) *ManualSearch {
	return &ManualSearch{search: search, store: store}
}

// Search resolves query to candidate recordings and, if any are found,
// stores them as path's candidates (discarding any prior resolved metadata
// or stored candidates, and setting its status to ambiguous — the same
// state a tied AcoustID result would produce). A zero-result search leaves
// path's tracked state untouched entirely. found is false (with a nil
// candidates slice and error) for an untracked path — checked before
// searching, since RecordAmbiguous itself doesn't validate that path is
// tracked and would otherwise leave orphaned candidate rows for one that
// isn't.
func (u *ManualSearch) Search(ctx context.Context, path, query string) (candidates []RecordingMetadata, found bool, err error) {
	_, found, err = u.store.Get(ctx, path)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	candidates, err = u.search.Search(ctx, query, manualSearchLimit)
	if err != nil {
		return nil, true, err
	}
	if len(candidates) == 0 {
		return nil, true, nil
	}

	if err := u.store.RecordAmbiguous(ctx, path, candidates); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}
