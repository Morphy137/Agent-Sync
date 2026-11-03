package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/state"
	"agentsync/internal/subscriptions"
)

// SettingsPayload defines incoming preferences from the settings view.
type SettingsPayload struct {
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	BackupPath           string `json:"backupPath"`
	BackupRetention      int    `json:"backupRetention"`
	CheckIntervalMinutes int    `json:"checkIntervalMinutes"`
	ConfirmBeforeSync    bool   `json:"confirmBeforeSync"`
	DisableUpdateCheck   bool   `json:"disableUpdateCheck"`
	AutoApplyUpdates     bool   `json:"autoApplyUpdates"`
	CloseToTray          bool   `json:"closeToTray"`
	StartWithWindows     bool   `json:"startWithWindows"`
}

// GitInfoPayload exposes current git status, branch, and identity details.
type GitInfoPayload struct {
	IsInstalled bool   `json:"isInstalled"`
	IsRepo      bool   `json:"isRepo"`
	Branch      string `json:"branch"`
	RemoteURL   string `json:"remoteUrl"`
	HasUpstream bool   `json:"hasUpstream"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	UserName    string `json:"userName"`
	UserEmail   string `json:"userEmail"`
}

// PathInfo describes the status and attributes of an inspected path (B.2).
type PathInfo struct {
	Normalized   string `json:"normalized"`
	Exists       bool   `json:"exists"`
	IsDir        bool   `json:"isDir"`
	Writable     bool   `json:"writable"`
	IsInsideRoot bool   `json:"isInsideRoot"`
}

const (
	// backgroundLockAttempts and backgroundLockRetryDelay bound how long a periodic check waits
	// for a momentary operation to release the service lock (about two seconds in total).
	backgroundLockAttempts   = 20
	backgroundLockRetryDelay = 100 * time.Millisecond
)

// Service owns the single in-memory *config.Config and *state.State instances
// and serializes all state and git operations behind a global mutex (C3).
type Service struct {
	mu           serviceLock
	lastSnapshot snapshotHolder
	cfg          *config.Config
	st           *state.State
	orch         *orchestrator.Orchestrator
	emitFunc     func(eventName string, data any)
	dialogFunc   func(title, defaultDir string) (string, error)
	fileDialog   func(title, defaultDir string) (string, error)
	filesDialog  func(title, defaultDir string) ([]string, error)
	subUpdates   map[string]subscriptions.UpdateInfo
}

// New creates a new Service, loading config and state into memory. The Service coordinates with
// other AgentSync processes (GUI, CLI, MCP server) through a file lock in the config dir.
func New(emitFunc func(eventName string, data any)) *Service {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	st, err := state.Load()
	if err != nil || st == nil {
		st = state.NewState()
	}

	svc := &Service{
		cfg:        cfg,
		st:         st,
		orch:       orchestrator.New(),
		subUpdates: make(map[string]subscriptions.UpdateInfo),
		emitFunc:   emitFunc,
	}
	svc.enableSharedMode()
	svc.migrateLegacyData()
	return svc
}

// migrateLegacyData converts data saved by earlier versions (folder connections) once, under the
// service lock so a second process starting at the same time does not repeat it. A failure leaves
// the old data in place, to be retried the next time the app starts.
func (s *Service) migrateLegacyData() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.migrateLegacyConnectionsLocked()
}

// NewWithInstances creates a Service with explicitly supplied config and state (for unit testing).
func NewWithInstances(cfg *config.Config, st *state.State, orch *orchestrator.Orchestrator, emitFunc func(eventName string, data any)) *Service {
	return &Service{
		cfg:        cfg,
		st:         st,
		orch:       orch,
		subUpdates: make(map[string]subscriptions.UpdateInfo),
		emitFunc:   emitFunc,
	}
}

// Orchestrator returns the underlying orchestrator (e.g. for testing hooks).
func (s *Service) Orchestrator() *orchestrator.Orchestrator {
	return s.orch
}

// ReloadFromDisk reloads config and state from disk under lock (useful when AGENTSYNC_CONFIG_DIR changes in tests).
func (s *Service) ReloadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := state.Load()
	if err != nil {
		return err
	}
	s.cfg = cfg
	s.st = st
	_ = s.migrateLegacyConnectionsLocked()
	return nil
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}
	cp := *cfg
	cp.Locations = make([]config.Location, len(cfg.Locations))
	copy(cp.Locations, cfg.Locations)
	cp.Subscriptions = make([]config.Subscription, len(cfg.Subscriptions))
	copy(cp.Subscriptions, cfg.Subscriptions)
	return &cp
}

func (s *Service) emitPlanLocked(plan *orchestrator.SyncPlan) {
	if s.emitFunc != nil && plan != nil {
		s.emitFunc("status:changed", plan)
	}
}

// GetConfig returns a copy of the stored configuration.
func (s *Service) GetConfig() (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneConfig(s.cfg), nil
}

// UpdateSettings updates user preferences under lock. Returns updated config and whether checkInterval changed.
func (s *Service) UpdateSettings(payload SettingsPayload) (*config.Config, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intervalChanged := s.cfg.CheckIntervalMinutes != payload.CheckIntervalMinutes

	if payload.Language != "" {
		s.cfg.Language = payload.Language
	}
	if payload.Theme != "" {
		s.cfg.Theme = payload.Theme
	}
	if payload.BackupPath != "" {
		s.cfg.BackupPath = payload.BackupPath
	}
	if payload.BackupRetention > 0 {
		s.cfg.BackupRetention = payload.BackupRetention
	}
	if payload.CheckIntervalMinutes >= 0 {
		s.cfg.CheckIntervalMinutes = payload.CheckIntervalMinutes
	}
	s.cfg.ConfirmBeforeSync = payload.ConfirmBeforeSync
	s.cfg.DisableUpdateCheck = payload.DisableUpdateCheck
	s.cfg.AutoApplyUpdates = payload.AutoApplyUpdates
	s.cfg.CloseToTray = payload.CloseToTray
	s.cfg.StartWithWindows = payload.StartWithWindows

	if err := config.Save(s.cfg); err != nil {
		return nil, false, err
	}

	return cloneConfig(s.cfg), intervalChanged, nil
}

// SetStartWithWindows records whether the Windows startup entry is wanted.
func (s *Service) SetStartWithWindows(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.StartWithWindows = enabled
	return config.Save(s.cfg)
}

// SetTutorialCompleted stores whether the first-run tutorial should stay hidden.
func (s *Service) SetTutorialCompleted(done bool) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.TutorialCompleted = done
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}
	return cloneConfig(s.cfg), nil
}

// SetRootPath validates and sets the root path in the configuration.
func (s *Service) SetRootPath(rootPath string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	fi, err := os.Stat(absRoot)
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	s.cfg.RootPath = absRoot
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return cloneConfig(s.cfg), nil
}

// GetStatus computes and returns the current local sync plan (without network fetch, H6).
func (s *Service) GetStatus() (*orchestrator.SyncPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncOrchestratorRulesLocked()
	plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false)
	if err != nil {
		return nil, err
	}
	s.annotatePlanLocked(&plan.RulesPending, &plan.RulesConflicts)
	return plan, nil
}

// PlanSync calculates the synchronization plan with remote fetch.
func (s *Service) PlanSync() (*orchestrator.SyncPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncOrchestratorRulesLocked()
	plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, true)
	if err != nil {
		return nil, err
	}
	s.annotatePlanLocked(&plan.RulesPending, &plan.RulesConflicts)
	nowStr := config.FormatRFC3339(time.Now())
	s.st.SetTimestamps("", nowStr)
	_ = s.st.Save()
	plan.LastCheckAt = nowStr
	s.emitPlanLocked(plan)
	return plan, nil
}

// CheckRemote performs a manual remote check (fetch + PlanSync) and emits the updated SyncPlan (H6, H7).
func (s *Service) CheckRemote() (*orchestrator.SyncPlan, error) {
	return s.PlanSync()
}

// tryLockBriefly retries TryLock for a short while, so a momentary read does not cancel a periodic
// check, and gives up while a long operation (such as a sync) holds the lock.
func (s *Service) tryLockBriefly() bool {
	for attempt := 0; attempt < backgroundLockAttempts; attempt++ {
		if s.mu.TryLock() {
			return true
		}
		time.Sleep(backgroundLockRetryDelay)
	}
	return false
}

// snapshotRemotesForCheck returns what a periodic check needs to fetch without holding the lock.
func (s *Service) snapshotRemotesForCheck() (rootPath string, subs []config.Subscription, ok bool) {
	if !s.tryLockBriefly() {
		return "", nil, false
	}
	defer s.mu.Unlock()
	if s.cfg.RootPath == "" {
		return "", nil, false
	}
	return s.cfg.RootPath, append([]config.Subscription(nil), s.cfg.Subscriptions...), true
}

// TryBackgroundCheck runs the periodic remote check for the scheduler (C3). The fetches run without
// the service lock, so reads stay responsive while the network is slow; the lock is taken only to
// snapshot the remotes and to apply the results (auto-updates and, when enabled, automatic rule
// application). It skips the check while a long operation holds the lock, and emits the full
// SyncPlan when it completes (H5).
func (s *Service) TryBackgroundCheck() (*orchestrator.SyncPlan, bool) {
	rootPath, subs, ok := s.snapshotRemotesForCheck()
	if !ok {
		return nil, false
	}
	rootFetchErr := orchestrator.FetchRemote(rootPath)
	subscriptionFetches := fetchSubscriptions(subs)

	if !s.tryLockBriefly() {
		return nil, false
	}
	defer s.mu.Unlock()

	// The fetched results belong to the root that was configured when they started.
	if s.cfg.RootPath != rootPath {
		return nil, false
	}

	var applied autoApplied
	s.checkFetchedSubscriptionsLocked(subscriptionFetches, &applied)
	s.autoApplySubscriptionRulesLocked(&applied)
	s.syncOrchestratorRulesLocked()
	plan, err := s.orch.PlanSyncAfterFetch(s.cfg, s.st, rootFetchErr)
	if err != nil {
		return nil, false
	}
	s.annotatePlanLocked(&plan.RulesPending, &plan.RulesConflicts)
	s.emitRulesChangedLocked()
	s.emitAutoAppliedLocked(&applied)

	nowStr := config.FormatRFC3339(time.Now())
	s.st.SetTimestamps("", nowStr)
	_ = s.st.Save()
	plan.LastCheckAt = nowStr

	s.emitPlanLocked(plan)
	return plan, true
}

// ExecuteSync performs the full synchronization operation under lock, verifying the plan fingerprint (H1, C5).
func (s *Service) ExecuteSync(fingerprint string, resolutions []orchestrator.ConflictResolution) (*orchestrator.SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncOrchestratorRulesLocked()
	res, err := s.orch.ExecuteSync(s.cfg, s.st, fingerprint, resolutions)
	if err != nil {
		return nil, err
	}

	// Rules run after pull so they distribute the freshest repo content; captures are committed and pushed.
	if err := s.syncRulesAfterGitLocked(res); err != nil {
		return nil, err
	}

	s.emitStatusLocked()
	s.emitRulesChangedLocked()
	return res, nil
}

// InitRepository initializes a git repository in the configured root directory under lock.
func (s *Service) InitRepository() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	if err := gitops.Init(s.cfg.RootPath); err != nil {
		return err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}
	return nil
}

// AddRemote adds an origin remote to the git repository and sets upstream under lock.
func (s *Service) AddRemote(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	runner := gitops.NewRunner(s.cfg.RootPath)
	if err := runner.AddRemote(url); err != nil {
		return err
	}
	if err := runner.PushSetUpstream(); err != nil {
		return err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}
	return nil
}

// SetGitIdentity sets local git user.name and user.email in the root repository (C6).
func (s *Service) SetGitIdentity(name string, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return apperr.New(apperr.RootNotFound)
	}
	if !gitops.IsRepo(s.cfg.RootPath) {
		return apperr.New(apperr.RootNotRepo)
	}
	runner := gitops.NewRunner(s.cfg.RootPath)
	return runner.SetLocalIdentity(name, email)
}

// GetGitInfo returns branch, remote URL, ahead/behind counts, and effective identity for the root repo under lock.
func (s *Service) GetGitInfo() (*GitInfoPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	installed := gitops.IsInstalled()
	if s.cfg.RootPath == "" || !installed {
		return &GitInfoPayload{
			IsInstalled: installed,
			IsRepo:      false,
		}, nil
	}

	if !gitops.IsRepo(s.cfg.RootPath) {
		return &GitInfoPayload{
			IsInstalled: true,
			IsRepo:      false,
		}, nil
	}

	runner := gitops.NewRunner(s.cfg.RootPath)
	branch, _ := runner.CurrentBranch()
	remoteURL, _ := runner.RemoteURL()
	hasUpstream := runner.HasUpstream()
	ahead, behind, _ := runner.AheadBehind()
	userName, userEmail := runner.GetIdentity()

	return &GitInfoPayload{
		IsInstalled: true,
		IsRepo:      true,
		Branch:      branch,
		RemoteURL:   remoteURL,
		HasUpstream: hasUpstream,
		Ahead:       ahead,
		Behind:      behind,
		UserName:    userName,
		UserEmail:   userEmail,
	}, nil
}

var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func isValidWindowsFolderName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed != name {
		return false
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for _, ch := range `<>:"/\|?*` {
		if strings.ContainsRune(name, ch) {
			return false
		}
	}
	base := strings.ToUpper(name)
	if idx := strings.IndexByte(base, '.'); idx != -1 {
		base = base[:idx]
	}
	return !windowsReservedNames[base]
}

func expandPath(path string) string {
	p := strings.TrimSpace(path)
	if strings.HasPrefix(p, "~") {
		home := os.Getenv("AGENTSYNC_HOME_OVERRIDE")
		if home == "" {
			home = os.Getenv("USERPROFILE")
			if home == "" {
				home = os.Getenv("HOME")
			}
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return os.ExpandEnv(p)
}

// SetDialogFunc configures the folder dialog handler (e.g. Wails runtime OpenDirectoryDialog).
func (s *Service) SetDialogFunc(fn func(title, defaultDir string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogFunc = fn
}

// SelectFolder opens a native folder selection dialog (B.2).
func (s *Service) SelectFolder(title, defaultDir string) (string, error) {
	s.mu.Lock()
	fn := s.dialogFunc
	s.mu.Unlock()

	if fn != nil {
		return fn(title, resolveDialogStartDir(defaultDir))
	}
	return "", nil
}

// SetFileDialogFunc configures the file selection dialog handler.
func (s *Service) SetFileDialogFunc(fn func(title, defaultDir string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fileDialog = fn
}

// SelectFile opens a native file selection dialog.
func (s *Service) SelectFile(title, defaultDir string) (string, error) {
	s.mu.Lock()
	fn := s.fileDialog
	s.mu.Unlock()

	if fn != nil {
		return fn(title, resolveDialogStartDir(defaultDir))
	}
	return "", nil
}

// SetFilesDialogFunc configures the multiple-file selection dialog handler.
func (s *Service) SetFilesDialogFunc(fn func(title, defaultDir string) ([]string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filesDialog = fn
}

// SelectFiles opens a native dialog that lets the user pick several files at once.
func (s *Service) SelectFiles(title, defaultDir string) ([]string, error) {
	s.mu.Lock()
	fn := s.filesDialog
	s.mu.Unlock()

	if fn == nil {
		return []string{}, nil
	}
	picked, err := fn(title, resolveDialogStartDir(defaultDir))
	if picked == nil {
		picked = []string{}
	}
	return picked, err
}

// resolveDialogStartDir returns the nearest existing directory for the given path, or "" when none applies.
// Native dialogs fail when the default directory does not exist, so partial or typed paths are walked up.
func resolveDialogStartDir(rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return ""
	}
	candidate := filepath.Clean(expandPath(rawPath))
	if !filepath.IsAbs(candidate) {
		return ""
	}
	for {
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return ""
		}
		candidate = parent
	}
}

// InspectPath inspects path properties without side-effects (B.2).
func (s *Service) InspectPath(rawPath string) PathInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	expanded := expandPath(rawPath)
	cleanPath := filepath.Clean(expanded)
	if !filepath.IsAbs(cleanPath) {
		cleanPath, _ = filepath.Abs(cleanPath)
	}

	fi, err := os.Stat(cleanPath)
	exists := err == nil
	isDir := exists && fi.IsDir()

	writable := false
	if isDir {
		probePath := filepath.Join(cleanPath, fmt.Sprintf(".agentsync_probe_%d.tmp", time.Now().UnixNano()))
		if err := os.WriteFile(probePath, []byte(""), 0o600); err == nil {
			writable = true
			_ = os.Remove(probePath)
		}
	} else if !exists {
		// Probe parent directory
		parent := filepath.Dir(cleanPath)
		for parent != "" && parent != filepath.Dir(parent) {
			if pfi, perr := os.Stat(parent); perr == nil && pfi.IsDir() {
				probePath := filepath.Join(parent, fmt.Sprintf(".agentsync_probe_%d.tmp", time.Now().UnixNano()))
				if err := os.WriteFile(probePath, []byte(""), 0o600); err == nil {
					writable = true
					_ = os.Remove(probePath)
				}
				break
			}
			parent = filepath.Dir(parent)
		}
	}

	isInsideRoot := false
	if s.cfg.RootPath != "" && exists {
		isInsideRoot = config.IsSubdirectory(cleanPath, s.cfg.RootPath)
	}

	return PathInfo{
		Normalized:   cleanPath,
		Exists:       exists,
		IsDir:        isDir,
		Writable:     writable,
		IsInsideRoot: isInsideRoot,
	}
}

// CreateFolder creates a directory on disk after validating Windows folder naming rules (B.2).
func (s *Service) CreateFolder(rawPath string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	expanded := expandPath(rawPath)
	cleanPath := filepath.Clean(expanded)
	if !filepath.IsAbs(cleanPath) {
		return "", apperr.New(apperr.PathNotAbsolute).WithParam("path", rawPath)
	}

	vol := filepath.VolumeName(cleanPath)
	rest := strings.TrimPrefix(cleanPath, vol)
	parts := strings.Split(rest, string(filepath.Separator))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if !isValidWindowsFolderName(part) {
			return "", apperr.New(apperr.InvalidFolderName).WithParam("name", part)
		}
	}

	if fi, err := os.Stat(cleanPath); err == nil {
		if fi.IsDir() {
			return "", apperr.New(apperr.FolderExists).WithParam("path", cleanPath)
		}
		return "", apperr.New(apperr.FolderExists).WithParam("path", cleanPath)
	}

	if err := os.MkdirAll(cleanPath, fsops.DefaultDirPerms); err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}

	return cleanPath, nil
}

// CreateRoot creates a directory and sets it as the repository root (B.2).
func (s *Service) CreateRoot(parentDir, name string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if parentDir == "" || !filepath.IsAbs(parentDir) {
		return nil, apperr.New(apperr.PathNotAbsolute).WithParam("path", parentDir)
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" || !isValidWindowsFolderName(trimmedName) {
		return nil, apperr.New(apperr.InvalidFolderName).WithParam("name", name)
	}

	fi, err := os.Stat(parentDir)
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.TargetNotFound).WithParam("path", parentDir)
	}

	rootPath := filepath.Join(parentDir, trimmedName)
	if _, err := os.Stat(rootPath); err == nil {
		return nil, apperr.New(apperr.FolderExists).WithParam("path", rootPath)
	}

	if err := os.MkdirAll(rootPath, fsops.DefaultDirPerms); err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	s.cfg.RootPath = rootPath
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return cloneConfig(s.cfg), nil
}

// CloneRepository clones a remote repository into a target folder and sets it as root (B.2).
func (s *Service) CloneRepository(url, parentDir, name string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(url) == "" {
		return nil, apperr.New(apperr.CloneFailed).WithParam("reason", "empty url")
	}

	if parentDir == "" || !filepath.IsAbs(parentDir) {
		return nil, apperr.New(apperr.PathNotAbsolute).WithParam("path", parentDir)
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" || !isValidWindowsFolderName(trimmedName) {
		return nil, apperr.New(apperr.InvalidFolderName).WithParam("name", name)
	}

	fi, err := os.Stat(parentDir)
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.TargetNotFound).WithParam("path", parentDir)
	}

	rootPath := filepath.Join(parentDir, trimmedName)
	if _, err := os.Stat(rootPath); err == nil {
		return nil, apperr.New(apperr.FolderExists).WithParam("path", rootPath)
	}

	if err := gitops.Clone(url, rootPath); err != nil {
		return nil, err
	}

	s.cfg.RootPath = rootPath
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return cloneConfig(s.cfg), nil
}
