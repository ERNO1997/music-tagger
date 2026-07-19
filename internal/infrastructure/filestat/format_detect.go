package filestat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"music-tagger/internal/domain"
)

// sniffFormat determines a file's real container format from its leading
// bytes, independent of its file extension. A file's extension is
// untrusted input here for the same reason the project already treats
// filenames and embedded tags as untrusted for identification purposes
// (see the Acoustic-First Identification Rule): a mislabeled extension
// (e.g. an M4A saved with a ".mp3" name by some downloader) would
// otherwise cause TagLib — which dispatches purely by extension — to
// write tags in the wrong format, silently having no effect for anything
// that reads the file by its real content type.
func sniffFormat(path string) (domain.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s to sniff format: %w", path, err)
	}
	defer f.Close()

	header := make([]byte, 12)
	n, err := io.ReadFull(f, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("reading header of %s: %w", path, err)
	}
	header = header[:n]

	switch {
	case len(header) >= 8 && string(header[4:8]) == "ftyp":
		return domain.FormatM4A, nil
	case len(header) >= 4 && string(header[:4]) == "fLaC":
		return domain.FormatFLAC, nil
	case len(header) >= 3 && string(header[:3]) == "ID3":
		return domain.FormatMP3, nil
	// Bare MPEG frame sync (11 set bits), for an MP3 with no ID3 header.
	case len(header) >= 2 && header[0] == 0xFF && header[1]&0xE0 == 0xE0:
		return domain.FormatMP3, nil
	default:
		return "", fmt.Errorf("unrecognized audio format signature for %s", path)
	}
}

// extensionFormat returns the format a path's extension implies, or "" if
// unrecognized.
func extensionFormat(path string) domain.Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return domain.FormatMP3
	case ".flac":
		return domain.FormatFLAC
	case ".m4a":
		return domain.FormatM4A
	default:
		return ""
	}
}

// formatExt returns the canonical extension (with leading dot) for a
// format.
func formatExt(f domain.Format) string {
	switch f {
	case domain.FormatMP3:
		return ".mp3"
	case domain.FormatFLAC:
		return ".flac"
	case domain.FormatM4A:
		return ".m4a"
	default:
		return ""
	}
}

// withCorrectExtension ensures fn operates against a path whose extension
// matches path's real, content-sniffed format — never its original
// extension when the two disagree — since go-taglib (like TagLib itself)
// dispatches purely by extension. When they agree, or the real format
// can't be determined, fn runs directly against path unchanged.
//
// When they disagree, path is renamed to a sibling with the correct
// extension for the duration of fn, then renamed back to its original
// name afterward regardless of fn's outcome — the file's name on disk
// never changes as a result of tagging; only which bytes get written
// where is corrected.
func withCorrectExtension(path string, fn func(workingPath string) error) error {
	realFormat, err := sniffFormat(path)
	if err != nil || realFormat == extensionFormat(path) {
		// Unrecognized signature: fall back to trusting the extension
		// rather than failing outright.
		return fn(path)
	}

	tempPath := path + ".taglib-tmp" + formatExt(realFormat)
	if err := os.Rename(path, tempPath); err != nil {
		return fmt.Errorf("staging %s at its real format's extension: %w", path, err)
	}
	defer func() {
		if _, statErr := os.Stat(tempPath); statErr == nil {
			os.Rename(tempPath, path)
		}
	}()

	return fn(tempPath)
}
