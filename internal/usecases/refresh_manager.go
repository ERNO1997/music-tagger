package usecases

import (
	"context"
	"errors"
)

// ErrRefreshInProgress is returned by RefreshManager.Start when a refresh is
// already running.
var ErrRefreshInProgress = ErrJobInProgress

// ErrBlockedByRelocate is returned by RefreshManager.Start when a relocate
// job is currently running. Deliberately a distinct value from
// ErrRefreshInProgress (which aliases the shared ErrJobInProgress) — see
// ErrBlockedByScan's doc comment for why aliasing would make the two
// indistinguishable via errors.Is.
var ErrBlockedByRelocate = errors.New("blocked by a running relocate job")

// RefreshStatus is a snapshot of the current/most recent refresh state.
type RefreshStatus = JobStatus

// RefreshManager coordinates a single background ScanLocalVolume.Refresh at
// a time and exposes its live progress for polling, via the shared
// JobManager. Only one refresh may run at once; a concurrent trigger is
// rejected rather than queued or fanned out.
type RefreshManager struct {
	scanner        *ScanLocalVolume
	root           string
	job            JobManager
	relocateStatus StatusChecker
}

func NewRefreshManager(scanner *ScanLocalVolume, root string) *RefreshManager {
	return &RefreshManager{scanner: scanner, root: root}
}

// SetRelocateStatus wires the relocate job's status checker in after both
// managers exist (RefreshManager is constructed first in the composition
// root), so scan and relocate mutually exclude each other without a
// construction-order dependency. Must be called before the automatic
// startup scan is triggered.
func (m *RefreshManager) SetRelocateStatus(s StatusChecker) {
	m.relocateStatus = s
}

// Start begins a refresh in the background if none is currently running
// and no relocate job is running. It returns ErrRefreshInProgress or
// ErrBlockedByRelocate accordingly.
func (m *RefreshManager) Start() error {
	if m.relocateStatus != nil && m.relocateStatus.Status().Running {
		return ErrBlockedByRelocate
	}

	return m.job.Start(func(report func(processed, total int)) {
		_, _ = m.scanner.Refresh(context.Background(), m.root, report)
	})
}

// Status returns a snapshot of the current/most recent refresh.
func (m *RefreshManager) Status() RefreshStatus {
	return m.job.Status()
}
