package usecases

import (
	"context"
	"errors"

	"music-tagger/internal/domain"
)

// minAcoustIDConfidence is the minimum AcoustID match score accepted before
// proceeding to MusicBrainz. A best match scoring below this is treated the
// same as no match at all — preferring no metadata over metadata resolved
// from an unreliable match. Retune here if real-world results warrant it.
const minAcoustIDConfidence = 0.7

// IdentifyFile resolves one tracked file's canonical metadata via AcoustID
// then MusicBrainz, and records the outcome in the tracking store. It
// fingerprints a file lazily, the first time that file is identified with
// no fingerprint already stored, and never touches a file's size or
// modification time.
type IdentifyFile struct {
	acoustID      AcoustIDLookup
	musicBrainz   MusicBrainzLookup
	fingerprinter Fingerprinter
	store         TrackingStore
}

func NewIdentifyFile(acoustID AcoustIDLookup, musicBrainz MusicBrainzLookup, fingerprinter Fingerprinter, store TrackingStore) *IdentifyFile {
	return &IdentifyFile{acoustID: acoustID, musicBrainz: musicBrainz, fingerprinter: fingerprinter, store: store}
}

// Identify resolves and records metadata for the tracked file at path,
// self-loading its tracked record. skipped is true (with a nil error) for
// an unknown path, or when fingerprint computation fails for this file —
// distinct from a gateway/network error, which is returned to the caller
// with the file's tracked state left unchanged rather than recorded as a
// `not_found` outcome.
func (u *IdentifyFile) Identify(ctx context.Context, path string) (skipped bool, err error) {
	rec, found, err := u.store.Get(ctx, path)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}

	fingerprint := rec.Fingerprint
	durationSeconds := rec.DurationSeconds

	if fingerprint == "" {
		fp, ferr := u.fingerprinter.Fingerprint(ctx, path)
		if ferr != nil {
			// Duration is left at its already-stored value (from the
			// cheap TagLib read during scan) rather than cleared —
			// fingerprinting's own failure doesn't mean that value is
			// wrong.
			if recErr := u.store.RecordFingerprint(ctx, path, "", rec.DurationSeconds, ferr.Error()); recErr != nil {
				return false, recErr
			}
			return true, nil
		}
		fingerprint = fp.Chroma
		durationSeconds = fp.Duration.Seconds()
		if err := u.store.RecordFingerprint(ctx, path, fingerprint, durationSeconds, ""); err != nil {
			return false, err
		}
	}

	results, err := u.acoustID.Lookup(ctx, fingerprint, durationSeconds)
	if err != nil {
		return false, err
	}

	if len(results) == 0 || results[0].Score < minAcoustIDConfidence {
		return false, u.store.RecordIdentification(ctx, path, IdentificationResult{Status: domain.StatusNotFound})
	}

	// Every result at or above the confidence threshold contributes its
	// tied recordings as a candidate — not just the single best-scoring
	// result — since a genuinely valid match for this audio can sometimes
	// land in AcoustID's second- or third-best result rather than being
	// tied into the top one. Results are ordered by descending score (an
	// AcoustID API guarantee already relied on for the confidence check
	// above), so iteration stops at the first result below the threshold.
	// Recordings are resolved and deduped by (artist, title) before
	// deciding whether this is a real ambiguity or just a harmless
	// MusicBrainz cataloguing duplicate. A recording with no resolvable
	// MusicBrainz release (e.g. a bare instrumental entry with no release
	// attached) isn't a viable candidate at all — it's skipped rather than
	// aborting the whole attempt. Any other (gateway/network) error still
	// aborts, leaving the file's tracked state unchanged, per the existing
	// gateway-error convention.
	var candidates []RecordingMetadata
	seen := make(map[[2]string]bool)
	for _, result := range results {
		if result.Score < minAcoustIDConfidence {
			break
		}
		for _, recordingID := range result.RecordingIDs {
			metadata, err := u.musicBrainz.Lookup(ctx, recordingID)
			if err != nil {
				if errors.Is(err, domain.ErrNoMusicBrainzRelease) {
					continue
				}
				return false, err
			}
			key := [2]string{metadata.Artist, metadata.Title}
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, metadata)
		}
	}

	if len(candidates) == 0 {
		return false, u.store.RecordIdentification(ctx, path, IdentificationResult{Status: domain.StatusNotFound})
	}

	if len(candidates) == 1 {
		return false, u.store.RecordIdentification(ctx, path, IdentificationResult{
			Status:   domain.StatusIdentified,
			Metadata: candidates[0],
		})
	}

	return false, u.store.RecordAmbiguous(ctx, path, candidates)
}

// ResolveAmbiguous records candidate recordingMBID as path's resolved
// identification, discarding its other stored candidates. found is false
// (with a nil error) when recordingMBID doesn't match any of path's stored
// candidates.
func (u *IdentifyFile) ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error) {
	return u.store.ResolveAmbiguous(ctx, path, recordingMBID)
}
