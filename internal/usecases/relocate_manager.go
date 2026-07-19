package usecases

import (
	"context"
	"errors"
	"log"
)

// ErrRelocateInProgress is returned by RelocateManager.Start when a
// relocate job is already running.
var ErrRelocateInProgress = ErrJobInProgress

// ErrBlockedByScan is returned by RelocateManager.Start when a scan
// refresh is currently running. Deliberately a distinct value from
// ErrRelocateInProgress (which aliases the shared ErrJobInProgress) so
// callers can tell "relocate itself is busy" apart from "blocked by a
// different job" via errors.Is — aliasing this to ErrRefreshInProgress
// would be indistinguishable from ErrRelocateInProgress, since both are
// the same underlying ErrJobInProgress value.
var ErrBlockedByScan = errors.New("blocked by a running scan refresh")

// RelocateStatus is a snapshot of the current/most recent relocate job.
type RelocateStatus = JobStatus

// RelocateManager coordinates a single background relocate job (over a
// list of paths) at a time, via its own JobManager. Unlike identify,
// enrich, and tag — which share no concurrency guard with anything —
// relocate is checked against scan's running state before starting: a
// scan refresh walking /music concurrently with a file being moved could
// see it as both missing at its old location and new at its new one.
type RelocateManager struct {
	relocate   *RelocateFile
	job        JobManager
	scanStatus StatusChecker
}

func NewRelocateManager(relocate *RelocateFile, scanStatus StatusChecker) *RelocateManager {
	return &RelocateManager{relocate: relocate, scanStatus: scanStatus}
}

// Start begins relocating paths in the background if no relocate job is
// currently running and no scan refresh is running. It returns
// ErrRelocateInProgress or ErrBlockedByScan accordingly. A path that isn't
// both identified and tagged is skipped, logged, and does not abort the
// rest of the job — same as a per-file relocation failure.
func (m *RelocateManager) Start(paths []string) error {
	if m.scanStatus != nil && m.scanStatus.Status().Running {
		return ErrBlockedByScan
	}

	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		for i, path := range paths {
			skipped, err := m.relocate.Relocate(context.Background(), path)
			switch {
			case skipped:
				log.Printf("relocate job: %s is not a tracked, identified, and tagged file, skipping", path)
			case err != nil:
				log.Printf("relocate job: %s: %v", path, err)
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent relocate job.
func (m *RelocateManager) Status() RelocateStatus {
	return m.job.Status()
}
