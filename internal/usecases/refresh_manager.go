package usecases

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrRefreshInProgress is returned by RefreshManager.Start when a refresh is
// already running.
var ErrRefreshInProgress = errors.New("refresh already in progress")

// RefreshStatus is a snapshot of the current/most recent refresh state.
type RefreshStatus struct {
	Running   bool
	Processed int
	Total     int
}

// RefreshManager coordinates a single background ScanLocalVolume.Refresh at
// a time and exposes its live progress for polling. Only one refresh may
// run at once; a concurrent trigger is rejected rather than queued or
// fanned out.
type RefreshManager struct {
	scanner *ScanLocalVolume
	root    string

	mu        sync.Mutex
	running   bool
	processed int
	total     int
	startedAt time.Time
}

func NewRefreshManager(scanner *ScanLocalVolume, root string) *RefreshManager {
	return &RefreshManager{scanner: scanner, root: root}
}

// Start begins a refresh in the background if none is currently running.
// It returns ErrRefreshInProgress otherwise. The refresh runs decoupled
// from any caller's request context (using context.Background()) since it
// must keep running after the triggering HTTP request/response completes.
func (m *RefreshManager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrRefreshInProgress
	}
	m.running = true
	m.processed = 0
	m.total = 0
	m.startedAt = time.Now()
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
		}()

		_, _ = m.scanner.Refresh(context.Background(), m.root, func(processed, total int) {
			m.mu.Lock()
			m.processed = processed
			m.total = total
			m.mu.Unlock()
		})
	}()

	return nil
}

// Status returns a snapshot of the current/most recent refresh.
func (m *RefreshManager) Status() RefreshStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return RefreshStatus{Running: m.running, Processed: m.processed, Total: m.total}
}
