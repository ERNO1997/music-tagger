package usecases

import (
	"context"
	"errors"
	"log"
	"sync"
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

// ErrBlockedByAnalysis is returned by RelocateManager.Start when a
// background analysis pass is currently running — the same reason scan
// and relocate already exclude each other: both analysis and relocate
// read/write a file's tracked path, and running concurrently could race.
var ErrBlockedByAnalysis = errors.New("blocked by a running background analysis pass")

// RelocateStatus is a snapshot of the current/most recent relocate job.
type RelocateStatus = JobStatus

// Relocation is one file successfully moved by a relocate job, reported so
// a client tracking a selection by path can update a stale path once a
// file moves.
type Relocation struct {
	OldPath string
	NewPath string
}

// RelocateManager coordinates a single background relocate job (over a
// list of paths) at a time, via its own JobManager. Unlike identify,
// enrich, and tag — which share no concurrency guard with anything —
// relocate is checked against scan's running state before starting: a
// scan refresh walking /music concurrently with a file being moved could
// see it as both missing at its old location and new at its new one.
type RelocateManager struct {
	relocate       *RelocateFile
	job            JobManager
	scanStatus     StatusChecker
	analysisStatus StatusChecker

	// mu guards relocations, which is accumulated by the job goroutine
	// (Start) and read concurrently by Relocations() (typically an HTTP
	// status poll) — a separate mutex from JobManager's own, since
	// relocations isn't part of the shared JobStatus.
	mu          sync.Mutex
	relocations []Relocation
}

func NewRelocateManager(relocate *RelocateFile, scanStatus StatusChecker) *RelocateManager {
	return &RelocateManager{relocate: relocate, scanStatus: scanStatus}
}

// SetAnalysisStatus wires the background analysis pass's status checker in
// after both managers exist, mirroring RefreshManager.SetRelocateStatus.
// Must be called before the automatic startup scan (and its chained
// analysis pass) is triggered.
func (m *RelocateManager) SetAnalysisStatus(s StatusChecker) {
	m.analysisStatus = s
}

// Start begins relocating paths in the background if no relocate job is
// currently running, no scan refresh is running, and no background
// analysis pass is running. It returns ErrRelocateInProgress,
// ErrBlockedByScan, or ErrBlockedByAnalysis accordingly. A path that isn't
// both identified and tagged is skipped, logged, and does not abort the
// rest of the job — same as a per-file relocation failure.
func (m *RelocateManager) Start(paths []string) error {
	if m.scanStatus != nil && m.scanStatus.Status().Running {
		return ErrBlockedByScan
	}
	if m.analysisStatus != nil && m.analysisStatus.Status().Running {
		return ErrBlockedByAnalysis
	}

	m.mu.Lock()
	m.relocations = nil
	m.mu.Unlock()

	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		for i, path := range paths {
			newPath, skipped, err := m.relocate.Relocate(context.Background(), path)
			switch {
			case skipped:
				log.Printf("relocate job: %s is not a tracked, identified, and tagged file, skipping", path)
			case err != nil:
				log.Printf("relocate job: %s: %v", path, err)
			default:
				m.mu.Lock()
				m.relocations = append(m.relocations, Relocation{OldPath: path, NewPath: newPath})
				m.mu.Unlock()
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent relocate job.
func (m *RelocateManager) Status() RelocateStatus {
	return m.job.Status()
}

// Relocations returns every file successfully relocated by the current (or
// most recently completed) job, accumulated since Start was last called.
func (m *RelocateManager) Relocations() []Relocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Relocation(nil), m.relocations...)
}
