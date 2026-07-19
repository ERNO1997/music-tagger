package usecases

import (
	"errors"
	"sync"
	"time"
)

// ErrJobInProgress is returned by JobManager.Start when a job is already
// running.
var ErrJobInProgress = errors.New("job already in progress")

// JobStatus is a snapshot of the current/most recent job state.
type JobStatus struct {
	Running   bool
	Processed int
	Total     int
}

// StatusChecker reports whether some other background job is currently
// running, used to gate mutual exclusion between two otherwise-independent
// job types (e.g. scan and relocate) whose concurrent execution isn't
// safe. Every *Manager type already satisfies this via its existing
// Status() method.
type StatusChecker interface {
	Status() JobStatus
}

// JobManager coordinates a single background unit of work at a time and
// exposes its live progress for polling. This is the shared concurrency
// primitive behind both the scan refresh and the identify job: extracted
// once because concurrency-correctness code (mutexes, running-state guards)
// is exactly the kind of logic that shouldn't be hand-duplicated — a race
// or guard bug fixed in one copy but not the other is a real risk.
type JobManager struct {
	mu        sync.Mutex
	running   bool
	processed int
	total     int
	startedAt time.Time
}

// Start begins work in the background if no job is currently running. It
// returns ErrJobInProgress otherwise. work is invoked with a report
// callback it should call to update progress as it proceeds. work runs
// decoupled from any caller's request context (it receives none) since it
// must keep running after the triggering HTTP request/response completes.
func (m *JobManager) Start(work func(report func(processed, total int))) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return ErrJobInProgress
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

		work(func(processed, total int) {
			m.mu.Lock()
			m.processed = processed
			m.total = total
			m.mu.Unlock()
		})
	}()

	return nil
}

// Status returns a snapshot of the current/most recent job.
func (m *JobManager) Status() JobStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return JobStatus{Running: m.running, Processed: m.processed, Total: m.total}
}
