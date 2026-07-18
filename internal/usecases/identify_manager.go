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
	store    TrackingStore
	job      JobManager
}

func NewIdentifyManager(identify *IdentifyFile, store TrackingStore) *IdentifyManager {
	return &IdentifyManager{identify: identify, store: store}
}

// Start begins identifying paths in the background if no identify job is
// currently running. It returns ErrIdentifyInProgress otherwise. A
// per-file failure (unknown path, no fingerprint, or gateway error) is
// logged and does not abort the rest of the job.
func (m *IdentifyManager) Start(paths []string) error {
	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		records, err := m.store.LoadAll(context.Background())
		if err != nil {
			log.Printf("identify job: loading tracked records: %v", err)
			report(total, total)
			return
		}

		for i, path := range paths {
			rec, ok := records[path]
			switch {
			case !ok:
				log.Printf("identify job: %s is not a tracked file, skipping", path)
			case rec.Fingerprint == "":
				log.Printf("identify job: %s has no usable fingerprint, skipping", path)
			default:
				if err := m.identify.Identify(context.Background(), path, rec.Fingerprint, rec.DurationSeconds); err != nil {
					log.Printf("identify job: %s: %v", path, err)
				}
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent identify job.
func (m *IdentifyManager) Status() IdentifyStatus {
	return m.job.Status()
}
