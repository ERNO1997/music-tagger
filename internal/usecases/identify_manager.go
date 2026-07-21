package usecases

import (
	"context"
	"log"
)

// ErrIdentifyInProgress is returned by IdentifyManager.Start when an
// identify job is already running.
var ErrIdentifyInProgress = ErrJobInProgress

// IdentifyStatus is a snapshot of the current/most recent identify job.
type IdentifyStatus = JobStatus

// IdentifyManager coordinates a single background identify job (over a
// list of paths) at a time, via its own JobManager — independent of
// RefreshManager's guard, since scanning and identifying touch different
// resources and neither needs to block the other.
type IdentifyManager struct {
	identify *IdentifyFile
	job      JobManager
}

func NewIdentifyManager(identify *IdentifyFile) *IdentifyManager {
	return &IdentifyManager{identify: identify}
}

// Start begins identifying paths in the background if no identify job is
// currently running. It returns ErrIdentifyInProgress otherwise. A
// per-file failure (unknown path, fingerprint computation failure, or
// gateway error) is logged and does not abort the rest of the job.
func (m *IdentifyManager) Start(paths []string) error {
	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		for i, path := range paths {
			skipped, err := m.identify.Identify(context.Background(), path)
			switch {
			case err != nil:
				log.Printf("identify job: %s: %v", path, err)
			case skipped:
				log.Printf("identify job: %s skipped (not tracked, or fingerprinting failed)", path)
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent identify job.
func (m *IdentifyManager) Status() IdentifyStatus {
	return m.job.Status()
}

// ResolveAmbiguous records candidate recordingMBID as path's resolved
// identification, delegating to the underlying IdentifyFile. found is false
// (with a nil error) when recordingMBID doesn't match any of path's stored
// candidates.
func (m *IdentifyManager) ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error) {
	return m.identify.ResolveAmbiguous(ctx, path, recordingMBID)
}
