package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	Direction     string                    `json:"direction"`
	Downloads     []SyncItem                `json:"downloads"`
	Uploads       []SyncItem                `json:"uploads"`
	Conflicts     []ConflictItem            `json:"conflicts"`
	CommitPreview []string                  `json:"commitPreview"`
	Ahead         int                       `json:"ahead"`
	Behind        int                       `json:"behind"`
	HasRemote     bool                      `json:"hasRemote"`
	IsOffline     bool                      `json:"isOffline"`
	DirtyOutside  []string                  `json:"dirtyOutside"`
	Connections   []syncer.ConnectionStatus `json:"connections"`
}

// SyncResult summarizes the outcome of ExecuteSync.
type SyncResult struct {
	Applied        int      `json:"applied"`
	Captured       int      `json:"captured"`
	Pulled         int      `json:"pulled"`
	Pushed         int      `json:"pushed"`
	BackupSessions []string `json:"backupSessions"`
}

// Orchestrator coordinates the end-to-end sync workflow.
type Orchestrator struct {
	mu sync.Mutex
}

// New creates a new Orchestrator instance.
func New() *Orchestrator {
	return &Orchestrator{}
}

// TryLock attempts to acquire the sync mutex without blocking.
func (o *Orchestrator) TryLock() bool {
	return o.mu.TryLock()
}

// Unlock releases the sync mutex.
func (o *Orchestrator) Unlock() {
	o.mu.Unlock()
}

// PlanSync calculates the required synchronization actions without performing mutations.
func (o *Orchestrator) PlanSync(cfg *config.Config, st *state.State) (*SyncPlan, error) {
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

	isRepo := gitops.IsRepo(cfg.RootPath)
	var runner *gitops.GitRunner
	if isRepo {
		runner = gitops.NewRunner(cfg.RootPath)
		remoteURL, _ := runner.RemoteURL()
		plan.HasRemote = remoteURL != ""

		if plan.HasRemote {
			if err := runner.Fetch(); err != nil {
				plan.IsOffline = true
			}
		}

		ahead, behind, _ := runner.AheadBehind()
		plan.Ahead = ahead
		plan.Behind = behind
	}

	// 1. Inspect each connection and collect file statuses
	connMap := make(map[string]config.Connection)
	for _, conn := range cfg.Connections {
		connMap[conn.ID] = conn
		status, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
		if err != nil {
			continue
		}
		plan.Connections = append(plan.Connections, status)

		for _, file := range status.Files {
			fullPath := filepath.ToSlash(filepath.Join(conn.Profile, file.RelPath))
			switch file.Status {
			case syncer.FileConflict:
				plan.Conflicts = append(plan.Conflicts, ConflictItem{
					Profile: conn.Profile,
					RelPath: file.RelPath,
					Reason:  "conflict",
				})
			case syncer.FileUntrackedDiff:
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
				// Check if this path is within one of the profiles
				inProfile := false
				for _, conn := range cfg.Connections {
					prefix := conn.Profile + "/"
					if strings.HasPrefix(ch.Path, prefix) {
						inProfile = true
						break
					}
				}
				if inProfile {
					plan.CommitPreview = append(plan.CommitPreview, ch.Path)
				} else {
					plan.DirtyOutside = append(plan.DirtyOutside, ch.Path)
				}
			}
		}

		// Detect overlaps between incoming remote changes and local uploads
		incoming, err := runner.IncomingChangedFiles()
		if err == nil && len(incoming) > 0 {
			uploadMap := make(map[string]bool)
			for _, up := range plan.Uploads {
				uploadMap[up.FullPath] = true
			}
			for _, inPath := range incoming {
				if uploadMap[inPath] {
					parts := strings.SplitN(inPath, "/", 2)
					profile := parts[0]
					rel := ""
					if len(parts) > 1 {
						rel = parts[1]
					}
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

	return plan, nil
}

// ExecuteSync performs the full synchronization workflow.
func (o *Orchestrator) ExecuteSync(cfg *config.Config, st *state.State, plan *SyncPlan, resolutions []ConflictResolution) (*SyncResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. Validate and apply conflict resolutions
	resolvedDownloads := make([]SyncItem, 0)
	resolvedUploads := make([]SyncItem, 0)

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
			} else {
				resolvedUploads = append(resolvedUploads, item)
			}
		}
	}

	allUploads := append(plan.Uploads, resolvedUploads...)
	allDownloads := append(plan.Downloads, resolvedDownloads...)

	result := &SyncResult{
		BackupSessions: make([]string, 0),
	}

	connByProfile := make(map[string]config.Connection)
	for _, c := range cfg.Connections {
		connByProfile[c.Profile] = c
	}

	// 2. Local Upload Phase: Capture target files to source
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

	// 3. Git commit of uploaded changes and uncommitted profile files
	isRepo := gitops.IsRepo(cfg.RootPath)
	var runner *gitops.GitRunner
	if isRepo {
		runner = gitops.NewRunner(cfg.RootPath)
		var filesToCommit []string
		for _, up := range allUploads {
			filesToCommit = append(filesToCommit, up.FullPath)
		}
		filesToCommit = append(filesToCommit, plan.CommitPreview...)

		if len(filesToCommit) > 0 {
			hostname, err := os.Hostname()
			if err != nil || hostname == "" {
				hostname = "local"
			}
			sort.Strings(committedProfiles)
			profList := strings.Join(committedProfiles, ", ")
			if profList == "" {
				profList = "profiles"
			}
			msg := fmt.Sprintf("chore(sync): update %s from %s", profList, hostname)
			_ = runner.CommitPaths(filesToCommit, msg)
		}

		// 4. Remote Pull Phase
		ahead, behind, _ := runner.AheadBehind()
		if behind > 0 && runner.HasUpstream() {
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

			// Step 4 from spec: Recalculate status after pull as source files changed
			allDownloads = make([]SyncItem, 0)
			for _, conn := range cfg.Connections {
				statusAfterPull, err := syncer.ComputeStatus(cfg.RootPath, conn, st)
				if err != nil {
					continue
				}
				for _, file := range statusAfterPull.Files {
					if file.Status == syncer.FileSourceChanged || file.Status == syncer.FileNewInSource || file.Status == syncer.FileTargetMissing {
						fullPath := filepath.ToSlash(filepath.Join(conn.Profile, file.RelPath))
						allDownloads = append(allDownloads, SyncItem{
							Profile:  conn.Profile,
							RelPath:  file.RelPath,
							FullPath: fullPath,
						})
					}
				}
			}
		}
	}

	// 5. Local Apply Phase: Download source files to targets with backup
	downloadsByProfile := make(map[string][]string)
	for _, dl := range allDownloads {
		downloadsByProfile[dl.Profile] = append(downloadsByProfile[dl.Profile], dl.RelPath)
	}

	for prof, files := range downloadsByProfile {
		conn, ok := connByProfile[prof]
		if !ok {
			continue
		}
		if err := syncer.Apply(cfg.RootPath, conn, cfg.Connections, cfg.BackupPath, files, st, cfg.BackupRetention, backup.ReasonSync); err != nil {
			return nil, err
		}
		result.Applied += len(files)
		result.BackupSessions = append(result.BackupSessions, conn.ID)
	}

	// 6. Git Push Phase
	if isRepo && runner != nil && runner.HasUpstream() {
		ahead, _, _ := runner.AheadBehind()
		if ahead > 0 {
			if err := runner.Push(); err != nil {
				return nil, err
			}
			result.Pushed = ahead
		}
	}

	// 7. Update state timestamps
	now := config.FormatRFC3339(time.Now())
	st.SetTimestamps(now, now)
	_ = st.Save()

	return result, nil
}
