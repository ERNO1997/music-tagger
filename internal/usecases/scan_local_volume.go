package usecases

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"music-tagger/internal/domain"
)

// ScanResult is one discovered audio file and its fingerprinting outcome.
// Error is populated (and Fingerprint/Duration left zero) when fingerprinting
// failed for this specific file; it never aborts the overall scan.
type ScanResult struct {
	Path        string
	Format      domain.Format
	Duration    time.Duration
	Fingerprint string
	Error       string
}

// ScanLocalVolume recursively discovers supported audio files under a root
// directory and computes a fingerprint for each. It performs no writes, no
// relocation, and no upstream network calls — read-only by design.
type ScanLocalVolume struct {
	fingerprinter Fingerprinter
}

func NewScanLocalVolume(fingerprinter Fingerprinter) *ScanLocalVolume {
	return &ScanLocalVolume{fingerprinter: fingerprinter}
}

// Scan walks root and returns one ScanResult per supported audio file found.
// A missing root or one containing no supported files yields an empty,
// non-nil slice rather than an error.
func (s *ScanLocalVolume) Scan(ctx context.Context, root string) ([]ScanResult, error) {
	results := []ScanResult{}

	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}

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

		result := ScanResult{Path: path, Format: format}
		fp, ferr := s.fingerprinter.Fingerprint(ctx, path)
		if ferr != nil {
			result.Error = ferr.Error()
		} else {
			result.Fingerprint = fp.Chroma
			result.Duration = fp.Duration
		}
		results = append(results, result)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return results, nil
}

func detectFormat(path string) (domain.Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return domain.FormatMP3, true
	case ".flac":
		return domain.FormatFLAC, true
	default:
		return "", false
	}
}
