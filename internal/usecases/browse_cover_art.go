package usecases

import "context"

// maxCoverCandidateReleases bounds how many sibling releases in a
// release-group are checked for cover art, so a heavily-reissued album
// (which can have dozens of editions) can't turn one browse request into
// an unbounded number of Cover Art Archive lookups.
const maxCoverCandidateReleases = 20

// BrowseCoverArt lists a release-group's sibling editions' front covers, so
// a user can pick a better one than CoverArtLookup's single automatic
// choice, and records a chosen one exactly like a normal enrichment.
type BrowseCoverArt struct {
	releaseGroups MusicBrainzReleaseGroupLookup
	coverArt      CoverArtBrowser
	storage       CoverArtStore
	store         TrackingStore
}

func NewBrowseCoverArt(releaseGroups MusicBrainzReleaseGroupLookup, coverArt CoverArtBrowser, storage CoverArtStore, store TrackingStore) *BrowseCoverArt {
	return &BrowseCoverArt{releaseGroups: releaseGroups, coverArt: coverArt, storage: storage, store: store}
}

// Candidates lists front-cover candidates across the tracked file's
// release-group's sibling editions (each checked independently — one
// release with no upload, or a failed lookup, doesn't exclude the others).
// found is false for an unknown path or one with no resolved release-group
// (not yet identified).
func (u *BrowseCoverArt) Candidates(ctx context.Context, path string) (candidates []CoverArtCandidate, found bool, err error) {
	rec, found, err := u.store.Get(ctx, path)
	if err != nil {
		return nil, false, err
	}
	if !found || rec.ReleaseGroupMBID == "" {
		return nil, false, nil
	}

	releases, err := u.releaseGroups.Releases(ctx, rec.ReleaseGroupMBID)
	if err != nil {
		return nil, true, err
	}
	if len(releases) > maxCoverCandidateReleases {
		releases = releases[:maxCoverCandidateReleases]
	}

	for _, release := range releases {
		thumbnailURL, imageURL, ok, err := u.coverArt.FrontImage(ctx, release.ReleaseMBID)
		if err != nil || !ok {
			continue
		}
		candidates = append(candidates, CoverArtCandidate{
			ReleaseMBID:  release.ReleaseMBID,
			ReleaseTitle: release.Title,
			ThumbnailURL: thumbnailURL,
			ImageURL:     imageURL,
		})
	}
	return candidates, true, nil
}

// Choose downloads imageURL (as returned by Candidates for releaseMBID),
// saves it under releaseMBID (shared with any other track already using
// that same release's cover, per CoverArtStore's existing dedup), and
// records it as path's cover art exactly like a normal enrichment.
func (u *BrowseCoverArt) Choose(ctx context.Context, path, releaseMBID, imageURL string) error {
	if existingPath, exists := u.storage.Path(releaseMBID); exists {
		return u.store.RecordCoverArt(ctx, path, existingPath)
	}

	data, err := u.coverArt.Download(ctx, imageURL)
	if err != nil {
		return err
	}

	savedPath, err := u.storage.Save(releaseMBID, data)
	if err != nil {
		return err
	}

	return u.store.RecordCoverArt(ctx, path, savedPath)
}
