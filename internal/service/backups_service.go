package service

import (
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/rulesync"
)

const (
	reasonParamKey    = "reason"
	reasonRepoChanged = "repo_changed"
	destinationJoiner = "/"
	currentDestPath   = "."
)

// BackupEntry is one backup shown in the list: a snapshot taken before a file was overwritten,
// deleted or discarded.
type BackupEntry struct {
	ID string `json:"id"`
	// Kind is rule (destination files), source (a rule source), repo (discarded repository edits)
	// or legacy (snapshots of earlier versions, which cannot be restored).
	Kind string `json:"kind"`
	// Owner is the rule name (current when the rule still exists), or the repository marker.
	Owner string `json:"owner"`
	// Destination is where the files go, e.g. "claude/skills"; empty when unknown.
	Destination string `json:"destination"`
	RuleID      string `json:"ruleId"`
	DestID      string `json:"destId"`
	// TargetPath is the folder the files were taken from when the backup was made.
	TargetPath string   `json:"targetPath"`
	CreatedAt  string   `json:"createdAt"`
	Reason     string   `json:"reason"`
	FileCount  int      `json:"fileCount"`
	TotalSize  int64    `json:"totalSize"`
	Files      []string `json:"files"`
	// Path is the backup folder on disk.
	Path string `json:"path"`
	// Restorable is false when the owner no longer exists or the backup predates sync rules.
	Restorable bool `json:"restorable"`
}

// RestoreBackupResult reports a finished restore.
type RestoreBackupResult struct {
	Restored []string `json:"restored"`
	// Target is the folder the files went to.
	Target string `json:"target"`
	// UndoBackupID is the backup of the replaced files; empty when nothing was replaced.
	UndoBackupID string `json:"undoBackupId"`
}

// ruleLabels are the current names of a rule and its destinations.
type ruleLabels struct {
	name  string
	dests map[string]string
}

func destinationLabel(location, path string) string {
	if path == "" || path == currentDestPath {
		return location
	}
	return location + destinationJoiner + path
}

func (s *Service) ruleLabelsLocked() map[string]ruleLabels {
	labels := map[string]ruleLabels{}
	all, err := s.allRulesLocked()
	if err != nil {
		return labels
	}
	for _, r := range all {
		entry := ruleLabels{name: r.Name, dests: map[string]string{}}
		for _, d := range r.Destinations {
			entry.dests[d.ID] = destinationLabel(d.Location, d.Path)
		}
		labels[r.ID] = entry
	}
	return labels
}

// ListBackups returns every backup under the backup folder, newest first. Unreadable entries are skipped.
func (s *Service) ListBackups() ([]BackupEntry, error) {
	s.mu.Lock()
	backupRoot := s.cfg.BackupPath
	repoSet := s.cfg.RootPath != ""
	labels := s.ruleLabelsLocked()
	s.mu.Unlock()

	if backupRoot == "" {
		return []BackupEntry{}, nil
	}
	entries, err := backup.ListAll(backupRoot)
	if err != nil {
		return nil, err
	}
	out := make([]BackupEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, toBackupEntry(e, labels, repoSet))
	}
	return out, nil
}

func toBackupEntry(e backup.Entry, labels map[string]ruleLabels, repoSet bool) BackupEntry {
	item := BackupEntry{
		ID:         e.ID,
		Kind:       e.Kind,
		Owner:      e.Name,
		TargetPath: e.TargetPath,
		CreatedAt:  e.CreatedAt,
		Reason:     e.Reason,
		FileCount:  len(e.Files),
		TotalSize:  e.Size,
		Files:      e.Files,
		Path:       e.Path,
	}
	switch e.Kind {
	case backup.KindRepo:
		item.Restorable = repoSet
	case backup.KindRule, backup.KindSource:
		fillRuleOwner(&item, e, labels)
	}
	return item
}

// fillRuleOwner takes the current rule and destination names when they still exist.
func fillRuleOwner(item *BackupEntry, e backup.Entry, labels map[string]ruleLabels) {
	ruleID, slot, ok := splitRuleOwner(e.OwnerID)
	if !ok {
		return
	}
	item.RuleID = ruleID
	rule, ruleExists := labels[ruleID]
	if ruleExists {
		item.Owner = rule.name
	}
	if e.Kind == backup.KindSource {
		item.Restorable = ruleExists
		return
	}
	item.DestID = slot
	label, destExists := rule.dests[slot]
	item.Destination = label
	item.Restorable = ruleExists && destExists
}

func splitRuleOwner(ownerID string) (ruleID, slot string, ok bool) {
	rest, found := strings.CutPrefix(ownerID, rulesync.BaselinePrefix)
	if !found {
		return "", "", false
	}
	idx := strings.LastIndex(rest, ":")
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

// RestoreBackup puts the files of a backup (all of them when files is empty) back where they came
// from. The current versions are backed up first so the restore can be undone from the same list.
// Restored destination files differ from their source, so rules report them as differences to
// review instead of overwriting them on the next sync (see docs/SYNC_RULES.md).
func (s *Service) RestoreBackup(id string, files []string) (*RestoreBackupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.BackupPath == "" {
		return nil, apperr.New(apperr.BackupNotFound).WithParam("id", id)
	}
	entry, err := backup.Open(s.cfg.BackupPath, id)
	if err != nil {
		return nil, err
	}
	rels, err := entry.Select(files)
	if err != nil {
		return nil, err
	}

	var outcome rulesync.RestoreOutcome
	switch entry.Kind {
	case backup.KindRepo:
		outcome, err = s.restoreRepoFilesLocked(entry, rels)
	case backup.KindRule, backup.KindSource:
		all, rulesErr := s.allRulesLocked()
		if rulesErr != nil {
			return nil, rulesErr
		}
		outcome, err = s.engineLocked().RestoreFiles(all, entry, rels)
	default:
		return nil, apperr.New(apperr.BackupNotRestorable).WithParam(reasonParamKey, backup.KindLegacy)
	}
	if err != nil {
		return nil, err
	}

	s.emitStatusLocked()
	s.emitRulesChangedLocked()
	return &RestoreBackupResult{Restored: outcome.Restored, Target: outcome.Root, UndoBackupID: outcome.UndoBackupID}, nil
}

// restoreRepoFilesLocked puts discarded repository edits back into the working tree, where they
// show up as uncommitted changes again.
func (s *Service) restoreRepoFilesLocked(entry backup.Entry, rels []string) (rulesync.RestoreOutcome, error) {
	root := s.cfg.RootPath
	if root == "" {
		return rulesync.RestoreOutcome{}, apperr.New(apperr.RootNotFound)
	}
	if !samePath(root, entry.TargetPath) {
		return rulesync.RestoreOutcome{}, apperr.New(apperr.BackupNotRestorable).WithParam(reasonParamKey, reasonRepoChanged)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return rulesync.RestoreOutcome{}, apperr.New(apperr.RootNotFound)
	}
	files, err := backup.PrepareRestore(entry, root, rels)
	if err != nil {
		return rulesync.RestoreOutcome{}, err
	}
	outcome := rulesync.RestoreOutcome{Root: root, Restored: rels}
	undo, err := backup.CreateRepoBackup(s.cfg.BackupPath, root, rels, backup.ReasonRestore, 0)
	if err != nil {
		return outcome, err
	}
	if undo != nil {
		if rel, relErr := filepath.Rel(s.cfg.BackupPath, undo.Path); relErr == nil {
			outcome.UndoBackupID = filepath.ToSlash(rel)
		}
	}
	return outcome, backup.ApplyRestore(files)
}
