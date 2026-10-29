package scheduler_test

import (
	"sync/atomic"
	"testing"
	"time"

	"agentsync/internal/scheduler"
)

func TestSchedulerDebounceSkipAndIntervalChange(t *testing.T) {
	var checkCount atomic.Int32
	var simulatedBusy atomic.Bool

	sched := scheduler.NewScheduler(func() bool {
		if simulatedBusy.Load() {
			return false
		}
		checkCount.Add(1)
		return true
	})

	// 1. Force check runs immediately
	if !sched.TriggerNow(true) {
		t.Errorf("expected forced TriggerNow to succeed")
	}
	if checkCount.Load() != 1 {
		t.Errorf("expected checkCount=1, got %d", checkCount.Load())
	}

	// 2. Non-forced check within MinCheckDebounce (30s) is debounced
	if sched.RunCheck(false) {
		t.Errorf("expected unforced RunCheck within debounce window to be skipped")
	}
	if checkCount.Load() != 1 {
		t.Errorf("expected checkCount still 1 after debounce skip, got %d", checkCount.Load())
	}

	// 3. Forced check when lock is busy skips without blocking
	simulatedBusy.Store(true)
	if sched.TriggerNow(true) {
		t.Errorf("expected TriggerNow when busy to return false")
	}
	if checkCount.Load() != 1 {
		t.Errorf("expected checkCount still 1 when busy")
	}
	simulatedBusy.Store(false)

	// 4. Interval ticker and dynamic interval change
	sched.SetTickerUnitOverride(15 * time.Millisecond)
	sched.Start(0)
	if sched.IsRunning() {
		t.Errorf("expected scheduler not running when interval=0")
	}

	sched.Start(1) // 15ms ticker
	if !sched.IsRunning() {
		t.Errorf("expected scheduler running when interval=1")
	}

	// Change interval dynamically to 2
	sched.Start(2)
	if !sched.IsRunning() {
		t.Errorf("expected scheduler running after interval update")
	}

	sched.Stop()
	if sched.IsRunning() {
		t.Errorf("expected scheduler stopped after Stop()")
	}
}
