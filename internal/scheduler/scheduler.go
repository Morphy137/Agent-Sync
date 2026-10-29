package scheduler

import (
	"sync"
	"time"

	"agentsync/internal/config"
	"agentsync/internal/orchestrator"
	"agentsync/internal/state"
)

const (
	MinCheckDebounce = 30 * time.Second
)

// StatusEventPayload is emitted to the frontend on status change.
type StatusEventPayload struct {
	Direction   string `json:"direction"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Connections any    `json:"connections"`
	LastCheckAt string `json:"lastCheckAt"`
}

// Scheduler handles periodic background status checks.
type Scheduler struct {
	orch      *orchestrator.Orchestrator
	emitFunc  func(eventName string, data any)
	stopChan  chan struct{}
	lastCheck time.Time
	mu        sync.Mutex
	isRunning bool
}

// NewScheduler creates a new background scheduler.
func NewScheduler(orch *orchestrator.Orchestrator, emitFunc func(eventName string, data any)) *Scheduler {
	return &Scheduler{
		orch:     orch,
		emitFunc: emitFunc,
		stopChan: make(chan struct{}),
	}
}

// Start initiates the periodic ticker based on checkIntervalMinutes.
func (s *Scheduler) Start(intervalMinutes int) {
	s.mu.Lock()
	if s.isRunning {
		close(s.stopChan)
		s.stopChan = make(chan struct{})
	}
	s.isRunning = true
	s.mu.Unlock()

	if intervalMinutes <= 0 {
		return
	}

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.RunCheck(false)
			case <-s.stopChan:
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
		s.isRunning = false
	}
}

// TriggerNow executes an immediate check, bypassing the debounce limit if forced.
func (s *Scheduler) TriggerNow(force bool) {
	s.RunCheck(force)
}

// RunCheck executes PlanSync if not debounced and if no sync operation is actively running.
func (s *Scheduler) RunCheck(force bool) {
	s.mu.Lock()
	now := time.Now()
	if !force && now.Sub(s.lastCheck) < MinCheckDebounce {
		s.mu.Unlock()
		return
	}
	s.lastCheck = now
	s.mu.Unlock()

	// Try to acquire the orchestrator mutex without blocking. If busy, skip check.
	if !s.orch.TryLock() {
		return
	}
	defer s.orch.Unlock()

	cfg, err := config.Load()
	if err != nil || cfg.RootPath == "" {
		return
	}

	st, err := state.Load()
	if err != nil {
		return
	}

	plan, err := s.orch.PlanSync(cfg, st)
	if err != nil {
		return
	}

	nowStr := config.FormatRFC3339(now)
	st.SetTimestamps("", nowStr)
	_ = st.Save()

	if s.emitFunc != nil {
		s.emitFunc("status:changed", StatusEventPayload{
			Direction:   plan.Direction,
			Ahead:       plan.Ahead,
			Behind:      plan.Behind,
			Connections: plan.Connections,
			LastCheckAt: nowStr,
		})
	}
}
