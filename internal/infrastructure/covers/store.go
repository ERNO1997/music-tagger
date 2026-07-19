// Package covers persists downloaded cover art images as files on disk,
// keyed by MusicBrainz Release ID — deliberately not stored as SQLite
// BLOBs, to keep the tracking database itself small.
package covers

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store manages cover art files under a "covers" subdirectory alongside
// wherever the tracking database lives (the same /data volume, no separate
// configuration needed).
type Store struct {
	dir string
}

func NewStore(baseDir string) (*Store, error) {
	dir := filepath.Join(baseDir, "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cover art directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Path returns the on-disk path for a release's cover art and whether a
// file already exists there — callers use this to skip a redundant
// Cover Art Archive lookup/download when another track on the same
// release has already been enriched.
func (s *Store) Path(releaseMBID string) (path string, exists bool) {
	path = filepath.Join(s.dir, releaseMBID+".jpg")
	_, err := os.Stat(path)
	return path, err == nil
}

// Save writes image bytes to disk for a release and returns the path.
func (s *Store) Save(releaseMBID string, data []byte) (string, error) {
	path := filepath.Join(s.dir, releaseMBID+".jpg")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing cover art for %s: %w", releaseMBID, err)
	}
	return path, nil
}
