package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

// Sync directions.
const (
	DirectionIdle     = "idle"
	DirectionDown     = "down"
	DirectionUp       = "up"
	DirectionBoth     = "both"
	DirectionConflict = "conflict"
	DirectionNoRemote = "no_remote"
	DirectionOffline  = "offline"
)

// SyncItem represents a file to be downloaded or uploaded.
type SyncItem struct {
	Profile  string `json:"profile"`
	RelPath  string `json:"relPath"`
	FullPath string `json:"fullPath"` // e.g. "CLAUDE/CLAUDE.md"
}

// ConflictItem represents an unresolved file conflict.
type ConflictItem struct {
	Profile string `json:"profile"`
	RelPath string `json:"relPath"`
	Reason  string `json:"reason"`
}

// ConflictResolution represents the user's decision for a conflicted file.
type ConflictResolution struct {
	Profile    string `json:"profile"`
	RelPath    string `json:"relPath"`
	Resolution string `json:"resolution"` // "keep_repo" or "keep_local"
}

// SyncPlan holds the calculated sync operations without side-effects.
type SyncPlan struct {
	Fingerprint        string                    `json:"fingerprint"`
	Direction          string                    `json:"direction"`
	Downloads          []SyncItem                `json:"downloads"`
	Uploads            []SyncItem                `json:"uploads"`
	Conflicts          []ConflictItem            `json:"conflicts"`
	CommitPreview      []string                  `json:"commitPreview"`
	Ahead              int                       `json:"ahead"`
	Behind             int                       `json:"behind"`
	HasRemote          bool                      `json:"hasRemote"`
	IsOffline          bool                      `json:"isOffline"`
	DirtyOutside       []string                  `json:"dirtyOutside"`
	DirtyOverlapExists bool                      `json:"dirtyOverlapExists"`
	Connections        []syncer.ConnectionStatus `json:"connections"`
	LastCheckAt        string                    `json:"lastCheckAt,omitempty"`
	RulesPending       int                       `json:"rulesPending"`
	RulesConflicts     int                       `json:"rulesConflicts"`
}

// SyncResult summarizes the outcome of ExecuteSync.
type SyncResult struct {
	Applied        int      `json:"applied"`
	Captured       int      `json:"captured"`
	Pulled         int      `json:"pulled"`
	Pushed         int      `json:"pushed"`
	BackupSessions []string `json:"backupSessions"`
	RulesWritten   int      `json:"rulesWritten"`
	RulesCaptured  int      `json:"rulesCaptured"`
	RulesSkipped   int      `json:"rulesSkipped"`
}

// Orchestrator coordinates the end-to-end sync workflow.
type Orchestrator struct {
	OnBeforePush func()
	// ManagedRepoPaths lists repo-relative paths (files or folders, forward slashes) owned by sync rules.
	// Changes under them are committed like profile changes instead of being reported as DirtyOutside.
	ManagedRepoPaths []string
}

// isManagedRepoPath reports whether a repo-relative path belongs to a profile connection or a sync rule.
func (o *Orchestrator) isManagedRepoPath(cfg *config.Config, repoPath string) bool {
	for _, conn := range cfg.Connections {
		if strings.HasPrefix(repoPath, conn.Profile+"/") {
			return true
		}
	}
	for _, managed := range o.ManagedRepoPaths {
		trimmed := strings.TrimSuffix(managed, "/")
		if repoPath == trimmed || strings.HasPrefix(repoPath, trimmed+"/") {
			return true
		}
	}
	return false
}

// New creates a new Orchestrator instance.
func New() *Orchestrator {
	return &Orchestrator{}
}

// computeFingerprint calculates a deterministic SHA-256 hash of the plan's actionable state.
func computeFingerprint(plan *SyncPlan) string {
	h := sha256.New()

	dls := make([]string, len(plan.Downloads))
	for i, d := range plan.Downloads {
		dls[i] = d.FullPath
	}
	sort.Strings(dls)

	ups := make([]string, len(plan.Uploads))
	for i, u := range plan.Uploads {
		ups[i] = u.FullPath
	}
	sort.Strings(ups)

	confs := make([]string, len(plan.Conflicts))
	for i, c := range plan.Conflicts {
		confs[i] = fmt.Sprintf("%s:%s:%s", c.Profile, c.RelPath, c.Reason)
	}
	sort.Strings(confs)

	commits := make([]string, len(plan.CommitPreview))
	copy(commits, plan.CommitPreview)
	sort.Strings(commits)

	dirty := make([]string, len(plan.DirtyOutside))
	copy(dirty, plan.DirtyOutside)
	sort.Strings(dirty)

	fmt.Fprintf(h, "dl:%s|up:%s|cf:%s|cp:%s|do:%s|a:%d|b:%d",
		strings.Join(dls, ","),
		strings.Join(ups, ","),
		strings.Join(confs, ","),
		strings.Join(commits, ","),
		strings.Join(dirty, ","),
		plan.Ahead,
		plan.Behind,
	)

	return hex.EncodeToString(h.Sum(nil))
}

// PlanSync calculates the required synchronization actions without performing mutations (with remote fetch).
func (o *Orchestrator) PlanSync(cfg *config.Config, st *state.State) (*SyncPlan, error) {
	return o.PlanSyncWithOptions(cfg, st, true)
}

// PlanSyncWithOptions calculates the synchronization plan, optionally fetching from the remote.
func (o *Orchestrator) PlanSyncWithOptions(cfg *config.Config, st *state.State, fetch bool) (*SyncPlan, error) {
	if cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}

	plan := &SyncPlan{
		Direction:     DirectionIdle,
		Downloads:     make([]SyncItem, 0),
		Uploads:       make([]SyncItem, 0),
		Conflicts:     make([]ConflictItem, 0),
		CommitPreview: make([]string, 0),
		DirtyOutside:  make([]string, 0),
		Connections:   make([]syncer.ConnectionStatus, 0),
	}
	if st != nil {
		plan.LastCheckAt = st.LastCheckAt
	}

	isRepo := gitops.IsRepo(cfg.RootPath)
	var runner *gitops.GitRunner
	if isRepo {
		runner = gitops.NewRunner(cfg.RootPath)
		remoteURL, _ := runner.RemoteURL()
		plan.HasRemote = remoteURL != ""

		if plan.HasRemote && fetch {
			if err := runner.Fetch(); err != nil {
				plan.IsOffline = true
			}
		}

		ahead, behind, _ := runner.AheadBehind()
		plan.Ahead = ahead
		plan.Behind = behind
	}

	// 1. Inspect each connection and collect file statuses
	conflictSeen := make(map[string]bool)
	for _, conn := range cfg.Connections {
		status, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
		if err != nil {
			continue
		}
		plan.Connections = append(plan.Connections, status)

		for _, file := range status.Files {
			fullPath := filepath.ToSlash(filepath.Join(conn.Profile, file.RelPath))
			switch file.Status {
			case syncer.FileConflict:
				conflictSeen[fullPath] = true
				plan.Conflicts = append(plan.Conflicts, ConflictItem{
					Profile: conn.Profile,
					RelPath: file.RelPath,
					Reason:  "conflict",
				})
			case syncer.FileUntrackedDiff:
				conflictSeen[fullPath] = true
				plan.Conflicts = append(plan.Conflicts, ConflictItem{
					Profile: conn.Profile,
					RelPath: file.RelPath,
					Reason:  "untracked_diff",
				})
			case syncer.FileTargetChanged:
				plan.Uploads = append(plan.Uploads, SyncItem{
					Profile:  conn.Profile,
					RelPath:  file.RelPath,
					FullPath: fullPath,
				})
			case syncer.FileSourceChanged, syncer.FileNewInSource, syncer.FileTargetMissing:
				plan.Downloads = append(plan.Downloads, SyncItem{
					Profile:  conn.Profile,
					RelPath:  file.RelPath,
					FullPath: fullPath,
				})
			}
		}
	}

	// 2. Check git status porcelain and incoming changed files if repository
	if isRepo && runner != nil {
		changes, err := runner.StatusPorcelain()
		if err == nil {
			for _, ch := range changes {
				if ch.Path == ".gitignore" {
					continue
				}
				if o.isManagedRepoPath(cfg, ch.Path) {
					plan.CommitPreview = append(plan.CommitPreview, ch.Path)
				} else {
					plan.DirtyOutside = append(plan.DirtyOutside, ch.Path)
				}
			}
		}

		// Detect overlaps between incoming remote changes and local uploads, CommitPreview, and DirtyOutside (H2)
		incoming, err := runner.IncomingChangedFiles()
		if err == nil && len(incoming) > 0 {
			uploadOrCommitMap := make(map[string]bool)
			for _, up := range plan.Uploads {
				uploadOrCommitMap[up.FullPath] = true
			}
			for _, cp := range plan.CommitPreview {
				uploadOrCommitMap[cp] = true
			}

			dirtyOutsideMap := make(map[string]bool)
			for _, do := range plan.DirtyOutside {
				dirtyOutsideMap[do] = true
			}

			for _, inPath := range incoming {
				if dirtyOutsideMap[inPath] {
					plan.DirtyOverlapExists = true
				}
				if uploadOrCommitMap[inPath] && !conflictSeen[inPath] {
					parts := strings.SplitN(inPath, "/", 2)
					profile := parts[0]
					rel := ""
					if len(parts) > 1 {
						rel = parts[1]
					}
					conflictSeen[inPath] = true
					plan.Conflicts = append(plan.Conflicts, ConflictItem{
						Profile: profile,
						RelPath: rel,
						Reason:  "remote_and_local_overlap",
					})
				}
			}
		}
	}

	// 3. Calculate Direction
	hasUploads := len(plan.Uploads) > 0 || len(plan.CommitPreview) > 0 || plan.Ahead > 0
	hasDownloads := len(plan.Downloads) > 0 || plan.Behind > 0

	if len(plan.Conflicts) > 0 {
		plan.Direction = DirectionConflict
	} else if hasUploads && hasDownloads {
		plan.Direction = DirectionBoth
	} else if hasUploads {
		plan.Direction = DirectionUp
	} else if hasDownloads {
		plan.Direction = DirectionDown
	} else if !plan.HasRemote {
		plan.Direction = DirectionIdle
	} else if plan.IsOffline {
		plan.Direction = DirectionOffline
	} else {
		plan.Direction = DirectionIdle
	}

	plan.Fingerprint = computeFingerprint(plan)
	return plan, nil
}

// ExecuteSync performs the full synchronization workflow using the A.1 algorithm under Service lock (R6).
func (o *Orchestrator) ExecuteSync(cfg *config.Config, st *state.State, expectedFingerprint string, resolutions []ConflictResolution) (*SyncResult, error) {
	// Reconcile baselines before planning
	if st != nil {
		mutated := false
		for _, conn := range cfg.Connections {
			if syncer.ReconcileBaselines(cfg.RootPath, conn, st) {
				mutated = true
			}
		}
		if mutated {
			_ = st.Save()
		}
	}

	// Step 1: Revalidate plan with fetch=true
	plan, err := o.PlanSyncWithOptions(cfg, st, true)
	if err != nil {
		return nil, err
	}
	if expectedFingerprint != "" && plan.Fingerprint != expectedFingerprint {
		return nil, apperr.New(apperr.PlanChanged)
	}

	// Step 2: Validate conflict resolutions
	resolvedDownloads := make([]SyncItem, 0)
	resolvedUploads := make([]SyncItem, 0)
	keepRepoSet := make(map[string]bool)
	keepLocalSet := make(map[string]bool)

	if len(plan.Conflicts) > 0 {
		resolutionMap := make(map[string]string)
		for _, r := range resolutions {
			key := fmt.Sprintf("%s:%s", r.Profile, r.RelPath)
			resolutionMap[key] = r.Resolution
		}

		for _, conflict := range plan.Conflicts {
			key := fmt.Sprintf("%s:%s", conflict.Profile, conflict.RelPath)
			res, ok := resolutionMap[key]
			if !ok || (res != syncer.ResolutionKeepRepo && res != syncer.ResolutionKeepLocal) {
				return nil, apperr.New(apperr.UnresolvedConflicts).WithParam("file", key)
			}

			fullPath := filepath.ToSlash(filepath.Join(conflict.Profile, conflict.RelPath))
			item := SyncItem{
				Profile:  conflict.Profile,
				RelPath:  conflict.RelPath,
				FullPath: fullPath,
			}

			if res == syncer.ResolutionKeepRepo {
				resolvedDownloads = append(resolvedDownloads, item)
				keepRepoSet[fullPath] = true
			} else {
				resolvedUploads = append(resolvedUploads, item)
				keepLocalSet[fullPath] = true
			}
		}
	}

	// Filter out conflicts from initial Downloads/Uploads
	conflictSet := make(map[string]bool)
	for _, c := range plan.Conflicts {
		conflictSet[filepath.ToSlash(filepath.Join(c.Profile, c.RelPath))] = true
	}

	filteredUploads := make([]SyncItem, 0)
	for _, up := range plan.Uploads {
		if !conflictSet[up.FullPath] {
			filteredUploads = append(filteredUploads, up)
		}
	}

	isRepo := gitops.IsRepo(cfg.RootPath)
	var runner *gitops.GitRunner

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "local"
	}

	// Step 3: Git preflight
	result := &SyncResult{
		BackupSessions: make([]string, 0),
	}

	if isRepo {
		runner = gitops.NewRunner(cfg.RootPath)
		runner.OnBeforePush = o.OnBeforePush

		if plan.DirtyOverlapExists {
			return nil, apperr.New(apperr.GitDirtyWorktree)
		}

		willCommit := len(plan.CommitPreview) > 0 || len(filteredUploads) > 0 || len(resolvedUploads) > 0
		if willCommit {
			if err := runner.EnsureIdentity(); err != nil {
				return nil, err
			}
		}

		// Step 4: Commit pending manual edits inside profiles (CommitPreview)
		if len(plan.CommitPreview) > 0 {
			var pendingToCommit []string
			var filesToDiscard []string
			for _, cp := range plan.CommitPreview {
				if keepRepoSet[cp] {
					filesToDiscard = append(filesToDiscard, cp)
				} else {
					pendingToCommit = append(pendingToCommit, cp)
				}
			}

			if len(filesToDiscard) > 0 {
				repoBackup, err := backup.CreateRepoBackup(cfg.BackupPath, cfg.RootPath, filesToDiscard, backup.ReasonDiscardRepoEdit, cfg.BackupRetention)
				if err != nil {
					return nil, err
				}
				if repoBackup != nil {
					result.BackupSessions = append(result.BackupSessions, repoBackup.ID)
				}
				for _, cp := range filesToDiscard {
					if _, err := runner.DiscardPath(cp); err != nil {
						return nil, err
					}
				}
			}

			if len(pendingToCommit) > 0 {
				msg := fmt.Sprintf("chore(sync): commit pending changes from %s", hostname)
				if err := runner.CommitPaths(pendingToCommit, msg); err != nil {
					return nil, err
				}
			}
		}
	}

	// Step 5: Pull first (if behind > 0)
	if isRepo && runner != nil && runner.HasUpstream() {
		ahead, behind, _ := runner.AheadBehind()
		if behind > 0 {
			if ahead == 0 {
				if err := runner.PullFastForward(); err != nil {
					return nil, err
				}
			} else {
				if err := runner.PullRebase(); err != nil {
					return nil, err
				}
			}
			result.Pulled = behind
		}
	}

	// Step 6: Recalculate status after pull
	allDownloads := append([]SyncItem{}, resolvedDownloads...)
	downloadSet := make(map[string]bool)
	for _, dl := range allDownloads {
		downloadSet[dl.FullPath] = true
	}

	allUploads := append([]SyncItem{}, resolvedUploads...)
	uploadSet := make(map[string]bool)
	for _, up := range allUploads {
		uploadSet[up.FullPath] = true
	}

	for _, conn := range cfg.Connections {
		statusAfterPull, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
		if err != nil {
			continue
		}
		for _, file := range statusAfterPull.Files {
			fullPath := filepath.ToSlash(filepath.Join(conn.Profile, file.RelPath))
			if keepRepoSet[fullPath] || keepLocalSet[fullPath] {
				continue
			}

			switch file.Status {
			case syncer.FileSourceChanged, syncer.FileNewInSource, syncer.FileTargetMissing:
				if !downloadSet[fullPath] {
					downloadSet[fullPath] = true
					allDownloads = append(allDownloads, SyncItem{
						Profile:  conn.Profile,
						RelPath:  file.RelPath,
						FullPath: fullPath,
					})
				}
			case syncer.FileTargetChanged:
				if !uploadSet[fullPath] {
					uploadSet[fullPath] = true
					allUploads = append(allUploads, SyncItem{
						Profile:  conn.Profile,
						RelPath:  file.RelPath,
						FullPath: fullPath,
					})
				}
			}
		}
	}

	connByProfile := make(map[string]config.Connection)
	for _, c := range cfg.Connections {
		connByProfile[c.Profile] = c
	}

	// Step 7: Download (Apply) with mandatory backup (C1) and BackupInfo.ID recording (M5)
	downloadsByProfile := make(map[string][]string)
	for _, dl := range allDownloads {
		downloadsByProfile[dl.Profile] = append(downloadsByProfile[dl.Profile], dl.RelPath)
	}

	for prof, files := range downloadsByProfile {
		conn, ok := connByProfile[prof]
		if !ok {
			continue
		}
		bInfo, err := syncer.Apply(cfg.RootPath, conn, cfg.Connections, cfg.BackupPath, files, st, cfg.BackupRetention, backup.ReasonSync)
		if err != nil {
			return nil, err
		}
		result.Applied += len(files)
		if bInfo != nil && bInfo.ID != "" {
			result.BackupSessions = append(result.BackupSessions, bInfo.ID)
		}
	}

	// Step 8: Upload (Capture) of target_changed + keep_local conflicts
	uploadsByProfile := make(map[string][]string)
	for _, up := range allUploads {
		uploadsByProfile[up.Profile] = append(uploadsByProfile[up.Profile], up.RelPath)
	}

	var committedProfiles []string
	for prof, files := range uploadsByProfile {
		conn, ok := connByProfile[prof]
		if !ok {
			continue
		}
		if err := syncer.Capture(cfg.RootPath, conn, files, st); err != nil {
			return nil, err
		}
		result.Captured += len(files)
		committedProfiles = append(committedProfiles, prof)
	}

	// Step 9: Commit of captures (propagate error, C2)
	if isRepo && runner != nil && len(allUploads) > 0 {
		var filesToCommit []string
		for _, up := range allUploads {
			filesToCommit = append(filesToCommit, up.FullPath)
		}
		sort.Strings(committedProfiles)
		profList := strings.Join(committedProfiles, ", ")
		if profList == "" {
			profList = "profiles"
		}
		msg := fmt.Sprintf("chore(sync): update %s from %s", profList, hostname)
		if err := runner.CommitPaths(filesToCommit, msg); err != nil {
			return nil, err
		}
	}

	// Step 10: Push with one retry on GitPushRejected (H3)
	if isRepo && runner != nil && runner.HasUpstream() {
		ahead, _, _ := runner.AheadBehind()
		if ahead > 0 {
			if err := runner.Push(); err != nil {
				var appErr *apperr.Error
				if errors.As(err, &appErr) && appErr.Code == apperr.GitPushRejected {
					if rbErr := runner.PullRebase(); rbErr != nil {
						return nil, rbErr
					}
					if retryErr := runner.Push(); retryErr != nil {
						return nil, retryErr
					}
				} else {
					return nil, err
				}
			}
			result.Pushed = ahead
		}
	}

	// Step 11: Persist state timestamps
	if st != nil {
		now := config.FormatRFC3339(time.Now())
		st.SetTimestamps(now, now)
		if err := st.Save(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
