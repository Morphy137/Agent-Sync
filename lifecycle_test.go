package main

import (
	"os"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"

	"agentsync/internal/orchestrator"
	"agentsync/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m))
}

func TestParseLaunchArgs(t *testing.T) {
	if parseLaunchArgs(nil).minimized {
		t.Error("no arguments must not start minimized")
	}
	if !parseLaunchArgs([]string{"--minimized"}).minimized || !parseLaunchArgs([]string{"--MINIMIZED"}).minimized {
		t.Error("--minimized was not recognized")
	}
}

func TestWindowStartHidesOnlyWhenTheTrayCanBringItBack(t *testing.T) {
	cases := []struct {
		name        string
		minimized   bool
		closeToTray bool
		wantHidden  bool
		wantState   options.WindowStartState
	}{
		{"normal launch", false, false, false, options.Normal},
		{"normal launch with tray", false, true, false, options.Normal},
		{"startup entry with tray", true, true, true, options.Normal},
		{"startup entry without tray", true, false, false, options.Minimised},
	}
	for _, tc := range cases {
		hidden, state := launchOptions{minimized: tc.minimized}.windowStart(tc.closeToTray)
		if hidden != tc.wantHidden || state != tc.wantState {
			t.Errorf("%s: hidden=%v state=%v, want hidden=%v state=%v", tc.name, hidden, state, tc.wantHidden, tc.wantState)
		}
	}
}

func TestSingleInstanceIDFollowsTheDataFolder(t *testing.T) {
	a := singleInstanceID(`C:\Users\me\AppData\Roaming\AgentSync`, false)
	if a != singleInstanceID(`c:\users\me\appdata\roaming\agentsync\`, false) {
		t.Error("the same folder must give the same id, whatever its case or trailing slash")
	}
	if a == singleInstanceID(`D:\Portable\data`, false) {
		t.Error("different data folders must not share a lock")
	}
	if a == singleInstanceID(`C:\Users\me\AppData\Roaming\AgentSync`, true) {
		t.Error("development builds must not share the lock of an installed copy")
	}
}

func TestClosePolicyHidesOnlyWhileTrayIsActiveAndNotQuitting(t *testing.T) {
	trayActive := false
	policy := closePolicy{trayActive: func() bool { return trayActive }}

	if policy.shouldHide() {
		t.Error("without a tray icon closing must quit")
	}
	trayActive = true
	if !policy.shouldHide() {
		t.Error("with a tray icon closing must hide")
	}
	policy.quitting.Store(true)
	if policy.shouldHide() {
		t.Error("a real quit (tray Quit, installing an update) must not be trapped")
	}
	if (&closePolicy{}).shouldHide() {
		t.Error("a policy without a tray must never hide")
	}
}

func TestPlanHasWork(t *testing.T) {
	cases := []struct {
		name string
		plan *orchestrator.SyncPlan
		want bool
	}{
		{"nil", nil, false},
		{"idle", &orchestrator.SyncPlan{Direction: orchestrator.DirectionIdle}, false},
		{"offline", &orchestrator.SyncPlan{Direction: orchestrator.DirectionOffline}, false},
		{"incoming", &orchestrator.SyncPlan{Direction: orchestrator.DirectionDown}, true},
		{"conflict", &orchestrator.SyncPlan{Direction: orchestrator.DirectionConflict}, true},
		{"pending rules", &orchestrator.SyncPlan{Direction: orchestrator.DirectionIdle, RulesPending: 2}, true},
		{"conflicting rules", &orchestrator.SyncPlan{Direction: orchestrator.DirectionIdle, RulesConflicts: 1}, true},
	}
	for _, tc := range cases {
		if got := planHasWork(tc.plan); got != tc.want {
			t.Errorf("%s: planHasWork = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTrayLabelsFollowTheConfiguredLanguage(t *testing.T) {
	if got := trayLabelsFor("es"); got != trayMessages["es"] {
		t.Errorf("es labels = %+v", got)
	}
	if got := trayLabelsFor("en"); got != trayMessages["en"] {
		t.Errorf("en labels = %+v", got)
	}
	if trayMessages["es"].Quit == "" || trayMessages["es"].Open == trayMessages["en"].Open {
		t.Error("Spanish labels are missing or untranslated")
	}
	for lang, labels := range trayMessages {
		if labels.Open == "" || labels.SyncNow == "" || labels.Quit == "" {
			t.Errorf("%s has an empty label: %+v", lang, labels)
		}
	}
}
