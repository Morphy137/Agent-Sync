package rulesync_test

import (
	"testing"

	"agentsync/internal/rulesync"
)

func TestEvaluateFileStatusTableRows(t *testing.T) {
	tests := []struct {
		name         string
		sourceHash   string
		targetHash   string
		baselineHash string
		expected     string
	}{
		{"source equals target", "hashA", "hashA", "hashA", rulesync.StatusInSync},
		{"source equals target without baseline", "hashA", "hashA", "", rulesync.StatusInSync},
		{"no baseline and no target", "hashA", "", "", rulesync.StatusNewInSource},
		{"no baseline and a different target", "hashA", "hashB", "", rulesync.StatusUntrackedDiff},
		{"only the source changed", "hashNewSource", "hashBaseline", "hashBaseline", rulesync.StatusSourceChanged},
		{"only the target changed", "hashBaseline", "hashModifiedTarget", "hashBaseline", rulesync.StatusTargetChanged},
		{"target deleted", "hashBaseline", "", "hashBaseline", rulesync.StatusTargetMissing},
		{"target deleted and source changed", "hashNewSource", "", "hashOldBaseline", rulesync.StatusSourceChanged},
		{"both changed differently", "hashSourceNew", "hashTargetNew", "hashOldBaseline", rulesync.StatusConflict},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rulesync.EvaluateFileStatus(tc.sourceHash, tc.targetHash, tc.baselineHash)
			if got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}
