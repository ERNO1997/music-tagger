package usecases

import (
	"context"
	"log"
)

// ErrTagInProgress is returned by TagManager.Start when a tag job is
// already running.
var ErrTagInProgress = ErrJobInProgress

// TagStatus is a snapshot of the current/most recent tag job.
type TagStatus = JobStatus

// TagManager coordinates a single background tag-writing job (over a list
// of paths) at a time, via its own JobManager — independent of scan,
// identify, and enrich, since tagging touches the filesystem, a distinct
// resource from any of the three.
type TagManager struct {
	tag *TagFile
	job JobManager
}

func NewTagManager(tag *TagFile) *TagManager {
	return &TagManager{tag: tag}
}

// Start begins tagging paths in the background if no tag job is currently
// running. It returns ErrTagInProgress otherwise. A path that isn't yet
// identified is skipped, logged, and does not abort the rest of the job —
// same as a per-file tag write failure.
func (m *TagManager) Start(paths []string) error {
	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		for i, path := range paths {
			skipped, err := m.tag.Tag(context.Background(), path)
			switch {
			case skipped:
				log.Printf("tag job: %s is not a tracked, identified file, skipping", path)
			case err != nil:
				log.Printf("tag job: %s: %v", path, err)
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent tag job.
func (m *TagManager) Status() TagStatus {
	return m.job.Status()
}
