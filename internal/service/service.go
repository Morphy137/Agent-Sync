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
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/profiles"
	"agentsync/internal/state"
	"agentsync/internal/subscriptions"
	"agentsync/internal/syncer"
)

// SettingsPayload defines incoming preferences from the settings view.
type SettingsPayload struct {
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	Palette              string `json:"palette"`
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

// PathInfo describes the status and attributes of an inspected path (B.2).
type PathInfo struct {
	Normalized       string `json:"normalized"`
	Exists           bool   `json:"exists"`
	IsDir            bool   `json:"isDir"`
	Writable         bool   `json:"writable"`
	IsInsideRoot     bool   `json:"isInsideRoot"`
	AlreadyConnected bool   `json:"alreadyConnected"`
}

// AgentFolderSuggestion represents an automatically detected agent config folder (B.2).
type AgentFolderSuggestion struct {
	Path             string `json:"path"`
	Label            string `json:"label"`
	SuggestedProfile string `json:"suggestedProfile"`
}

// TargetEntry represents a file or directory inside a destination folder (B.2).
type TargetEntry struct {
	RelPath   string `json:"relPath"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	FileCount int    `json:"fileCount"`
	TooLarge  bool   `json:"tooLarge"`
	Reason    string `json:"reason,omitempty"`
}

// ConnectionDraftRequest defines the payload for creating a connection draft (B.2).
type ConnectionDraftRequest struct {
	Profile              string   `json:"profile"`
	TargetPath           string   `json:"targetPath"`
	CreateProfile        bool     `json:"createProfile"`
	ImportPaths          []string `json:"importPaths"`
	ReplacesConnectionID string   `json:"replacesConnectionId,omitempty"`
}

// DraftDifference describes a file existing in both target and repo with differing contents.
type DraftDifference struct {
	RelPath    string `json:"relPath"`
	RepoHash   string `json:"repoHash"`
	TargetHash string `json:"targetHash"`
}

// ConnectionDraftPreview provides calculated preview information for a draft (B.2).
type ConnectionDraftPreview struct {
	ToImport         []string          `json:"toImport"`
	ToCreateInTarget []string          `json:"toCreateInTarget"`
	ToOverwrite      []string          `json:"toOverwrite"`
	Differences      []DraftDifference `json:"differences"`
}

// ConnectionDraft represents an in-flight connection configuration being created in the wizard (B.2).
type ConnectionDraft struct {
	ID                   string                 `json:"id"`
	Profile              string                 `json:"profile"`
	TargetPath           string                 `json:"targetPath"`
	CreateProfile        bool                   `json:"createProfile"`
	ImportPaths          []string               `json:"importPaths"`
	ReplacesConnectionID string                 `json:"replacesConnectionId,omitempty"`
	CreatedAt            time.Time              `json:"createdAt"`
	Preview              ConnectionDraftPreview `json:"preview"`
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
	mu           serviceLock
	lastSnapshot snapshotHolder
	cfg          *config.Config
	st           *state.State
	orch         *orchestrator.Orchestrator
	pendingLinks map[string]PendingLink
	drafts       map[string]*ConnectionDraft
	emitFunc     func(eventName string, data any)
	dialogFunc   func(title, defaultDir string) (string, error)
	fileDialog   func(title, defaultDir string) (string, error)
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
		cfg:          cfg,
		st:           st,
		orch:         orchestrator.New(),
		pendingLinks: make(map[string]PendingLink),
		drafts:       make(map[string]*ConnectionDraft),
		subUpdates:   make(map[string]subscriptions.UpdateInfo),
		emitFunc:     emitFunc,
	}
	svc.enableSharedMode()
	return svc
}

// NewWithInstances creates a Service with explicitly supplied config and state (for unit testing).
func NewWithInstances(cfg *config.Config, st *state.State, orch *orchestrator.Orchestrator, emitFunc func(eventName string, data any)) *Service {
	return &Service{
		cfg:          cfg,
		st:           st,
		orch:         orch,
		pendingLinks: make(map[string]PendingLink),
		drafts:       make(map[string]*ConnectionDraft),
		subUpdates:   make(map[string]subscriptions.UpdateInfo),
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
	for id, d := range s.drafts {
		if now.Sub(d.CreatedAt) > PendingLinkTTL {
			delete(s.drafts, id)
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
	s.drafts = make(map[string]*ConnectionDraft)
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
	cp.Locations = make([]config.Location, len(cfg.Locations))
	copy(cp.Locations, cfg.Locations)
	cp.Subscriptions = make([]config.Subscription, len(cfg.Subscriptions))
	copy(cp.Subscriptions, cfg.Subscriptions)
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
	if payload.Palette != "" {
		s.cfg.Palette = payload.Palette
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

	s.reconcileAllLocked()
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
	s.checkSubscriptionsLocked()
	s.syncOrchestratorRulesLocked()
	plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, true)
	if err != nil {
		return nil, false
	}
	s.annotatePlanLocked(&plan.RulesPending, &plan.RulesConflicts)
	s.emitRulesChangedLocked()

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

	alreadyConnected := false
	for _, c := range s.cfg.Connections {
		if strings.EqualFold(filepath.Clean(c.TargetPath), cleanPath) {
			alreadyConnected = true
			break
		}
	}

	return PathInfo{
		Normalized:       cleanPath,
		Exists:           exists,
		IsDir:            isDir,
		Writable:         writable,
		IsInsideRoot:     isInsideRoot,
		AlreadyConnected: alreadyConnected,
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

// CreateProfile creates a profile directory with a .gitkeep file inside the configured root (B.2).
func (s *Service) CreateProfile(name string) (profiles.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return profiles.Profile{}, apperr.New(apperr.RootNotFound)
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.HasPrefix(trimmed, ".") || !isValidWindowsFolderName(trimmed) {
		return profiles.Profile{}, apperr.New(apperr.InvalidFolderName).WithParam("name", name)
	}

	profilePath := filepath.Join(s.cfg.RootPath, trimmed)
	if _, err := os.Stat(profilePath); err == nil {
		return profiles.Profile{}, apperr.New(apperr.FolderExists).WithParam("profile", trimmed)
	}

	for _, c := range s.cfg.Connections {
		if strings.EqualFold(c.Profile, trimmed) {
			return profiles.Profile{}, apperr.New(apperr.ProfileAlreadyConnected).WithParam("profile", trimmed)
		}
	}

	if err := os.MkdirAll(profilePath, fsops.DefaultDirPerms); err != nil {
		return profiles.Profile{}, apperr.Wrap(apperr.WriteFailed, err)
	}

	gitkeepPath := filepath.Join(profilePath, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, []byte(""), fsops.DefaultFilePerms); err != nil {
		_ = os.RemoveAll(profilePath)
		return profiles.Profile{}, apperr.Wrap(apperr.WriteFailed, err)
	}

	return profiles.Profile{
		Name:      trimmed,
		Path:      profilePath,
		FileCount: 0,
	}, nil
}

// DetectAgentFolders returns suggestions for standard agent folders that exist on this PC (B.2).
func (s *Service) DetectAgentFolders() []AgentFolderSuggestion {
	homeDir := os.Getenv("AGENTSYNC_HOME_OVERRIDE")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
		if homeDir == "" {
			homeDir = os.Getenv("HOME")
		}
	}
	if homeDir == "" {
		return []AgentFolderSuggestion{}
	}

	candidates := []struct {
		relPath          string
		label            string
		suggestedProfile string
	}{
		{relPath: ".claude", label: "Claude Code", suggestedProfile: "CLAUDE"},
		{relPath: ".gemini", label: "Gemini CLI", suggestedProfile: "GEMINI"},
		{relPath: ".codex", label: "Codex", suggestedProfile: "CODEX"},
		{relPath: filepath.Join(".config", "opencode"), label: "OpenCode", suggestedProfile: "OPENCODE"},
	}

	var suggestions []AgentFolderSuggestion
	for _, cand := range candidates {
		candPath := filepath.Join(homeDir, cand.relPath)
		if fi, err := os.Stat(candPath); err == nil && fi.IsDir() {
			suggestions = append(suggestions, AgentFolderSuggestion{
				Path:             candPath,
				Label:            cand.label,
				SuggestedProfile: cand.suggestedProfile,
			})
		}
	}

	if suggestions == nil {
		return []AgentFolderSuggestion{}
	}
	return suggestions
}

var DefaultExcludedFolderNames = map[string]bool{
	"projects":        true,
	"todos":           true,
	"statsig":         true,
	"shell-snapshots": true,
	"logs":            true,
	"cache":           true,
	"tmp":             true,
	"node_modules":    true,
	".git":            true,
}

const (
	MaxTargetEntryFiles = 2000
	MaxTargetEntryBytes = 50 * 1024 * 1024 // 50 MB
)

// scanDirStats recursively counts files and total size inside a directory.
func scanDirStats(dir string) (int, int64) {
	var count int
	var size int64
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			count++
			size += info.Size()
		}
		return nil
	})
	return count, size
}

// ListTargetEntries returns the tree of destination files and folders up to maxDepth (B.2).
func (s *Service) ListTargetEntries(rawPath string, maxDepth int) []TargetEntry {
	if maxDepth <= 0 {
		maxDepth = 2
	}

	cleanTarget := filepath.Clean(expandPath(rawPath))
	fi, err := os.Stat(cleanTarget)
	if err != nil || !fi.IsDir() {
		return []TargetEntry{}
	}

	var results []TargetEntry

	var walk func(currentDir string, depth int)
	walk = func(currentDir string, depth int) {
		if depth > maxDepth {
			return
		}

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			return
		}

		for _, entry := range entries {
			name := entry.Name()
			fullPath := filepath.Join(currentDir, name)
			rel, err := filepath.Rel(cleanTarget, fullPath)
			if err != nil {
				continue
			}
			relSlash := filepath.ToSlash(rel)

			if entry.IsDir() {
				lowerName := strings.ToLower(name)
				if DefaultExcludedFolderNames[lowerName] {
					results = append(results, TargetEntry{
						RelPath:  relSlash,
						IsDir:    true,
						TooLarge: true,
						Reason:   "cache_dir",
					})
					continue
				}

				count, size := scanDirStats(fullPath)
				tooLarge := count > MaxTargetEntryFiles || size > MaxTargetEntryBytes
				reason := ""
				if tooLarge {
					reason = "too_large"
				}

				results = append(results, TargetEntry{
					RelPath:   relSlash,
					IsDir:     true,
					Size:      size,
					FileCount: count,
					TooLarge:  tooLarge,
					Reason:    reason,
				})

				if !tooLarge && depth < maxDepth {
					walk(fullPath, depth+1)
				}
			} else {
				if fsops.ShouldIgnore(name) {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					continue
				}
				results = append(results, TargetEntry{
					RelPath:   relSlash,
					IsDir:     false,
					Size:      info.Size(),
					FileCount: 1,
					TooLarge:  false,
				})
			}
		}
	}

	walk(cleanTarget, 1)

	if results == nil {
		return []TargetEntry{}
	}
	return results
}

// BeginConnection prepares and returns a connection draft preview without writing to disk (B.2).
func (s *Service) BeginConnection(req ConnectionDraftRequest) (*ConnectionDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredPendingLinksLocked()

	if s.cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}

	trimmedProfile := strings.TrimSpace(req.Profile)
	if trimmedProfile == "" {
		return nil, apperr.New(apperr.InvalidFolderName).WithParam("name", req.Profile)
	}

	if !isValidWindowsFolderName(trimmedProfile) || strings.HasPrefix(trimmedProfile, ".") {
		return nil, apperr.New(apperr.InvalidFolderName).WithParam("name", req.Profile)
	}

	profileDir := filepath.Join(s.cfg.RootPath, trimmedProfile)

	if req.CreateProfile {
		if _, err := os.Stat(profileDir); err == nil {
			return nil, apperr.New(apperr.FolderExists).WithParam("profile", trimmedProfile)
		}
		for _, c := range s.cfg.Connections {
			if strings.EqualFold(c.Profile, trimmedProfile) && c.ID != req.ReplacesConnectionID {
				return nil, apperr.New(apperr.ProfileAlreadyConnected).WithParam("profile", trimmedProfile)
			}
		}
	} else {
		fi, err := os.Stat(profileDir)
		if err != nil || !fi.IsDir() {
			return nil, apperr.New(apperr.ProfileNotFound).WithParam("profile", trimmedProfile)
		}
		for _, c := range s.cfg.Connections {
			if strings.EqualFold(c.Profile, trimmedProfile) && c.ID != req.ReplacesConnectionID {
				return nil, apperr.New(apperr.ProfileAlreadyConnected).WithParam("profile", trimmedProfile)
			}
		}
	}

	targetClean := filepath.Clean(expandPath(req.TargetPath))
	if err := config.ValidateConnection(s.cfg.RootPath, targetClean, s.cfg.Connections, req.ReplacesConnectionID); err != nil {
		return nil, err
	}

	preview := ConnectionDraftPreview{
		ToImport:         make([]string, 0),
		ToCreateInTarget: make([]string, 0),
		ToOverwrite:      make([]string, 0),
		Differences:      make([]DraftDifference, 0),
	}

	for _, ip := range req.ImportPaths {
		cleanIP := filepath.ToSlash(filepath.Clean(ip))
		if cleanIP != "" && cleanIP != "." {
			preview.ToImport = append(preview.ToImport, cleanIP)
		}
	}

	if !req.CreateProfile {
		repoFiles, err := fsops.WalkManaged(profileDir)
		if err == nil {
			for relPath, absRepoFile := range repoFiles {
				if fsops.ShouldIgnore(filepath.Base(relPath)) {
					continue
				}
				targetFilePath := filepath.Join(targetClean, filepath.FromSlash(relPath))
				tfi, err := os.Stat(targetFilePath)
				if err != nil || tfi.IsDir() {
					preview.ToCreateInTarget = append(preview.ToCreateInTarget, relPath)
				} else {
					repoHash, _ := fsops.HashFile(absRepoFile)
					targetHash, _ := fsops.HashFile(targetFilePath)
					if repoHash != targetHash {
						preview.Differences = append(preview.Differences, DraftDifference{
							RelPath:    relPath,
							RepoHash:   repoHash,
							TargetHash: targetHash,
						})
					}
				}
			}
		}
	}

	draft := &ConnectionDraft{
		ID:                   generateID(),
		Profile:              trimmedProfile,
		TargetPath:           targetClean,
		CreateProfile:        req.CreateProfile,
		ImportPaths:          preview.ToImport,
		ReplacesConnectionID: req.ReplacesConnectionID,
		CreatedAt:            time.Now(),
		Preview:              preview,
	}

	s.drafts[draft.ID] = draft
	return draft, nil
}

// CommitConnection applies a connection draft with automatic rollback on error (B.2).
func (s *Service) CommitConnection(draftID string, resolutions []orchestrator.ConflictResolution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanExpiredPendingLinksLocked()

	draft, ok := s.drafts[draftID]
	if !ok {
		return apperr.New(apperr.DraftNotFound).WithParam("draftId", draftID)
	}

	var createdProfileDir string
	var importedRepoFiles []string

	rollback := func() {
		for _, f := range importedRepoFiles {
			_ = os.Remove(f)
		}
		if createdProfileDir != "" {
			_ = os.RemoveAll(createdProfileDir)
		}
	}

	profileDir := filepath.Join(s.cfg.RootPath, draft.Profile)

	// 1. Create profile folder if needed
	if draft.CreateProfile {
		if err := os.MkdirAll(profileDir, fsops.DefaultDirPerms); err != nil {
			return apperr.Wrap(apperr.WriteFailed, err)
		}
		createdProfileDir = profileDir

		gitkeepPath := filepath.Join(profileDir, ".gitkeep")
		if err := os.WriteFile(gitkeepPath, []byte(""), fsops.DefaultFilePerms); err != nil {
			rollback()
			return apperr.Wrap(apperr.WriteFailed, err)
		}
	}

	// 2. Import selected files from target to repo
	for _, relPath := range draft.ImportPaths {
		src := filepath.Join(draft.TargetPath, filepath.FromSlash(relPath))
		dst := filepath.Join(profileDir, filepath.FromSlash(relPath))
		if fi, err := os.Stat(src); err == nil && !fi.IsDir() {
			if err := fsops.AtomicCopy(src, dst); err != nil {
				rollback()
				return apperr.Wrap(apperr.WriteFailed, err)
			}
			importedRepoFiles = append(importedRepoFiles, dst)
		}
	}

	resolutionMap := make(map[string]string)
	for _, r := range resolutions {
		resolutionMap[r.RelPath] = r.Resolution
	}

	var filesToApplyToTarget []string
	for _, diff := range draft.Preview.Differences {
		res := resolutionMap[diff.RelPath]
		if res == syncer.ResolutionKeepLocal {
			src := filepath.Join(draft.TargetPath, filepath.FromSlash(diff.RelPath))
			dst := filepath.Join(profileDir, filepath.FromSlash(diff.RelPath))
			if err := fsops.AtomicCopy(src, dst); err != nil {
				rollback()
				return apperr.Wrap(apperr.WriteFailed, err)
			}
			importedRepoFiles = append(importedRepoFiles, dst)
		} else {
			filesToApplyToTarget = append(filesToApplyToTarget, diff.RelPath)
		}
	}

	filesToApplyToTarget = append(filesToApplyToTarget, draft.Preview.ToCreateInTarget...)

	newConn := config.Connection{
		ID:         generateID(),
		Profile:    draft.Profile,
		TargetPath: draft.TargetPath,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	// 3. Apply files to target with mandatory safety backup
	if len(filesToApplyToTarget) > 0 {
		allConns := append(append([]config.Connection{}, s.cfg.Connections...), newConn)
		if _, err := syncer.Apply(s.cfg.RootPath, newConn, allConns, s.cfg.BackupPath, filesToApplyToTarget, s.st, s.cfg.BackupRetention, backup.ReasonInitialLink); err != nil {
			rollback()
			return err
		}
	}

	// 4. Update config connections (atomically replacing replaced connection if applicable)
	if draft.ReplacesConnectionID != "" {
		var updatedConns []config.Connection
		for _, c := range s.cfg.Connections {
			if c.ID != draft.ReplacesConnectionID {
				updatedConns = append(updatedConns, c)
			}
		}
		s.cfg.Connections = updatedConns
		delete(s.st.Baselines, draft.ReplacesConnectionID)
	}

	s.cfg.Connections = append(s.cfg.Connections, newConn)

	syncer.ReconcileBaselines(s.cfg.RootPath, newConn, s.st)
	_ = s.st.Save()

	if err := config.Save(s.cfg); err != nil {
		rollback()
		return err
	}

	delete(s.drafts, draftID)

	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return nil
}

// CancelConnection discards a connection draft (B.2).
func (s *Service) CancelConnection(draftID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.drafts, draftID)
	return nil
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

	s.reconcileAllLocked()
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

	s.reconcileAllLocked()
	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.emitPlanLocked(plan)
	}

	return cloneConfig(s.cfg), nil
}
