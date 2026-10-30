package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
)

const (
	CurrentVersion              = 1
	ConfigFileName              = "config.json"
	ConfigEnvVar                = "AGENTSYNC_CONFIG_DIR"
	DefaultBackupRetention      = 20
	DefaultCheckIntervalMinutes = 5
	DefaultLanguage             = "system"
	DefaultTheme                = "system"
	DefaultConfirmBeforeSync    = true
)

// Connection links a profile name to a target directory.
type Connection struct {
	ID         string `json:"id"`
	Profile    string `json:"profile"`
	TargetPath string `json:"targetPath"`
	CreatedAt  string `json:"createdAt"`
}

// Config stores user preferences and configured connections.
type Config struct {
	Version              int          `json:"version"`
	RootPath             string       `json:"rootPath"`
	BackupPath           string       `json:"backupPath"`
	BackupRetention      int          `json:"backupRetention"`
	CheckIntervalMinutes int          `json:"checkIntervalMinutes"`
	Language             string       `json:"language"`
	Theme                string       `json:"theme"`
	ConfirmBeforeSync    bool         `json:"confirmBeforeSync"`
	Connections          []Connection `json:"connections"`
}

// GetConfigDir returns the active configuration directory.
func GetConfigDir() string {
	if envDir := os.Getenv(ConfigEnvVar); envDir != "" {
		return envDir
	}
	userDir, err := os.UserConfigDir()
	if err != nil {
		userDir = "."
	}
	return filepath.Join(userDir, "AgentSync")
}

// DefaultConfig returns a new Config with standard default values.
func DefaultConfig() *Config {
	configDir := GetConfigDir()
	defaultBackup := filepath.Join(configDir, "backups")

	return &Config{
		Version:              CurrentVersion,
		RootPath:             "",
		BackupPath:           defaultBackup,
		BackupRetention:      DefaultBackupRetention,
		CheckIntervalMinutes: DefaultCheckIntervalMinutes,
		Language:             DefaultLanguage,
		Theme:                DefaultTheme,
		ConfirmBeforeSync:    DefaultConfirmBeforeSync,
		Connections:          make([]Connection, 0),
	}
}

// Load loads the configuration from disk, returning defaults if not found.
func Load() (*Config, error) {
	configDir := GetConfigDir()
	configPath := filepath.Join(configDir, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			return cfg, nil
		}
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	// Apply defaults if zero values exist
	if cfg.BackupRetention <= 0 {
		cfg.BackupRetention = DefaultBackupRetention
	}
	if cfg.CheckIntervalMinutes < 0 {
		cfg.CheckIntervalMinutes = DefaultCheckIntervalMinutes
	}
	if cfg.Language == "" {
		cfg.Language = DefaultLanguage
	}
	if cfg.Theme == "" {
		cfg.Theme = DefaultTheme
	}
	if cfg.BackupPath == "" {
		cfg.BackupPath = filepath.Join(configDir, "backups")
	}
	if cfg.Connections == nil {
		cfg.Connections = make([]Connection, 0)
	}

	return cfg, nil
}

// Save writes the configuration atomically to disk.
func Save(cfg *Config) error {
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	configPath := filepath.Join(configDir, ConfigFileName)
	return fsops.AtomicWrite(configPath, data, fsops.DefaultFilePerms)
}

// IsSubdirectory returns true if childPath is equal to or inside parentPath.
func IsSubdirectory(childPath, parentPath string) bool {
	if parentPath == "" || childPath == "" {
		return false
	}
	absParent, err := filepath.Abs(parentPath)
	if err != nil {
		return false
	}
	absChild, err := filepath.Abs(childPath)
	if err != nil {
		return false
	}
	if strings.EqualFold(absParent, absChild) {
		return true
	}
	rel, err := filepath.Rel(absParent, absChild)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !strings.HasPrefix(rel, "/") && !strings.HasPrefix(rel, "\\")
}

// ValidateConnection checks that a connection target is valid.
func ValidateConnection(rootPath, targetPath string, existing []Connection, currentID string) error {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return apperr.New(apperr.TargetNotFound).WithParam("path", targetPath)
	}

	targetInfo, err := os.Stat(absTarget)
	if err != nil {
		return apperr.New(apperr.TargetNotFound).WithParam("path", targetPath)
	}
	if !targetInfo.IsDir() {
		return apperr.New(apperr.TargetNotFound).WithParam("path", targetPath)
	}

	if rootPath != "" && IsSubdirectory(absTarget, rootPath) {
		return apperr.New(apperr.TargetInsideRoot).WithParam("path", targetPath)
	}

	for _, conn := range existing {
		if conn.ID == currentID {
			continue
		}
		absExisting, err := filepath.Abs(conn.TargetPath)
		if err == nil && strings.EqualFold(absExisting, absTarget) {
			return apperr.New(apperr.TargetAlreadyConnected).WithParam("path", targetPath)
		}
	}

	return nil
}

// FormatRFC3339 formats a time.Time using RFC3339.
func FormatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
