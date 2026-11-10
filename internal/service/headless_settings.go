package service

import (
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
)

// OperationalSettings excludes preferences with desktop-only side effects.
type OperationalSettings struct {
	Language             string `json:"language"`
	BackupPath           string `json:"backupPath"`
	BackupRetention      int    `json:"backupRetention"`
	CheckIntervalMinutes int    `json:"checkIntervalMinutes"`
	ConfirmBeforeSync    bool   `json:"confirmBeforeSync"`
	DisableUpdateCheck   bool   `json:"disableUpdateCheck"`
	AutoApplyUpdates     bool   `json:"autoApplyUpdates"`
}

// OperationalSettingsPatch preserves omitted values, including desktop preferences.
type OperationalSettingsPatch struct {
	Language             *string `json:"language,omitempty"`
	BackupPath           *string `json:"backupPath,omitempty"`
	BackupRetention      *int    `json:"backupRetention,omitempty"`
	CheckIntervalMinutes *int    `json:"checkIntervalMinutes,omitempty"`
	ConfirmBeforeSync    *bool   `json:"confirmBeforeSync,omitempty"`
	DisableUpdateCheck   *bool   `json:"disableUpdateCheck,omitempty"`
	AutoApplyUpdates     *bool   `json:"autoApplyUpdates,omitempty"`
}

func operationalSettings(cfg *config.Config) OperationalSettings {
	return OperationalSettings{Language: cfg.Language, BackupPath: cfg.BackupPath, BackupRetention: cfg.BackupRetention, CheckIntervalMinutes: cfg.CheckIntervalMinutes, ConfirmBeforeSync: cfg.ConfirmBeforeSync, DisableUpdateCheck: cfg.DisableUpdateCheck, AutoApplyUpdates: cfg.AutoApplyUpdates}
}

func (s *Service) GetOperationalSettings() (OperationalSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return operationalSettings(s.cfg), nil
}

// UpdateOperationalSettings merges and persists under the shared service lock.
// Headless callers cannot toggle autostart/tray without applying their desktop effects.
func (s *Service) UpdateOperationalSettings(in OperationalSettingsPatch) (OperationalSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneConfig(s.cfg)
	invalid := func(field string) (OperationalSettings, error) {
		return OperationalSettings{}, apperr.New(apperr.InvalidArgument).WithParam("field", field)
	}
	if in.Language != nil {
		switch *in.Language {
		case "system", "en", "es":
			next.Language = *in.Language
		default:
			return invalid("language")
		}
	}
	if in.BackupPath != nil {
		if strings.TrimSpace(*in.BackupPath) == "" || !filepath.IsAbs(*in.BackupPath) {
			return invalid("backupPath")
		}
		next.BackupPath = *in.BackupPath
	}
	if in.BackupRetention != nil {
		if *in.BackupRetention <= 0 {
			return invalid("backupRetention")
		}
		next.BackupRetention = *in.BackupRetention
	}
	if in.CheckIntervalMinutes != nil {
		if *in.CheckIntervalMinutes < 0 {
			return invalid("checkIntervalMinutes")
		}
		next.CheckIntervalMinutes = *in.CheckIntervalMinutes
	}
	if in.ConfirmBeforeSync != nil {
		next.ConfirmBeforeSync = *in.ConfirmBeforeSync
	}
	if in.DisableUpdateCheck != nil {
		next.DisableUpdateCheck = *in.DisableUpdateCheck
	}
	if in.AutoApplyUpdates != nil {
		next.AutoApplyUpdates = *in.AutoApplyUpdates
	}
	if err := config.Save(next); err != nil {
		return OperationalSettings{}, err
	}
	s.cfg = next
	return operationalSettings(next), nil
}
