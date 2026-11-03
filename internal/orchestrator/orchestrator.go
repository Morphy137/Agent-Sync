package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/state"
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

// Resolution choices for a conflicted repository file.
const (
	ResolutionKeepRepo  = "keep_repo"
	ResolutionKeepLocal = "keep_local"
)

// Conflict reasons.
const (
	ConflictRemoteAndLocalOverlap = "remote_and_local_overlap"
)

// ConflictItem represents an unresolved conflict on a repository file.
type ConflictItem struct {
	Path   string `json:"path"` // repo-relative, forward slashes
	Reason string `json:"reason"`
}

// ConflictResolution represents the user's decision for a conflicted repository file.
type ConflictResolution struct {
	Path       string `json:"path"`
	Resolution string `json:"resolution"` // "keep_repo" or "keep_local"
}

// SyncPlan holds the calculated sync operations without side-effects.
type SyncPlan struct {
	Fingerprint        string         `json:"fingerprint"`
	Direction          string         `json:"direction"`
	Conflicts          []ConflictItem `json:"conflicts"`
	CommitPreview      []string       `json:"commitPreview"`
	Ahead              int            `json:"ahead"`
	Behind             int            `json:"behind"`
	HasRemote          bool           `json:"hasRemote"`
	IsOffline          bool           `json:"isOffline"`
	DirtyOutside       []string       `json:"dirtyOutside"`
	DirtyOverlapExists bool           `json:"dirtyOverlapExists"`
	LastCheckAt        string         `json:"lastCheckAt,omitempty"`
	RulesPending       int            `json:"rulesPending"`
	RulesConflicts     int            `json:"rulesConflicts"`
}

// SyncResult summarizes the outcome of ExecuteSync.
type SyncResult struct {
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
	// Changes under them are committed by Sync instead of being reported as DirtyOutside.
	ManagedRepoPaths []string
}

// isManagedRepoPath reports whether a repo-relative path belongs to a sync rule.
func (o *Orchestrator) isManagedRepoPath(repoPath string) bool {
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

	confs := make([]string, len(plan.Conflicts))
	for i, c := range plan.Conflicts {
		confs[i] = fmt.Sprintf("%s:%s", c.Path, c.Reason)
	}
	sort.Strings(confs)

	commits := make([]string, len(plan.CommitPreview))
	copy(commits, plan.CommitPreview)
	sort.Strings(commits)

	dirty := make([]string, len(plan.DirtyOutside))
	copy(dirty, plan.DirtyOutside)
	sort.Strings(dirty)

	fmt.Fprintf(h, "cf:%s|cp:%s|do:%s|a:%d|b:%d",
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

// FetchRemote fetches the root repository's remote and returns the failure, if any. A root that is
// not a repository or has no remote needs no fetch and reports success. It reads no config or state,
// so callers can run it without holding the service lock and pass the result to PlanSyncAfterFetch.
func FetchRemote(rootPath string) error {
	if rootPath == "" || !gitops.IsRepo(rootPath) {
		return nil
	}
	runner := gitops.NewRunner(rootPath)
	if remoteURL, _ := runner.RemoteURL(); remoteURL == "" {
		return nil
	}
	return runner.Fetch()
}

// PlanSyncWithOptions calculates the synchronization plan, optionally fetching from the remote.
func (o *Orchestrator) PlanSyncWithOptions(cfg *config.Config, st *state.State, fetch bool) (*SyncPlan, error) {
	return o.planSync(cfg, st, func(runner *gitops.GitRunner) bool {
		return fetch && runner.Fetch() != nil
	})
}

// PlanSyncAfterFetch calculates the plan from a fetch already done with FetchRemote, without any
// network access. A failed fetch marks the plan as offline.
func (o *Orchestrator) PlanSyncAfterFetch(cfg *config.Config, st *state.State, fetchErr error) (*SyncPlan, error) {
	return o.planSync(cfg, st, func(*gitops.GitRunner) bool {
		return fetchErr != nil
	})
}

// planSync builds the plan. fetchOffline runs only for repositories with a remote and reports
// whether the remote could not be reached.
func (o *Orchestrator) planSync(cfg *config.Config, st *state.State, fetchOffline func(*gitops.GitRunner) bool) (*SyncPlan, error) {
	if cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}

	plan := &SyncPlan{
		Direction:     DirectionIdle,
		Conflicts:     make([]ConflictItem, 0),
		CommitPreview: make([]string, 0),
		DirtyOutside:  make([]string, 0),
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

		if plan.HasRemote {
			plan.IsOffline = fetchOffline(runner)
		}

		ahead, behind, _ := runner.AheadBehind()
		plan.Ahead = ahead
		plan.Behind = behind
	}

	// 1. Check git status porcelain and incoming changed files if repository
	if isRepo && runner != nil {
		changes, err := runner.StatusPorcelain()
		if err == nil {
			for _, ch := range changes {
				if ch.Path == ".gitignore" {
					continue
				}
				if o.isManagedRepoPath(ch.Path) {
					plan.CommitPreview = append(plan.CommitPreview, ch.Path)
				} else {
					plan.DirtyOutside = append(plan.DirtyOutside, ch.Path)
				}
			}
		}

		// Detect overlaps between incoming remote changes and local edits (CommitPreview, DirtyOutside)
		incoming, err := runner.IncomingChangedFiles()
		if err == nil && len(incoming) > 0 {
			commitSet := make(map[string]bool, len(plan.CommitPreview))
			for _, cp := range plan.CommitPreview {
				commitSet[cp] = true
			}
			dirtyOutsideSet := make(map[string]bool, len(plan.DirtyOutside))
			for _, do := range plan.DirtyOutside {
				dirtyOutsideSet[do] = true
			}

			for _, inPath := range incoming {
				if dirtyOutsideSet[inPath] {
					plan.DirtyOverlapExists = true
				}
				if commitSet[inPath] {
					plan.Conflicts = append(plan.Conflicts, ConflictItem{
						Path:   inPath,
						Reason: ConflictRemoteAndLocalOverlap,
					})
				}
			}
		}
	}

	// 2. Calculate Direction
	hasUploads := len(plan.CommitPreview) > 0 || plan.Ahead > 0
	hasDownloads := plan.Behind > 0

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

// ExecuteSync commits the pending edits inside rule-managed repository paths, pulls, and pushes,
// under the Service lock. Rules are applied by the caller afterwards, on the freshly pulled content.
func (o *Orchestrator) ExecuteSync(cfg *config.Config, st *state.State, expectedFingerprint string, resolutions []ConflictResolution) (*SyncResult, error) {
	// Step 1: Revalidate plan with fetch=true
	plan, err := o.PlanSyncWithOptions(cfg, st, true)
	if err != nil {
		return nil, err
	}
	if expectedFingerprint != "" && plan.Fingerprint != expectedFingerprint {
		return nil, apperr.New(apperr.PlanChanged)
	}

	// Step 2: Every conflict needs a decision
	keepRepoSet, keepLocalSet, err := conflictDecisions(plan.Conflicts, resolutions)
	if err != nil {
		return nil, err
	}

	isRepo := gitops.IsRepo(cfg.RootPath)
	var runner *gitops.GitRunner

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "local"
	}

	result := &SyncResult{
		BackupSessions: make([]string, 0),
	}

	// Step 3: Git preflight
	if isRepo {
		runner = gitops.NewRunner(cfg.RootPath)
		runner.OnBeforePush = o.OnBeforePush

		if plan.DirtyOverlapExists {
			return nil, apperr.New(apperr.GitDirtyWorktree)
		}

		if len(plan.CommitPreview) > 0 {
			if err := runner.EnsureIdentity(); err != nil {
				return nil, err
			}
		}

		// Step 4: Commit pending manual edits inside rule-managed paths (CommitPreview)
		if err := o.commitPendingEdits(cfg, runner, plan.CommitPreview, keepRepoSet, keepLocalSet, hostname, result); err != nil {
			return nil, err
		}
	}

	// Local versions kept over incoming changes are set aside while pulling and re-applied after.
	var keptLocal []keptEdit
	if isRepo && runner != nil && len(keepLocalSet) > 0 {
		keptLocal, err = setAsideLocalEdits(cfg, runner, plan.CommitPreview, keepLocalSet, result)
		if err != nil {
			return nil, err
		}
	}

	// Step 5: Pull first (if behind > 0)
	if isRepo && runner != nil && runner.HasUpstream() {
		ahead, behind, _ := runner.AheadBehind()
		if behind > 0 {
			if err := pullBehind(runner, ahead); err != nil {
				restoreKeptEdits(cfg.RootPath, keptLocal)
				return nil, err
			}
			result.Pulled = behind
		}
	}
	if err := commitKeptEdits(cfg.RootPath, runner, keptLocal, hostname); err != nil {
		return nil, err
	}

	// Step 6: Push with one retry on GitPushRejected (H3)
	if isRepo && runner != nil && runner.HasUpstream() {
		ahead, _, _ := runner.AheadBehind()
		if ahead > 0 {
			if err := pushWithRetry(runner); err != nil {
				return nil, err
			}
			result.Pushed = ahead
		}
	}

	// Step 7: Persist state timestamps
	if st != nil {
		now := config.FormatRFC3339(time.Now())
		st.SetTimestamps(now, now)
		if err := st.Save(); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// pullBehind brings in the remote commits: a fast-forward when nothing is ahead, a rebase otherwise.
func pullBehind(runner *gitops.GitRunner, ahead int) error {
	if ahead == 0 {
		return runner.PullFastForward()
	}
	return runner.PullRebase()
}

// conflictDecisions checks that every conflict has a valid resolution and returns the sets of paths
// where the user chose to keep the repository version and the local version.
func conflictDecisions(conflicts []ConflictItem, resolutions []ConflictResolution) (keepRepoSet, keepLocalSet map[string]bool, err error) {
	keepRepoSet = make(map[string]bool)
	keepLocalSet = make(map[string]bool)
	if len(conflicts) == 0 {
		return keepRepoSet, keepLocalSet, nil
	}

	chosen := make(map[string]string, len(resolutions))
	for _, r := range resolutions {
		chosen[r.Path] = r.Resolution
	}
	for _, conflict := range conflicts {
		res := chosen[conflict.Path]
		switch res {
		case ResolutionKeepRepo:
			keepRepoSet[conflict.Path] = true
		case ResolutionKeepLocal:
			keepLocalSet[conflict.Path] = true
		default:
			return nil, nil, apperr.New(apperr.UnresolvedConflicts).WithParam("file", conflict.Path)
		}
	}
	return keepRepoSet, keepLocalSet, nil
}

// commitPendingEdits commits the local edits in rule-managed paths. Edits where the user kept the
// repository version are backed up and discarded instead, and edits kept over incoming changes wait
// until after the pull (see setAsideLocalEdits).
func (o *Orchestrator) commitPendingEdits(cfg *config.Config, runner *gitops.GitRunner, pending []string, keepRepoSet, keepLocalSet map[string]bool, hostname string, result *SyncResult) error {
	var pendingToCommit []string
	var filesToDiscard []string
	for _, path := range pending {
		switch {
		case keepRepoSet[path]:
			filesToDiscard = append(filesToDiscard, path)
		case keepLocalSet[path]:
			continue
		default:
			pendingToCommit = append(pendingToCommit, path)
		}
	}

	if len(filesToDiscard) > 0 {
		repoBackup, err := backup.CreateRepoBackup(cfg.BackupPath, cfg.RootPath, filesToDiscard, backup.ReasonDiscardRepoEdit, cfg.BackupRetention)
		if err != nil {
			return err
		}
		if repoBackup != nil {
			result.BackupSessions = append(result.BackupSessions, repoBackup.ID)
		}
		for _, path := range filesToDiscard {
			if _, err := runner.DiscardPath(path); err != nil {
				return err
			}
		}
	}

	if len(pendingToCommit) > 0 {
		msg := fmt.Sprintf("chore(sync): commit pending changes from %s", hostname)
		return runner.CommitPaths(pendingToCommit, msg)
	}
	return nil
}

// pushWithRetry pushes and, when the remote moved in the meantime, rebases and pushes once more.
func pushWithRetry(runner *gitops.GitRunner) error {
	err := runner.Push()
	if err == nil {
		return nil
	}
	if !apperr.HasCode(err, apperr.GitPushRejected) {
		return err
	}
	if rbErr := runner.PullRebase(); rbErr != nil {
		return rbErr
	}
	return runner.Push()
}

// keptEdit is a local edit set aside while the remote changes are pulled. A nil content means the
// file was deleted locally.
type keptEdit struct {
	path    string
	content []byte
}

// setAsideLocalEdits backs up the local versions the user chose to keep over incoming changes and
// reverts them in the worktree so the pull cannot collide with them. commitKeptEdits puts them back.
func setAsideLocalEdits(cfg *config.Config, runner *gitops.GitRunner, pending []string, keepLocalSet map[string]bool, result *SyncResult) ([]keptEdit, error) {
	var paths []string
	for _, path := range pending {
		if keepLocalSet[path] {
			paths = append(paths, path)
		}
	}

	snapshot, err := backup.CreateRepoBackup(cfg.BackupPath, cfg.RootPath, paths, backup.ReasonSync, cfg.BackupRetention)
	if err != nil {
		return nil, err
	}
	if snapshot != nil {
		result.BackupSessions = append(result.BackupSessions, snapshot.ID)
	}

	kept := make([]keptEdit, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(cfg.RootPath, filepath.FromSlash(path)))
		if err != nil && !os.IsNotExist(err) {
			return nil, apperr.Wrap(apperr.WriteFailed, err)
		}
		kept = append(kept, keptEdit{path: path, content: content})
	}
	for _, edit := range kept {
		if _, err := runner.DiscardPath(edit.path); err != nil {
			restoreKeptEdits(cfg.RootPath, kept)
			return nil, err
		}
	}
	return kept, nil
}

// restoreKeptEdits writes the set-aside edits back into the worktree without committing them.
func restoreKeptEdits(rootPath string, kept []keptEdit) {
	for _, edit := range kept {
		full := filepath.Join(rootPath, filepath.FromSlash(edit.path))
		if edit.content == nil {
			_ = os.Remove(full)
			continue
		}
		_ = fsops.AtomicWrite(full, edit.content, fsops.DefaultFilePerms)
	}
}

// commitKeptEdits re-applies the set-aside edits on top of the pulled content and commits them.
func commitKeptEdits(rootPath string, runner *gitops.GitRunner, kept []keptEdit, hostname string) error {
	if len(kept) == 0 {
		return nil
	}
	restoreKeptEdits(rootPath, kept)
	paths := make([]string, len(kept))
	for i, edit := range kept {
		paths[i] = edit.path
	}
	return runner.CommitPaths(paths, fmt.Sprintf("chore(sync): keep local changes from %s", hostname))
}
