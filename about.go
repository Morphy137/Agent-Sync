package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/updater"
)

// changelog is CHANGELOG.md, embedded so the app can show "What's new" offline.
//
//go:embed CHANGELOG.md
var changelog string

// wailsConfig is wails.json; its info.productVersion is the single source of the app version.
//
//go:embed wails.json
var wailsConfig []byte

const (
	// devVersion is reported when wails.json has no product version.
	devVersion = "dev"
	// uninstallerName is written next to the executable by the NSIS installer.
	uninstallerName = "uninstall.exe"
	// installerSilentFlag and installerRelaunchFlag make the installer update quietly and reopen the app.
	installerSilentFlag   = "/S"
	installerRelaunchFlag = "/relaunch"
	updateCheckTimeout    = 20 * time.Second
)

// AppInfo describes this copy of AgentSync.
type AppInfo struct {
	Version string `json:"version"`
	// Installed is true when the NSIS installer put this copy in place (updates can be automatic).
	Installed bool `json:"installed"`
	// Portable is true when data lives next to the executable (a "portable" file sits beside it).
	Portable bool   `json:"portable"`
	DataDir  string `json:"dataDir"`
}

// UpdateStatus is the result of an update check.
type UpdateStatus struct {
	Current   string           `json:"current"`
	Available bool             `json:"available"`
	Release   *updater.Release `json:"release,omitempty"`
}

// appVersion reads info.productVersion from the embedded wails.json.
func appVersion() string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfig, &cfg); err != nil || cfg.Info.ProductVersion == "" {
		return devVersion
	}
	return cfg.Info.ProductVersion
}

// isInstalled reports whether the executable sits next to the installer's uninstaller.
func isInstalled() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), uninstallerName))
	return err == nil
}

// GetChangelog returns the user-facing changelog (Markdown).
func (a *App) GetChangelog() string {
	return changelog
}

// GetAppInfo returns the version and how this copy was installed.
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Version:   appVersion(),
		Installed: isInstalled(),
		Portable:  config.IsPortable(),
		DataDir:   config.GetConfigDir(),
	}
}

// CheckForUpdate asks GitHub for the newest release.
func (a *App) CheckForUpdate() (*UpdateStatus, error) {
	ctx, cancel := context.WithTimeout(a.ctx, updateCheckTimeout)
	defer cancel()
	current := appVersion()
	latest, err := updater.NewClient().Latest(ctx)
	if err != nil {
		return nil, err
	}
	return &UpdateStatus{Current: current, Available: updater.IsNewer(latest, current), Release: latest}, nil
}

// InstallUpdate downloads and verifies the newest installer, starts it silently and closes the
// app; the installer reopens AgentSync when it finishes. Copies not installed with the installer
// (portable or a loose .exe) must be updated by hand.
func (a *App) InstallUpdate() error {
	if !isInstalled() || config.IsPortable() {
		return apperr.New(apperr.UpdateManualRequired)
	}
	status, err := a.CheckForUpdate()
	if err != nil {
		return err
	}
	if !status.Available {
		return apperr.New(apperr.UpdateUnavailable)
	}
	installer, err := updater.NewClient().Download(a.ctx, status.Release, os.TempDir())
	if err != nil {
		return err
	}
	if err := exec.Command(installer, installerSilentFlag, installerRelaunchFlag).Start(); err != nil {
		return apperr.Wrap(apperr.UpdateDownloadFailed, err)
	}
	runtime.Quit(a.ctx)
	return nil
}
