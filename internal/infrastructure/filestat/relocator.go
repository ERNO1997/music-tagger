package filestat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"music-tagger/internal/usecases"
)

// PathRelocator is a Relocator that physically moves audio files into a
// canonical Artist/Album/Track hierarchy under a configured music root.
type PathRelocator struct {
	musicRoot string
}

func NewPathRelocator(musicRoot string) *PathRelocator {
	return &PathRelocator{musicRoot: musicRoot}
}

// sanitizeSegment strips the filesystem-prohibited characters listed in
// the project charter (§4.3) from a path segment. Callers must sanitize
// before using a segment in any filesystem call — never after.
func sanitizeSegment(s string) string {
	return prohibitedCharsReplacer.Replace(s)
}

var prohibitedCharsReplacer = strings.NewReplacer(
	`\`, "",
	"/", "",
	":", "",
	"*", "",
	"?", "",
	`"`, "",
	"<", "",
	">", "",
	"|", "",
)

// albumDirName renders the album directory name as "{year} - {album}"
// when year is known, or just "{album}" when it isn't (year <= 0, meaning
// the release had no usable date) — omitted rather than shown as a
// placeholder, consistent with how the rest of the app treats unresolved
// optional fields. Sanitizes the album segment only; the year, being
// digits, needs no sanitization.
func albumDirName(year int, album string) string {
	sanitizedAlbum := sanitizeSegment(album)
	if year <= 0 {
		return sanitizedAlbum
	}
	return fmt.Sprintf("%d - %s", year, sanitizedAlbum)
}

// ComputeDestination returns path's canonical destination under
// {musicRoot}/{sanitized artist}/{year - }{sanitized album}/{zero-padded
// track} - {sanitized title}{ext} — the year prefix is omitted when the
// release has no usable date (Year <= 0) — with no filesystem access.
// path is used only for its extension; it need not exist on disk. Reused
// by Relocate (which then performs the move) and by the background
// analysis pass (which only needs to compare a file's current path
// against this, never moving it).
func (r *PathRelocator) ComputeDestination(path string, meta usecases.RelocateInput) string {
	ext := filepath.Ext(path)
	dir := filepath.Join(r.musicRoot, sanitizeSegment(meta.Artist), albumDirName(meta.Year, meta.Album))
	filename := fmt.Sprintf("%02d - %s%s", meta.TrackNumber, sanitizeSegment(meta.Title), ext)
	return filepath.Join(dir, filename)
}

// Relocate moves path to its computed destination (see ComputeDestination).
// If the destination already exists, or the move fails at any step, path is
// left untouched and an error is returned. If the file is already at its
// computed destination (e.g. relocating an already-relocated file again
// after a no-op re-identification), this is a successful no-op, not a
// collision with itself.
func (r *PathRelocator) Relocate(ctx context.Context, path string, meta usecases.RelocateInput) (string, error) {
	dest := r.ComputeDestination(path, meta)
	dir := filepath.Dir(dest)

	if filepath.Clean(dest) == filepath.Clean(path) {
		return dest, nil
	}

	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking destination %s: %w", dest, err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating destination directory %s: %w", dir, err)
	}

	if err := os.Rename(path, dest); err != nil {
		return "", fmt.Errorf("moving %s to %s: %w", path, dest, err)
	}
	r.removeEmptyDirs(filepath.Dir(path))

	return dest, nil
}

// Undo moves a file from currentPath back to originalPath — a bare move,
// no sanitization or directory creation, since originalPath's directory
// already existed. Used as a best-effort rollback when recording a
// successful relocation fails.
func (r *PathRelocator) Undo(ctx context.Context, currentPath, originalPath string) error {
	if err := os.Rename(currentPath, originalPath); err != nil {
		return fmt.Errorf("moving %s back to %s: %w", currentPath, originalPath, err)
	}
	r.removeEmptyDirs(filepath.Dir(currentPath))
	return nil
}

// junkFiles are OS-generated files (not part of the user's music library)
// that don't prevent a directory from being considered empty for cleanup
// purposes — notably macOS's .DS_Store, since Finder recreates one in
// nearly every directory it has ever browsed, which would otherwise
// defeat empty-directory removal in practice.
var junkFiles = map[string]bool{
	".DS_Store":   true,
	"Thumbs.db":   true,
	"desktop.ini": true,
}

func isJunkFile(name string) bool {
	if junkFiles[name] {
		return true
	}
	// AppleDouble resource-fork sidecar files, e.g. "._song.mp3".
	return strings.HasPrefix(name, "._")
}

// removeEmptyDirs walks upward from dir, removing each directory that
// contains nothing but junk files (deleting those junk files first),
// stopping at the first directory containing a real file or subdirectory,
// or at musicRoot itself — never above it. Best-effort: any error (a
// permissions issue, a directory that isn't actually empty once junk is
// disregarded) simply stops the walk rather than failing the relocation
// that triggered it, since this is cleanup, not part of its correctness.
func (r *PathRelocator) removeEmptyDirs(dir string) {
	root := filepath.Clean(r.musicRoot)
	dir = filepath.Clean(dir)

	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, entry := range entries {
			if entry.IsDir() || !isJunkFile(entry.Name()) {
				return
			}
		}

		for _, entry := range entries {
			os.Remove(filepath.Join(dir, entry.Name()))
		}

		if err := os.Remove(dir); err != nil {
			return
		}

		dir = filepath.Dir(dir)
	}
}
