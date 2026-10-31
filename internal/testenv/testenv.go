// Package testenv isolates test binaries from the user's real AgentSync data.
package testenv

import (
	"fmt"
	"os"
	"testing"

	"agentsync/internal/config"
)

const (
	tempDirPattern  = "agentsync-test-config-*"
	homeOverrideEnv = "AGENTSYNC_HOME_OVERRIDE"
	setupFailedCode = 1
)

// Run points the config directory and home override at a throwaway directory
// before any test runs, so no test can write into the real user profile.
func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", tempDirPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testenv: create temp config dir: %v\n", err)
		return setupFailedCode
	}
	defer os.RemoveAll(dir)

	_ = os.Setenv(config.ConfigEnvVar, dir)
	_ = os.Setenv(homeOverrideEnv, dir)
	return m.Run()
}
