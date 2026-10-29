package scheduler

import (
	"sync"
	"time"
)

const (
	MinCheckDebounce = 30 * time.Second
)

// CheckFunc is executed by the scheduler when a check runs.
// It should attempt a non-blocking lock (TryLock) and return true if the check was executed.
type CheckFunc func() bool

// Scheduler handles periodic background status checks.
type Scheduler struct {
	checkFunc      CheckFunc
	stopChan       chan struct{}
	lastCheck      time.Time
	mu             sync.Mutex
	isRunning      bool
	tickerDuration time.Duration // custom override for tests; if 0, uses intervalMinutes * time.Minute
}

// NewScheduler creates a new background scheduler using the given CheckFunc.
func NewScheduler(checkFunc CheckFunc) *Scheduler {
	return &Scheduler{
		checkFunc: checkFunc,
		stopChan:  make(chan struct{}),
	}
}

// SetTickerUnitOverride sets a custom base unit for testing (e.g. 20ms instead of 1 minute).
func (s *Scheduler) SetTickerUnitOverride(unit time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickerDuration = unit
}

// Start initiates the periodic ticker based on checkIntervalMinutes.
func (s *Scheduler) Start(intervalMinutes int) {
	s.mu.Lock()
	if s.isRunning {
		close(s.stopChan)
		s.stopChan = make(chan struct{})
		s.isRunning = false
	}

	if intervalMinutes <= 0 {
		s.mu.Unlock()
		return
	}

	unit := time.Minute
	if s.tickerDuration > 0 {
		unit = s.tickerDuration
	}
	dur := time.Duration(intervalMinutes) * unit
	stopCh := s.stopChan
	s.isRunning = true
	s.mu.Unlock()

	ticker := time.NewTicker(dur)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.RunCheck(false)
			case <-stopCh:
				return
			}
		}
	}()
}

// Stop halts the periodic checks.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		close(s.stopChan)
		s.stopChan = make(chan struct{})
		s.isRunning = false
	}
}

// IsRunning returns whether the scheduler ticker is currently active.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// TriggerNow executes an immediate check, bypassing the debounce limit if forced.
func (s *Scheduler) TriggerNow(force bool) bool {
	return s.RunCheck(force)
}

// RunCheck executes the checkFunc if not debounced and if no sync operation is actively running.
func (s *Scheduler) RunCheck(force bool) bool {
	s.mu.Lock()
	now := time.Now()
	if !force && !s.lastCheck.IsZero() && now.Sub(s.lastCheck) < MinCheckDebounce {
		s.mu.Unlock()
		return false
	}
	s.lastCheck = now
	s.mu.Unlock()

	if s.checkFunc == nil {
		return false
	}
	return s.checkFunc()
}
