package usecases

import (
	"context"
	"fmt"
	"os"

	"music-tagger/internal/domain"
)

// TagFile writes one already-identified tracked file's resolved metadata,
// cover art, and lyrics into the physical file's own tags, and records the
// outcome in the tracking store. Unlike EnrichFile, TagFile loads the
// file's tracked record itself (a single indexed lookup) rather than
// requiring its caller to assemble an input struct, since tagging needs
// nothing beyond what's already resolved and stored.
type TagFile struct {
	tagger Tagger
	store  TrackingStore
}

func NewTagFile(tagger Tagger, store TrackingStore) *TagFile {
	return &TagFile{tagger: tagger, store: store}
}

// Tag writes tags for the tracked file at path. skipped is true (with a
// nil error) when the path is unknown or not yet identified — distinct
// from a write failure, so the caller can log the two cases differently
// without treating a skip as an error.
func (t *TagFile) Tag(ctx context.Context, path string) (skipped bool, err error) {
	rec, found, err := t.store.Get(ctx, path)
	if err != nil {
		return false, fmt.Errorf("loading tracked record for %s: %w", path, err)
	}
	if !found || rec.Status != domain.StatusIdentified {
		return true, nil
	}

	var coverArt []byte
	if rec.CoverArtPath != "" {
		coverArt, err = os.ReadFile(rec.CoverArtPath)
		if err != nil {
			return false, fmt.Errorf("reading stored cover art for %s: %w", path, err)
		}
	}

	input := TagInput{
		Artist:           rec.Artist,
		Album:            rec.Album,
		Title:            rec.Title,
		AlbumArtist:      rec.AlbumArtist,
		TrackNumber:      rec.TrackNumber,
		TotalTracks:      rec.TotalTracks,
		DiscNumber:       rec.DiscNumber,
		TotalDiscs:       rec.TotalDiscs,
		Year:             rec.Year,
		RecordingMBID:    rec.RecordingMBID,
		ReleaseMBID:      rec.ReleaseMBID,
		ReleaseGroupMBID: rec.ReleaseGroupMBID,
		ArtistMBID:       rec.ArtistMBID,
		CoverArt:         coverArt,
		Lyrics:           rec.Lyrics,
	}

	if tagErr := t.tagger.Tag(ctx, path, input); tagErr != nil {
		if recErr := t.store.RecordTagged(ctx, path, false, tagErr.Error()); recErr != nil {
			return false, fmt.Errorf("tagging %s: %w (and recording the failure: %v)", path, tagErr, recErr)
		}
		return false, fmt.Errorf("tagging %s: %w", path, tagErr)
	}

	// Writing tags changes the file's own size and modification time on
	// disk. Refresh the stored baseline to match, so the next scan sees
	// this file as unchanged rather than concluding it was modified
	// (which would reset its status and resolved metadata to blank).
	info, statErr := os.Stat(path)
	if statErr != nil {
		return false, fmt.Errorf("tagging %s: stat after successful write: %w", path, statErr)
	}
	if err := t.store.RecordFileStat(ctx, path, info.Size(), info.ModTime().Unix()); err != nil {
		return false, fmt.Errorf("tagging %s: recording updated file stat: %w", path, err)
	}

	return false, t.store.RecordTagged(ctx, path, true, "")
}

// GetEmbeddedTags reads path's actual embedded tags live from disk,
// independent of the tracking store's resolved metadata. found is false
// when the path is unknown or currently missing from disk.
func (t *TagFile) GetEmbeddedTags(ctx context.Context, path string) (tags EmbeddedTags, found bool, err error) {
	rec, found, err := t.store.Get(ctx, path)
	if err != nil {
		return EmbeddedTags{}, false, fmt.Errorf("loading tracked record for %s: %w", path, err)
	}
	if !found || rec.Missing {
		return EmbeddedTags{}, false, nil
	}

	tags, err = t.tagger.ReadEmbeddedTags(ctx, path)
	if err != nil {
		return EmbeddedTags{}, false, fmt.Errorf("reading embedded tags for %s: %w", path, err)
	}
	return tags, true, nil
}
