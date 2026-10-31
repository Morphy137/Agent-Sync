package main

import (
	"context"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/profiles"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/scheduler"
	"agentsync/internal/service"
	"agentsync/internal/subscriptions"
)

// SettingsPayload is an alias for service.SettingsPayload for Wails bindings.
type SettingsPayload = service.SettingsPayload

// InitialLinkResult is an alias for service.InitialLinkResult for Wails bindings.
type InitialLinkResult = service.InitialLinkResult

// GitInfoPayload is an alias for service.GitInfoPayload for Wails bindings.
type GitInfoPayload = service.GitInfoPayload

// App struct manages application lifecycle and delegates all business logic to Service.
type App struct {
	ctx       context.Context
	svc       *service.Service
	scheduler *scheduler.Scheduler
}

// NewApp creates a new App application struct.
func NewApp() *App {
	app := &App{}
	app.svc = service.New(func(eventName string, data any) {
		if app.ctx != nil {
			runtime.EventsEmit(app.ctx, eventName, data)
		}
	})
	app.scheduler = scheduler.NewScheduler(func() bool {
		_, ok := app.svc.TryBackgroundCheck()
		return ok
	})
	return app
}

// startup is called when Wails initializes.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.svc.SetDialogFunc(func(title, defaultDir string) (string, error) {
		return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
			Title:                title,
			DefaultDirectory:     defaultDir,
			CanCreateDirectories: true,
		})
	})
	a.svc.SetFileDialogFunc(func(title, defaultDir string) (string, error) {
		return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: defaultDir,
		})
	})
	// Reflect changes made by agentsync-cli or its MCP server while the window is open.
	go a.svc.WatchExternalChanges(ctx, service.DefaultExternalWatchInterval)
	cfg, err := a.svc.GetConfig()
	if err == nil && cfg != nil {
		a.scheduler.Start(cfg.CheckIntervalMinutes)
		go a.scheduler.TriggerNow(true)
	}
}

// GetConfig returns the stored configuration.
func (a *App) GetConfig() (*config.Config, error) {
	return a.svc.GetConfig()
}

// UpdateSettings updates user settings and reconfigures the scheduler if needed.
func (a *App) UpdateSettings(payload SettingsPayload) (*config.Config, error) {
	cfg, intervalChanged, err := a.svc.UpdateSettings(payload)
	if err != nil {
		return nil, err
	}
	if intervalChanged {
		a.scheduler.Start(cfg.CheckIntervalMinutes)
	}
	return cfg, nil
}

// SelectFolder opens a generic native folder selection dialog (B.2).
func (a *App) SelectFolder(title string, defaultDir string) (string, error) {
	return a.svc.SelectFolder(title, defaultDir)
}

// SelectRootFolder opens a native folder selection dialog with the translated title passed from the frontend (M3).
func (a *App) SelectRootFolder(title string) (string, error) {
	return a.SelectFolder(title, "")
}

// SetRootPath updates the root path in the configuration.
func (a *App) SetRootPath(rootPath string) (*config.Config, error) {
	return a.svc.SetRootPath(rootPath)
}

// ListProfiles discovers profiles in the configured root directory.
func (a *App) ListProfiles() ([]profiles.Profile, error) {
	return a.svc.ListProfiles()
}

// SelectTargetFolder opens a native folder selection dialog with the translated title passed from the frontend (M3).
func (a *App) SelectTargetFolder(title string) (string, error) {
	return a.SelectFolder(title, "")
}

// InspectPath inspects path properties without side-effects (B.2).
func (a *App) InspectPath(path string) service.PathInfo {
	return a.svc.InspectPath(path)
}

// CreateFolder creates a directory on disk after validation (B.2).
func (a *App) CreateFolder(path string) (string, error) {
	return a.svc.CreateFolder(path)
}

// CreateProfile creates a profile directory with a .gitkeep file inside the configured root (B.2).
func (a *App) CreateProfile(name string) (profiles.Profile, error) {
	return a.svc.CreateProfile(name)
}

// DetectAgentFolders returns detected agent configurations on this system (B.2).
func (a *App) DetectAgentFolders() []service.AgentFolderSuggestion {
	return a.svc.DetectAgentFolders()
}

// ConnectProfile connects a profile to a target directory.
func (a *App) ConnectProfile(profile string, targetPath string) (*InitialLinkResult, error) {
	return a.svc.ConnectProfile(profile, targetPath)
}

// ChangeTarget changes the target directory of an existing connection.
func (a *App) ChangeTarget(connectionID string, targetPath string) (*InitialLinkResult, error) {
	return a.svc.ChangeTarget(connectionID, targetPath)
}

// ResolveInitialLink applies the user's choice for a pending initial link by pendingID (M2).
func (a *App) ResolveInitialLink(pendingID string, choice string) error {
	return a.svc.ResolveInitialLink(pendingID, choice)
}

// CancelPendingLink cancels and cleans up a pending link (R3).
func (a *App) CancelPendingLink(pendingID string) error {
	return a.svc.CancelPendingLink(pendingID)
}

// DisconnectProfile removes a connection and cleans up its baseline state.
func (a *App) DisconnectProfile(connectionID string) (*config.Config, error) {
	return a.svc.DisconnectProfile(connectionID)
}

// GetStatus computes and returns the current local sync plan (without network fetch, H6).
func (a *App) GetStatus() (*orchestrator.SyncPlan, error) {
	return a.svc.GetStatus()
}

// CheckRemote performs a remote fetch and returns the updated SyncPlan (H6, H7).
func (a *App) CheckRemote() (*orchestrator.SyncPlan, error) {
	return a.svc.CheckRemote()
}

// ApplyConnection performs a local download for a specific connection.
func (a *App) ApplyConnection(connectionID string) error {
	return a.svc.ApplyConnection(connectionID)
}

// PlanSync calculates the actions needed for the main sync button (with fetch).
func (a *App) PlanSync() (*orchestrator.SyncPlan, error) {
	return a.svc.PlanSync()
}

// ExecuteSync performs the full synchronization operation, validating the plan fingerprint (H1).
func (a *App) ExecuteSync(fingerprint string, resolutions []orchestrator.ConflictResolution) (*orchestrator.SyncResult, error) {
	return a.svc.ExecuteSync(fingerprint, resolutions)
}

// ListBackups lists available snapshots for a given connection.
func (a *App) ListBackups(connectionID string) ([]backup.BackupInfo, error) {
	return a.svc.ListBackups(connectionID)
}

// RestoreBackup restores files from a snapshot back to the target directory.
func (a *App) RestoreBackup(connectionID string, backupID string) error {
	return a.svc.RestoreBackup(connectionID, backupID)
}

// SelectFile opens a native file selection dialog.
func (a *App) SelectFile(title string, defaultDir string) (string, error) {
	return a.svc.SelectFile(title, defaultDir)
}

// ---- Sync rules ----

// GetRulesReport returns every rule with status, collisions and unmapped locations.
func (a *App) GetRulesReport() (*rulesync.Report, error) { return a.svc.GetRulesReport() }

// SaveRule creates or updates a rule in scope "repo" (shared through git) or "local" (this PC only).
func (a *App) SaveRule(rule rules.Rule, scope string) (*service.SaveRuleResult, error) {
	return a.svc.SaveRule(rule, scope)
}

// DeleteRule removes a rule; files already written stay in place.
func (a *App) DeleteRule(id string) error { return a.svc.DeleteRule(id) }

// SyncRules applies every non-conflicting rule change with backups.
func (a *App) SyncRules() (*rulesync.ApplyResult, error) { return a.svc.SyncRules() }

// ResolveRuleFile applies the user decision for a conflicting or drifted file.
func (a *App) ResolveRuleFile(res rulesync.Resolution) error { return a.svc.ResolveRuleFile(res) }

// GetRuleDiff returns source vs destination content of one rule file.
func (a *App) GetRuleDiff(ruleID, destID, relPath string) (*rulesync.Diff, error) {
	return a.svc.GetRuleDiff(ruleID, destID, relPath)
}

// ListRepoTree lists repository entries with detected kinds for picking rule sources.
func (a *App) ListRepoTree() ([]service.SourceTreeEntry, error) { return a.svc.ListRepoTree() }

// GetGitignore returns the repository .gitignore and the recommended patterns it lacks.
func (a *App) GetGitignore() (*service.GitignorePayload, error) { return a.svc.GetGitignore() }

// SaveGitignore overwrites the repository .gitignore.
func (a *App) SaveGitignore(content string) (*service.GitignorePayload, error) {
	return a.svc.SaveGitignore(content)
}

// AddRecommendedGitignore appends the missing recommended exclusions.
func (a *App) AddRecommendedGitignore() (*service.GitignorePayload, error) {
	return a.svc.AddRecommendedGitignore()
}

// ExcludeRepoPath adds a repository file or folder to .gitignore.
func (a *App) ExcludeRepoPath(path string) (*service.GitignorePayload, error) {
	return a.svc.ExcludeRepoPath(path)
}

// ImportIntoRepo copies a local file/folder into the repository and returns its repo path.
func (a *App) ImportIntoRepo(sourcePath, repoRelDir string) (string, error) {
	return a.svc.ImportIntoRepo(sourcePath, repoRelDir)
}

// DetectSource reports the detected content kind of a source.
func (a *App) DetectSource(src rules.Source) (*recognize.SourceInfo, error) {
	return a.svc.DetectSource(src)
}

// SuggestPlacement proposes destination path/mode/transform for a source in a location.
func (a *App) SuggestPlacement(src rules.Source, alias string) (*recognize.Suggestion, error) {
	return a.svc.SuggestPlacement(src, alias)
}

// GetRecognizersPath returns the user recognizers.json path (created from defaults if missing).
func (a *App) GetRecognizersPath() (string, error) { return a.svc.GetRecognizersPath() }

// ---- Locations ----

// ListLocations returns the location aliases known on this PC.
func (a *App) ListLocations() ([]service.LocationInfo, error) { return a.svc.ListLocations() }

// SaveLocation maps an alias to a folder on this PC.
func (a *App) SaveLocation(alias, path string) (*config.Config, error) {
	return a.svc.SaveLocation(alias, path)
}

// RemoveLocation removes an alias mapping on this PC.
func (a *App) RemoveLocation(alias string) (*config.Config, error) {
	return a.svc.RemoveLocation(alias)
}

// ---- Subscriptions ----

// ListSubscriptions returns subscriptions with their last update check.
func (a *App) ListSubscriptions() ([]service.SubscriptionInfo, error) {
	return a.svc.ListSubscriptions()
}

// AddSubscription clones a remote repository as a read-only source.
func (a *App) AddSubscription(url, name string) (*config.Subscription, error) {
	return a.svc.AddSubscription(url, name)
}

// RemoveSubscription deletes a subscription and its rules.
func (a *App) RemoveSubscription(id string) error { return a.svc.RemoveSubscription(id) }

// CheckSubscription fetches and reports incoming changes.
func (a *App) CheckSubscription(id string) (*subscriptions.UpdateInfo, error) {
	return a.svc.CheckSubscription(id)
}

// UpdateSubscription pulls remote changes and applies the subscription rules.
func (a *App) UpdateSubscription(id string) (*service.SubscriptionUpdateResult, error) {
	return a.svc.UpdateSubscription(id)
}

// SetSubscriptionAutoUpdate toggles automatic updates during periodic checks.
func (a *App) SetSubscriptionAutoUpdate(id string, enabled bool) (*config.Config, error) {
	return a.svc.SetSubscriptionAutoUpdate(id, enabled)
}

// GetSubscriptionTree lists subscription entries with detected kinds.
func (a *App) GetSubscriptionTree(id string) ([]service.SourceTreeEntry, error) {
	return a.svc.GetSubscriptionTree(id)
}

// GetSubscriptionIncomingDiff compares a subscription file between local clone and remote.
func (a *App) GetSubscriptionIncomingDiff(id, relPath string) (*rulesync.Diff, error) {
	return a.svc.GetSubscriptionIncomingDiff(id, relPath)
}

// ---- Repository remote ----

// ConnectRemote attaches the repository root to a remote URL without losing local files.
func (a *App) ConnectRemote(url string) (*gitops.ConnectRemoteResult, error) {
	return a.svc.ConnectRemote(url)
}

// GetRepoFileDiff compares a repo file ("working": HEAD vs disk, "incoming": HEAD vs upstream).
func (a *App) GetRepoFileDiff(relPath, mode string) (*rulesync.Diff, error) {
	return a.svc.GetRepoFileDiff(relPath, mode)
}

// DiscardRepoFile reverts a local repo change (backup first).
func (a *App) DiscardRepoFile(relPath string) error { return a.svc.DiscardRepoFile(relPath) }

// OpenPath opens a file with its default application or a folder in the file explorer.
func (a *App) OpenPath(path string) error { return a.OpenInExplorer(path) }

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
	return a.svc.InitRepository()
}

// AddRemote adds an origin remote to the git repository and sets upstream.
func (a *App) AddRemote(url string) error {
	return a.svc.AddRemote(url)
}

// SetGitIdentity sets local user.name and user.email in the repository (C6).
func (a *App) SetGitIdentity(name string, email string) error {
	return a.svc.SetGitIdentity(name, email)
}

// GetGitInfo returns branch, remote URL, ahead/behind counts, and identity for the root repo.
func (a *App) GetGitInfo() (*GitInfoPayload, error) {
	return a.svc.GetGitInfo()
}

// ListTargetEntries lists files and folders inside a destination folder (B.2).
func (a *App) ListTargetEntries(path string, maxDepth int) []service.TargetEntry {
	return a.svc.ListTargetEntries(path, maxDepth)
}

// BeginConnection prepares and returns a connection draft (B.2).
func (a *App) BeginConnection(req service.ConnectionDraftRequest) (*service.ConnectionDraft, error) {
	return a.svc.BeginConnection(req)
}

// CommitConnection executes and saves a connection draft (B.2).
func (a *App) CommitConnection(draftID string, resolutions []orchestrator.ConflictResolution) error {
	return a.svc.CommitConnection(draftID, resolutions)
}

// CancelConnection discards a connection draft (B.2).
func (a *App) CancelConnection(draftID string) error {
	return a.svc.CancelConnection(draftID)
}

// CreateRoot creates a directory and sets it as the repository root (B.2).
func (a *App) CreateRoot(parentDir string, name string) (*config.Config, error) {
	return a.svc.CreateRoot(parentDir, name)
}

// CloneRepository clones a remote repository into a target folder and sets it as root (B.2).
func (a *App) CloneRepository(url string, parentDir string, name string) (*config.Config, error) {
	return a.svc.CloneRepository(url, parentDir, name)
}
