package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/profiles"
	"agentsync/internal/scheduler"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

// SettingsPayload defines incoming preferences from the settings view.
type SettingsPayload struct {
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	BackupPath           string `json:"backupPath"`
	BackupRetention      int    `json:"backupRetention"`
	CheckIntervalMinutes int    `json:"checkIntervalMinutes"`
	ConfirmBeforeSync    bool   `json:"confirmBeforeSync"`
}

// InitialLinkResult represents the outcome of connecting a profile.
type InitialLinkResult struct {
	Connection     config.Connection `json:"connection"`
	NeedsDecision  bool              `json:"needsDecision"`
	UntrackedFiles []string          `json:"untrackedFiles"`
}

// GitInfoPayload exposes current git status and branch details.
type GitInfoPayload struct {
	IsInstalled bool   `json:"isInstalled"`
	IsRepo      bool   `json:"isRepo"`
	Branch      string `json:"branch"`
	RemoteURL   string `json:"remoteUrl"`
	HasUpstream bool   `json:"hasUpstream"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
}

// App struct manages application state and exposes bindings to the frontend.
type App struct {
	ctx       context.Context
	orch      *orchestrator.Orchestrator
	scheduler *scheduler.Scheduler
}

// NewApp creates a new App application struct.
func NewApp() *App {
	orch := orchestrator.New()
	app := &App{
		orch: orch,
	}

	app.scheduler = scheduler.NewScheduler(orch, func(eventName string, data any) {
		if app.ctx != nil {
			runtime.EventsEmit(app.ctx, eventName, data)
		}
	})

	return app
}

// startup is called when Wails initializes.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load()
	if err == nil && cfg != nil {
		a.scheduler.Start(cfg.CheckIntervalMinutes)
		go a.scheduler.TriggerNow(true)
	}
}

// generateID returns a random hex string.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetConfig returns the stored configuration.
func (a *App) GetConfig() (*config.Config, error) {
	return config.Load()
}

// UpdateSettings updates user settings and reconfigures the scheduler if needed.
func (a *App) UpdateSettings(payload SettingsPayload) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	intervalChanged := cfg.CheckIntervalMinutes != payload.CheckIntervalMinutes

	if payload.Language != "" {
		cfg.Language = payload.Language
	}
	if payload.Theme != "" {
		cfg.Theme = payload.Theme
	}
	if payload.BackupPath != "" {
		cfg.BackupPath = payload.BackupPath
	}
	if payload.BackupRetention > 0 {
		cfg.BackupRetention = payload.BackupRetention
	}
	if payload.CheckIntervalMinutes >= 0 {
		cfg.CheckIntervalMinutes = payload.CheckIntervalMinutes
	}
	cfg.ConfirmBeforeSync = payload.ConfirmBeforeSync

	if err := config.Save(cfg); err != nil {
		return nil, err
	}

	if intervalChanged {
		a.scheduler.Start(cfg.CheckIntervalMinutes)
	}

	return cfg, nil
}

// SelectRootFolder opens a native folder selection dialog for the root directory.
func (a *App) SelectRootFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select agent configurations root",
	})
}

// SetRootPath updates the root path in the configuration.
func (a *App) SetRootPath(rootPath string) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	fi, err := os.Stat(absRoot)
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	cfg.RootPath = absRoot
	if err := config.Save(cfg); err != nil {
		return nil, err
	}

	go a.scheduler.TriggerNow(true)
	return cfg, nil
}

// ListProfiles discovers profiles in the configured root directory.
func (a *App) ListProfiles() ([]profiles.Profile, error) {
	cfg, err := config.Load()
	if err != nil || cfg.RootPath == "" {
		return []profiles.Profile{}, nil
	}
	return profiles.Discover(cfg.RootPath)
}

// SelectTargetFolder opens a native folder selection dialog for a target connection.
func (a *App) SelectTargetFolder(profile string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: fmt.Sprintf("Select target directory for profile %s", profile),
	})
}

// ConnectProfile connects a profile to a target directory.
func (a *App) ConnectProfile(profile string, targetPath string) (*InitialLinkResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := config.ValidateConnection(cfg.RootPath, targetPath, cfg.Connections, ""); err != nil {
		return nil, err
	}

	conn := config.Connection{
		ID:         generateID(),
		Profile:    profile,
		TargetPath: targetPath,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	st, _ := state.Load()
	status, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
	if err != nil {
		return nil, err
	}

	var untracked []string
	for _, f := range status.Files {
		if f.Status == syncer.FileUntrackedDiff {
			untracked = append(untracked, f.RelPath)
		}
	}

	if len(untracked) > 0 {
		return &InitialLinkResult{
			Connection:     conn,
			NeedsDecision:  true,
			UntrackedFiles: untracked,
		}, nil
	}

	cfg.Connections = append(cfg.Connections, conn)
	if err := config.Save(cfg); err != nil {
		return nil, err
	}

	go a.scheduler.TriggerNow(true)

	return &InitialLinkResult{
		Connection:     conn,
		NeedsDecision:  false,
		UntrackedFiles: []string{},
	}, nil
}

// ResolveInitialLink applies the user's choice for untracked differences upon linking.
func (a *App) ResolveInitialLink(conn config.Connection, choice string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	st, _ := state.Load()
	status, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
	if err != nil {
		return err
	}

	var untracked []string
	for _, f := range status.Files {
		if f.Status == syncer.FileUntrackedDiff {
			untracked = append(untracked, f.RelPath)
		}
	}

	switch choice {
	case syncer.ResolutionKeepRepo:
		if err := syncer.Apply(cfg.RootPath, conn, cfg.Connections, cfg.BackupPath, untracked, st, cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
			return err
		}
	case syncer.ResolutionKeepLocal:
		if err := syncer.Capture(cfg.RootPath, conn, untracked, st); err != nil {
			return err
		}
	case "decide_later":
		// Left as untracked_diff
	}

	cfg.Connections = append(cfg.Connections, conn)
	if err := config.Save(cfg); err != nil {
		return err
	}

	go a.scheduler.TriggerNow(true)
	return nil
}

// DisconnectProfile removes a connection and cleans up its baseline state.
func (a *App) DisconnectProfile(connectionID string) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	var updated []config.Connection
	for _, c := range cfg.Connections {
		if c.ID != connectionID {
			updated = append(updated, c)
		}
	}
	cfg.Connections = updated

	if err := config.Save(cfg); err != nil {
		return nil, err
	}

	st, err := state.Load()
	if err == nil && st != nil {
		st.RemoveConnection(connectionID)
		_ = st.Save()
	}

	go a.scheduler.TriggerNow(true)
	return cfg, nil
}

// GetStatus computes and returns the current sync plan and status across all connections.
func (a *App) GetStatus() (*orchestrator.SyncPlan, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	return a.orch.PlanSync(cfg, st)
}

// ApplyConnection performs a local download for a specific connection.
func (a *App) ApplyConnection(connectionID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var targetConn *config.Connection
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == connectionID {
			targetConn = &cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return apperr.New(apperr.TargetNotFound).WithParam("connectionId", connectionID)
	}

	st, _ := state.Load()
	status, err := syncer.ComputeStatus(cfg.RootPath, *targetConn, st)
	if err != nil {
		return err
	}

	var filesToApply []string
	for _, f := range status.Files {
		if f.Status == syncer.FileSourceChanged || f.Status == syncer.FileNewInSource || f.Status == syncer.FileTargetMissing {
			filesToApply = append(filesToApply, f.RelPath)
		}
	}

	if len(filesToApply) == 0 {
		return nil
	}

	if err := syncer.Apply(cfg.RootPath, *targetConn, cfg.Connections, cfg.BackupPath, filesToApply, st, cfg.BackupRetention, backup.ReasonApply); err != nil {
		return err
	}

	go a.scheduler.TriggerNow(true)
	return nil
}

// PlanSync calculates the actions needed for the main sync button.
func (a *App) PlanSync() (*orchestrator.SyncPlan, error) {
	return a.GetStatus()
}

// ExecuteSync performs the full synchronization operation.
func (a *App) ExecuteSync(resolutions []orchestrator.ConflictResolution) (*orchestrator.SyncResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, err
	}

	plan, err := a.orch.PlanSync(cfg, st)
	if err != nil {
		return nil, err
	}

	result, err := a.orch.ExecuteSync(cfg, st, plan, resolutions)
	if err != nil {
		return nil, err
	}

	go a.scheduler.TriggerNow(true)
	return result, nil
}

// ListBackups lists available snapshots for a given connection.
func (a *App) ListBackups(connectionID string) ([]backup.BackupInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	var targetConn *config.Connection
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == connectionID {
			targetConn = &cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return []backup.BackupInfo{}, nil
	}

	return backup.ListBackups(cfg.BackupPath, *targetConn, cfg.Connections)
}

// RestoreBackup restores files from a snapshot back to the target directory.
func (a *App) RestoreBackup(connectionID string, backupID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var targetConn *config.Connection
	for i := range cfg.Connections {
		if cfg.Connections[i].ID == connectionID {
			targetConn = &cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return apperr.New(apperr.TargetNotFound).WithParam("connectionId", connectionID)
	}

	st, _ := state.Load()
	if err := backup.RestoreBackup(cfg.BackupPath, *targetConn, cfg.Connections, backupID, st, cfg.BackupRetention); err != nil {
		return err
	}

	go a.scheduler.TriggerNow(true)
	return nil
}

// OpenInExplorer opens the OS file explorer at the specified directory path.
func (a *App) OpenInExplorer(path string) error {
	cleanPath := filepath.Clean(path)
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("explorer", cleanPath).Start()
	case "darwin":
		return exec.Command("open", cleanPath).Start()
	default:
		return exec.Command("xdg-open", cleanPath).Start()
	}
}

// InitRepository initializes a git repository in the configured root directory.
func (a *App) InitRepository() error {
	cfg, err := config.Load()
	if err != nil || cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	if err := gitops.Init(cfg.RootPath); err != nil {
		return err
	}
	go a.scheduler.TriggerNow(true)
	return nil
}

// AddRemote adds an origin remote to the git repository and sets upstream.
func (a *App) AddRemote(url string) error {
	cfg, err := config.Load()
	if err != nil || cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	runner := gitops.NewRunner(cfg.RootPath)
	if err := runner.AddRemote(url); err != nil {
		return err
	}
	if err := runner.PushSetUpstream(); err != nil {
		return err
	}
	go a.scheduler.TriggerNow(true)
	return nil
}

// GetGitInfo returns branch, remote URL and ahead/behind counts for the root repo.
func (a *App) GetGitInfo() (*GitInfoPayload, error) {
	cfg, err := config.Load()
	if err != nil || cfg.RootPath == "" {
		return &GitInfoPayload{
			IsInstalled: gitops.IsInstalled(),
			IsRepo:      false,
		}, nil
	}

	installed := gitops.IsInstalled()
	if !installed {
		return &GitInfoPayload{
			IsInstalled: false,
			IsRepo:      false,
		}, nil
	}

	if !gitops.IsRepo(cfg.RootPath) {
		return &GitInfoPayload{
			IsInstalled: true,
			IsRepo:      false,
		}, nil
	}

	runner := gitops.NewRunner(cfg.RootPath)
	branch, _ := runner.CurrentBranch()
	remoteURL, _ := runner.RemoteURL()
	hasUpstream := runner.HasUpstream()
	ahead, behind, _ := runner.AheadBehind()

	return &GitInfoPayload{
		IsInstalled: true,
		IsRepo:      true,
		Branch:      branch,
		RemoteURL:   remoteURL,
		HasUpstream: hasUpstream,
		Ahead:       ahead,
		Behind:      behind,
	}, nil
}
