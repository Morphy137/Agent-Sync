package config

import (
	"os"
	"path/filepath"
)

const (
	// PortableMarkerName is a file placed next to the executable to enable portable mode.
	PortableMarkerName = "portable"
	// PortableDataDirName is the folder next to the executable that holds the data in portable mode.
	PortableDataDirName = "data"
)

// executablePath is replaceable in tests.
var executablePath = os.Executable

// PortableDataDir returns <exe folder>/data when a "portable" marker file sits next to the
// executable (portable mode keeps config, state and backups beside the program), or "".
func PortableDataDir() string {
	exe, err := executablePath()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	info, err := os.Stat(filepath.Join(dir, PortableMarkerName))
	if err != nil || info.IsDir() {
		return ""
	}
	return filepath.Join(dir, PortableDataDirName)
}

// IsPortable reports whether this run stores its data next to the executable.
func IsPortable() bool {
	return os.Getenv(ConfigEnvVar) == "" && PortableDataDir() != ""
}
