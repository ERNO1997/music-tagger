package usecases

import "context"

// ErrRefreshInProgress is returned by RefreshManager.Start when a refresh is
// already running.
var ErrRefreshInProgress = ErrJobInProgress

// RefreshStatus is a snapshot of the current/most recent refresh state.
type RefreshStatus = JobStatus

// RefreshManager coordinates a single background ScanLocalVolume.Refresh at
// a time and exposes its live progress for polling, via the shared
// JobManager. Only one refresh may run at once; a concurrent trigger is
// rejected rather than queued or fanned out.
type RefreshManager struct {
	scanner *ScanLocalVolume
	root    string
	job     JobManager
}

func NewRefreshManager(scanner *ScanLocalVolume, root string) *RefreshManager {
	return &RefreshManager{scanner: scanner, root: root}
}

// Start begins a refresh in the background if none is currently running.
// It returns ErrRefreshInProgress otherwise.
func (m *RefreshManager) Start() error {
	return m.job.Start(func(report func(processed, total int)) {
		_, _ = m.scanner.Refresh(context.Background(), m.root, report)
	})
}

// Status returns a snapshot of the current/most recent refresh.
func (m *RefreshManager) Status() RefreshStatus {
	return m.job.Status()
}
