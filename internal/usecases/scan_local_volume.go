package usecases

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"music-tagger/internal/domain"
)

// RefreshSummary counts the outcome of one refresh pass.
type RefreshSummary struct {
	New        int
	Changed    int
	Unchanged  int
	Reappeared int
	Missing    int
	Errors     int
}

// pendingFile is a candidate discovered on disk during pass 1 that needs a
// duration read in pass 2, because it's new or has changed since last seen.
type pendingFile struct {
	path    string
	format  domain.Format
	size    int64
	modTime int64
}

// upsertChunkSize bounds how many upserts accumulate before being flushed
// in one BulkApply transaction during pass 2. This keeps commits far fewer
// than one-per-file (the original cost problem) while still giving GET
// /api/v1/library visibility into progress well before the whole refresh
// finishes, rather than only once at the very end.
const upsertChunkSize = 25

// ScanLocalVolume performs a two-pass refresh of the mounted /music volume:
// pass 1 cheaply stats every file and diffs against the tracking store to
// classify it as new/changed/unchanged/missing; pass 2 reads duration and a
// raw tag snapshot only for the new/changed set, committing in chunks (see
// upsertChunkSize) rather than one commit per file or one for the whole
// refresh. Fingerprint computation never happens during a refresh — it
// happens lazily during identification instead. It performs no writes to
// /music itself and no upstream network calls.
type ScanLocalVolume struct {
	durationReader DurationReader
	rawTagReader   RawTagReader
	store          TrackingStore
}

func NewScanLocalVolume(durationReader DurationReader, rawTagReader RawTagReader, store TrackingStore) *ScanLocalVolume {
	return &ScanLocalVolume{durationReader: durationReader, rawTagReader: rawTagReader, store: store}
}

// Refresh walks root, classifies every candidate file against the tracking
// store, reads duration and a raw tag snapshot only for the new/changed
// set (reporting progress via onProgress, which may be nil), and commits
// the outcome in one batched transaction. A missing root yields an
// empty-effect refresh, not an error.
func (s *ScanLocalVolume) Refresh(ctx context.Context, root string, onProgress func(processed, total int)) (RefreshSummary, error) {
	summary := RefreshSummary{}

	tracked, err := s.store.LoadAll(ctx)
	if err != nil {
		return summary, err
	}

	seen := make(map[string]bool, len(tracked))
	var toRead []pendingFile
	var reappeared []string

	if _, statErr := os.Stat(root); statErr == nil {
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			format, ok := detectFormat(path)
			if !ok {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			size := info.Size()
			modTime := info.ModTime().Unix()

			seen[path] = true

			existing, wasTracked := tracked[path]
			switch {
			case !wasTracked:
				toRead = append(toRead, pendingFile{path, format, size, modTime})
			case existing.Size != size || existing.ModTime != modTime:
				toRead = append(toRead, pendingFile{path, format, size, modTime})
			case existing.Missing:
				reappeared = append(reappeared, path)
				summary.Reappeared++
			default:
				summary.Unchanged++
			}
			return nil
		})
		if walkErr != nil {
			return summary, walkErr
		}
	} else if !os.IsNotExist(statErr) {
		return summary, statErr
	}

	// Missing/reappeared paths are already fully known after pass 1 (they
	// don't depend on fingerprinting), so commit them immediately rather
	// than waiting for pass 2 to finish — this alone makes removals and
	// reappearances visible right away, even before any fingerprinting
	// starts.
	var missingPaths []string
	for path, rec := range tracked {
		if seen[path] || rec.Missing {
			continue
		}
		missingPaths = append(missingPaths, path)
		summary.Missing++
	}
	if len(missingPaths) > 0 || len(reappeared) > 0 {
		if err := s.store.BulkApply(ctx, BulkApply{MissingPaths: missingPaths, ReappearedPaths: reappeared}); err != nil {
			return summary, err
		}
	}

	total := len(toRead)
	if onProgress != nil {
		onProgress(0, total)
	}

	chunk := make([]domain.FileRecord, 0, upsertChunkSize)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		if err := s.store.BulkApply(ctx, BulkApply{Upserts: chunk}); err != nil {
			return err
		}
		chunk = chunk[:0]
		return nil
	}

	for i, pf := range toRead {
		rec := domain.FileRecord{
			Path:    pf.path,
			Format:  pf.format,
			Size:    pf.size,
			ModTime: pf.modTime,
			Status:  domain.StatusNew,
		}

		duration, derr := s.durationReader.ReadDuration(ctx, pf.path)
		if derr != nil {
			rec.FingerprintError = derr.Error()
			summary.Errors++
		} else {
			rec.DurationSeconds = duration.Seconds()
		}

		// Raw tags are a separate, independent read from duration — a
		// failure here doesn't affect the duration already read above, and
		// doesn't count as a second error against the same file.
		if rawTags, rerr := s.rawTagReader.ReadRawTags(ctx, pf.path); rerr == nil {
			rec.RawTitle = rawTags.Title
			rec.RawArtist = rawTags.Artist
			rec.RawAlbum = rawTags.Album
			rec.RawAlbumArtist = rawTags.AlbumArtist
		}

		if _, ok := tracked[pf.path]; ok {
			summary.Changed++
		} else {
			summary.New++
		}

		chunk = append(chunk, rec)
		if len(chunk) >= upsertChunkSize {
			if err := flush(); err != nil {
				return summary, err
			}
		}
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	if err := flush(); err != nil {
		return summary, err
	}

	return summary, nil
}

func detectFormat(path string) (domain.Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return domain.FormatMP3, true
	case ".flac":
		return domain.FormatFLAC, true
	case ".m4a":
		return domain.FormatM4A, true
	default:
		return "", false
	}
}
