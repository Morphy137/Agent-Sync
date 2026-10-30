package service

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/profiles"
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
	PendingID      string            `json:"pendingId"`
	Connection     config.Connection `json:"connection"`
	NeedsDecision  bool              `json:"needsDecision"`
	UntrackedFiles []string          `json:"untrackedFiles"`
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

const (
	PendingLinkTTL = 15 * time.Minute
)

// PendingLink stores an in-flight connection awaiting user resolution (R2, R3).
type PendingLink struct {
	Connection           config.Connection
	ReplacesConnectionID string
	CreatedAt            time.Time
}

// Service owns the single in-memory *config.Config and *state.State instances
// and serializes all state and git operations behind a global mutex (C3).
type Service struct {
	mu           sync.Mutex
	cfg          *config.Config
	st           *state.State
	orch         *orchestrator.Orchestrator
	pendingLinks map[string]PendingLink
	emitFunc     func(eventName string, data any)
}

// New creates a new Service, loading config and state once into memory.
func New(emitFunc func(eventName string, data any)) *Service {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	st, err := state.Load()
	if err != nil || st == nil {
		st = state.NewState()
	}

	return &Service{
		cfg:          cfg,
		st:           st,
		orch:         orchestrator.New(),
		pendingLinks: make(map[string]PendingLink),
		emitFunc:     emitFunc,
	}
}

func (s *Service) cleanExpiredPendingLinksLocked() {
	now := time.Now()
	for id, p := range s.pendingLinks {
		if now.Sub(p.CreatedAt) > PendingLinkTTL {
			delete(s.pendingLinks, id)
		}
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
	s.pendingLinks = make(map[string]PendingLink)
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
	cp.Connections = make([]config.Connection, len(cfg.Connections))
	copy(cp.Connections, cfg.Connections)
	return &cp
}

func (s *Service) reconcileAllLocked() {
	if s.cfg.RootPath == "" || s.st == nil {
		return
	}
	mutated := false
	for _, conn := range s.cfg.Connections {
		if syncer.ReconcileBaselines(s.cfg.RootPath, conn, s.st) {
			mutated = true
		}
	}
	if mutated {
		_ = s.st.Save()
	}
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

	if err := config.Save(s.cfg); err != nil {
		return nil, false, err
	}

	return cloneConfig(s.cfg), intervalChanged, nil
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

	s.reconcileAllLocked()
	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return cloneConfig(s.cfg), nil
}

// ListProfiles discovers profiles in the configured root directory.
func (s *Service) ListProfiles() ([]profiles.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return []profiles.Profile{}, nil
	}
	return profiles.Discover(s.cfg.RootPath)
}

// ConnectProfile connects a profile to a target directory, validating M1 and storing pending links by ID (M2).
func (s *Service) ConnectProfile(profile string, targetPath string) (*InitialLinkResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}

	// M1: Validate that the profile exists in RootPath
	discovered, err := profiles.Discover(s.cfg.RootPath)
	if err != nil {
		return nil, err
	}
	foundProfile := false
	for _, p := range discovered {
		if p.Name == profile {
			foundProfile = true
			break
		}
	}
	if !foundProfile {
		return nil, apperr.New(apperr.ProfileNotFound).WithParam("profile", profile)
	}

	// Validate that the profile is not already connected
	for _, c := range s.cfg.Connections {
		if strings.EqualFold(c.Profile, profile) {
			return nil, apperr.New(apperr.ProfileAlreadyConnected).WithParam("profile", profile)
		}
	}

	if err := config.ValidateConnection(s.cfg.RootPath, targetPath, s.cfg.Connections, ""); err != nil {
		return nil, err
	}

	absTarget, _ := filepath.Abs(targetPath)
	conn := config.Connection{
		ID:         generateID(),
		Profile:    profile,
		TargetPath: absTarget,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	status, err := syncer.ComputeStatus(s.cfg.RootPath, conn, s.st)
	if err != nil {
		return nil, err
	}

	var untracked []string
	var toDownload []string
	for _, f := range status.Files {
		switch f.Status {
		case syncer.FileUntrackedDiff:
			untracked = append(untracked, f.RelPath)
		case syncer.FileNewInSource, syncer.FileSourceChanged, syncer.FileTargetMissing:
			toDownload = append(toDownload, f.RelPath)
		}
	}

	if len(untracked) > 0 {
		s.cleanExpiredPendingLinksLocked()
		s.pendingLinks[conn.ID] = PendingLink{
			Connection: conn,
			CreatedAt:  time.Now(),
		}
		return &InitialLinkResult{
			PendingID:      conn.ID,
			Connection:     conn,
			NeedsDecision:  true,
			UntrackedFiles: untracked,
		}, nil
	}

	// If no untracked diffs exist, apply any new_in_source files immediately and reconcile baselines
	if len(toDownload) > 0 {
		allConns := append(append([]config.Connection{}, s.cfg.Connections...), conn)
		if _, err := syncer.Apply(s.cfg.RootPath, conn, allConns, s.cfg.BackupPath, toDownload, s.st, s.cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
			return nil, err
		}
	}
	syncer.ReconcileBaselines(s.cfg.RootPath, conn, s.st)
	_ = s.st.Save()

	s.cfg.Connections = append(s.cfg.Connections, conn)
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return &InitialLinkResult{
		PendingID:      "",
		Connection:     conn,
		NeedsDecision:  false,
		UntrackedFiles: []string{},
	}, nil
}

// ChangeTarget updates the target directory of an existing connection atomically under lock (R2).
// The original connection remains intact if validation fails or pending decision is cancelled.
func (s *Service) ChangeTarget(connectionID string, targetPath string) (*InitialLinkResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredPendingLinksLocked()

	var connToReplace *config.Connection
	for i := range s.cfg.Connections {
		if s.cfg.Connections[i].ID == connectionID {
			connToReplace = &s.cfg.Connections[i]
			break
		}
	}
	if connToReplace == nil {
		return nil, apperr.New(apperr.TargetNotFound).WithParam("connectionId", connectionID)
	}

	if err := config.ValidateConnection(s.cfg.RootPath, targetPath, s.cfg.Connections, connectionID); err != nil {
		return nil, err
	}

	absTarget, _ := filepath.Abs(targetPath)
	newConn := config.Connection{
		ID:         generateID(),
		Profile:    connToReplace.Profile,
		TargetPath: absTarget,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	status, err := syncer.ComputeStatus(s.cfg.RootPath, newConn, s.st)
	if err != nil {
		return nil, err
	}

	var untracked []string
	var toDownload []string
	for _, f := range status.Files {
		switch f.Status {
		case syncer.FileUntrackedDiff:
			untracked = append(untracked, f.RelPath)
		case syncer.FileNewInSource, syncer.FileSourceChanged, syncer.FileTargetMissing:
			toDownload = append(toDownload, f.RelPath)
		}
	}

	if len(untracked) > 0 {
		s.pendingLinks[newConn.ID] = PendingLink{
			Connection:           newConn,
			ReplacesConnectionID: connectionID,
			CreatedAt:            time.Now(),
		}
		return &InitialLinkResult{
			PendingID:      newConn.ID,
			Connection:     newConn,
			NeedsDecision:  true,
			UntrackedFiles: untracked,
		}, nil
	}

	// No untracked diffs: apply new files immediately and atomically replace connection
	if len(toDownload) > 0 {
		allConns := append(append([]config.Connection{}, s.cfg.Connections...), newConn)
		if _, err := syncer.Apply(s.cfg.RootPath, newConn, allConns, s.cfg.BackupPath, toDownload, s.st, s.cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
			return nil, err
		}
	}

	var updatedConns []config.Connection
	for _, c := range s.cfg.Connections {
		if c.ID != connectionID {
			updatedConns = append(updatedConns, c)
		}
	}
	s.cfg.Connections = append(updatedConns, newConn)
	delete(s.st.Baselines, connectionID)

	syncer.ReconcileBaselines(s.cfg.RootPath, newConn, s.st)
	_ = s.st.Save()

	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return &InitialLinkResult{
		PendingID:      "",
		Connection:     newConn,
		NeedsDecision:  false,
		UntrackedFiles: []string{},
	}, nil
}

// ResolveInitialLink resolves a pending initial link by pendingID (M2, R2).
func (s *Service) ResolveInitialLink(pendingID string, choice string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredPendingLinksLocked()

	pending, ok := s.pendingLinks[pendingID]
	if !ok {
		return apperr.New(apperr.TargetNotFound).WithParam("pendingId", pendingID)
	}

	conn := pending.Connection

	// Revalidate connection before persisting
	excludeID := pending.ReplacesConnectionID
	if excludeID == "" {
		excludeID = conn.ID
	}
	if err := config.ValidateConnection(s.cfg.RootPath, conn.TargetPath, s.cfg.Connections, excludeID); err != nil {
		delete(s.pendingLinks, pendingID)
		return err
	}

	status, err := syncer.ComputeStatus(s.cfg.RootPath, conn, s.st)
	if err != nil {
		return err
	}

	var untracked []string
	var toDownload []string
	for _, f := range status.Files {
		switch f.Status {
		case syncer.FileUntrackedDiff:
			untracked = append(untracked, f.RelPath)
		case syncer.FileNewInSource, syncer.FileSourceChanged, syncer.FileTargetMissing:
			toDownload = append(toDownload, f.RelPath)
		}
	}

	allConns := append(append([]config.Connection{}, s.cfg.Connections...), conn)

	switch choice {
	case syncer.ResolutionKeepRepo:
		filesToApply := append(untracked, toDownload...)
		if len(filesToApply) > 0 {
			if _, err := syncer.Apply(s.cfg.RootPath, conn, allConns, s.cfg.BackupPath, filesToApply, s.st, s.cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
				return err
			}
		}
	case syncer.ResolutionKeepLocal:
		if len(untracked) > 0 {
			if err := syncer.Capture(s.cfg.RootPath, conn, untracked, s.st); err != nil {
				return err
			}
		}
		if len(toDownload) > 0 {
			if _, err := syncer.Apply(s.cfg.RootPath, conn, allConns, s.cfg.BackupPath, toDownload, s.st, s.cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
				return err
			}
		}
	case "decide_later":
		// Leave untracked_diff untouched
	default:
		return apperr.New(apperr.UnresolvedConflicts)
	}

	// Atomically remove replaced connection if applicable (R2)
	if pending.ReplacesConnectionID != "" {
		var updatedConns []config.Connection
		for _, c := range s.cfg.Connections {
			if c.ID != pending.ReplacesConnectionID {
				updatedConns = append(updatedConns, c)
			}
		}
		s.cfg.Connections = updatedConns
		delete(s.st.Baselines, pending.ReplacesConnectionID)
	}

	delete(s.pendingLinks, pendingID)
	syncer.ReconcileBaselines(s.cfg.RootPath, conn, s.st)
	_ = s.st.Save()

	s.cfg.Connections = append(s.cfg.Connections, conn)
	if err := config.Save(s.cfg); err != nil {
		return err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return nil
}

// CancelPendingLink cancels and removes a pending connection link (R3).
func (s *Service) CancelPendingLink(pendingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingLinks, pendingID)
	return nil
}

// DisconnectProfile removes a connection and cleans up its baseline state under lock.
func (s *Service) DisconnectProfile(connectionID string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updated []config.Connection
	for _, c := range s.cfg.Connections {
		if c.ID != connectionID {
			updated = append(updated, c)
		}
	}
	s.cfg.Connections = updated

	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}

	s.st.RemoveConnection(connectionID)
	_ = s.st.Save()

	if s.cfg.RootPath != "" {
		if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
			s.emitPlanLocked(plan)
		}
	}

	return cloneConfig(s.cfg), nil
}

// GetStatus computes and returns the current local sync plan (without network fetch, H6).
func (s *Service) GetStatus() (*orchestrator.SyncPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reconcileAllLocked()
	return s.orch.PlanSyncWithOptions(s.cfg, s.st, false)
}

// PlanSync calculates the synchronization plan with remote fetch.
func (s *Service) PlanSync() (*orchestrator.SyncPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reconcileAllLocked()
	plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, true)
	if err != nil {
		return nil, err
	}
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

// TryBackgroundCheck attempts to acquire the service lock without blocking (for the scheduler, C3).
// If acquired, it runs a remote fetch + PlanSync and emits the full SyncPlan (H5).
func (s *Service) TryBackgroundCheck() (*orchestrator.SyncPlan, bool) {
	if !s.mu.TryLock() {
		return nil, false
	}
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return nil, false
	}

	s.reconcileAllLocked()
	plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, true)
	if err != nil {
		return nil, false
	}

	nowStr := config.FormatRFC3339(time.Now())
	s.st.SetTimestamps("", nowStr)
	_ = s.st.Save()
	plan.LastCheckAt = nowStr

	s.emitPlanLocked(plan)
	return plan, true
}

// ApplyConnection performs a local download for a specific connection under lock.
func (s *Service) ApplyConnection(connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var targetConn *config.Connection
	for i := range s.cfg.Connections {
		if s.cfg.Connections[i].ID == connectionID {
			targetConn = &s.cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return apperr.New(apperr.TargetNotFound).WithParam("connectionId", connectionID)
	}

	status, err := syncer.ComputeStatus(s.cfg.RootPath, *targetConn, s.st)
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

	if _, err := syncer.Apply(s.cfg.RootPath, *targetConn, s.cfg.Connections, s.cfg.BackupPath, filesToApply, s.st, s.cfg.BackupRetention, backup.ReasonApply); err != nil {
		return err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return nil
}

// ExecuteSync performs the full synchronization operation under lock, verifying the plan fingerprint (H1, C5).
func (s *Service) ExecuteSync(fingerprint string, resolutions []orchestrator.ConflictResolution) (*orchestrator.SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.orch.ExecuteSyncLocked(s.cfg, s.st, fingerprint, resolutions)
	if err != nil {
		return nil, err
	}

	if plan, planErr := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); planErr == nil {
		s.emitPlanLocked(plan)
	}

	return res, nil
}

// ListBackups lists available snapshots for a given connection under lock.
func (s *Service) ListBackups(connectionID string) ([]backup.BackupInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var targetConn *config.Connection
	for i := range s.cfg.Connections {
		if s.cfg.Connections[i].ID == connectionID {
			targetConn = &s.cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return []backup.BackupInfo{}, nil
	}

	return backup.ListBackups(s.cfg.BackupPath, *targetConn, s.cfg.Connections)
}

// RestoreBackup restores files from a snapshot back to the target directory under lock (C4).
func (s *Service) RestoreBackup(connectionID string, backupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var targetConn *config.Connection
	for i := range s.cfg.Connections {
		if s.cfg.Connections[i].ID == connectionID {
			targetConn = &s.cfg.Connections[i]
			break
		}
	}
	if targetConn == nil {
		return apperr.New(apperr.TargetNotFound).WithParam("connectionId", connectionID)
	}

	if err := backup.RestoreBackup(s.cfg.BackupPath, *targetConn, s.cfg.Connections, backupID, s.st, s.cfg.BackupRetention); err != nil {
		return err
	}

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return nil
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
