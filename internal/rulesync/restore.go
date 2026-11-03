package rulesync

import (
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
)

const (
	reasonOwnerGone = "owner_gone"
	reasonLegacy    = "legacy"
)

// RestoreOutcome reports what a restore did.
type RestoreOutcome struct {
	// Root is the folder the files were restored into.
	Root     string
	Restored []string
	// UndoBackupID is the snapshot of the replaced files, empty when none of them existed.
	UndoBackupID string
}

// restoreScope is where the files of one rule backup go and how to back up what they replace.
type restoreScope struct {
	rule       rules.Rule
	ownerID    string
	backupRoot string
	root       string
	// dest is set for destination backups; nil for source backups.
	dest     *rules.Destination
	resolved rules.ResolvedDestination
}

// parseOwnerID splits a snapshot owner "rule:<ruleID>:<destID>" into its parts.
func parseOwnerID(ownerID string) (ruleID, slot string, ok bool) {
	rest, found := strings.CutPrefix(ownerID, BaselinePrefix)
	if !found {
		return "", "", false
	}
	idx := strings.LastIndex(rest, keySeparator)
	if idx <= 0 || idx == len(rest)-1 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

func notRestorable(reason string) *apperr.Error {
	return apperr.New(apperr.BackupNotRestorable).WithParam(reasonParam, reason)
}

func requireFolder(path string) error {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return apperr.New(apperr.TargetNotFound).WithParam("path", path)
	}
	return nil
}

// restoreScopeFor maps a backup owner to the folder where its files are restored today. The
// destination comes from the current rule and location mapping, never from the manifest path
// alone, so an edited manifest cannot redirect a restore.
func (e *Engine) restoreScopeFor(all []rules.Rule, ownerID, manifestRoot string) (*restoreScope, error) {
	ruleID, slot, ok := parseOwnerID(ownerID)
	if !ok {
		return nil, notRestorable(reasonLegacy)
	}
	if slot == SourceBackupID {
		return e.sourceScope(all, ruleID, ownerID, manifestRoot)
	}
	r, idx, err := findRuleDest(all, ruleID, slot)
	if err != nil {
		return nil, notRestorable(reasonOwnerGone)
	}
	d := r.Destinations[idx]
	resolved, err := rules.Resolve(e.Env, r, d)
	if err != nil {
		return nil, err
	}
	if resolved.Unmapped {
		return nil, apperr.New(apperr.LocationUnmapped).WithParam("alias", d.Location)
	}
	locationRoot, _ := rules.LocationRoot(e.Env, d.Location)
	if err := requireFolder(locationRoot); err != nil {
		return nil, err
	}
	return &restoreScope{
		rule:       r,
		ownerID:    ownerID,
		backupRoot: BackupRootFor(e.BackupRoot, r.ID, d.ID),
		root:       resolved.TargetRoot,
		dest:       &d,
		resolved:   resolved,
	}, nil
}

func (e *Engine) sourceScope(all []rules.Rule, ruleID, ownerID, manifestRoot string) (*restoreScope, error) {
	var found *rules.Rule
	for i := range all {
		if all[i].ID == ruleID {
			found = &all[i]
		}
	}
	if found == nil {
		return nil, notRestorable(reasonOwnerGone)
	}
	if !found.HasWritableSource() {
		return nil, apperr.New(apperr.SourceReadOnly).WithParam("ruleId", ruleID)
	}
	srcAbs, err := rules.SourcePath(e.Env, *found)
	if err != nil {
		return nil, err
	}
	root, ok := sourceRestoreRoot(srcAbs, manifestRoot)
	if !ok {
		return nil, notRestorable(reasonOwnerGone)
	}
	if err := requireFolder(root); err != nil {
		return nil, err
	}
	return &restoreScope{
		rule:       *found,
		ownerID:    ownerID,
		backupRoot: BackupRootFor(e.BackupRoot, found.ID, SourceBackupID),
		root:       root,
	}, nil
}

// sourceRestoreRoot picks the folder that source files are stored relative to: the source itself
// for a folder, its parent for a single file. The manifest only chooses between those two.
func sourceRestoreRoot(srcAbs, manifestRoot string) (string, bool) {
	if info, err := os.Stat(srcAbs); err == nil {
		if info.IsDir() {
			return srcAbs, true
		}
		return filepath.Dir(srcAbs), true
	}
	for _, candidate := range []string{srcAbs, filepath.Dir(srcAbs)} {
		if strings.EqualFold(filepath.Clean(candidate), filepath.Clean(manifestRoot)) {
			return candidate, true
		}
	}
	return "", false
}

// RestoreFiles puts files of a backup back where they came from. The current versions are
// backed up first (without pruning, so the snapshot being restored survives), and the sync
// baselines are updated so the next sync never overwrites a restored file without review.
func (e *Engine) RestoreFiles(all []rules.Rule, entry backup.Entry, rels []string) (RestoreOutcome, error) {
	scope, err := e.restoreScopeFor(all, entry.OwnerID, entry.TargetPath)
	if err != nil {
		return RestoreOutcome{}, err
	}
	files, err := backup.PrepareRestore(entry, scope.root, rels)
	if err != nil {
		return RestoreOutcome{}, err
	}
	if e.BackupRoot == "" {
		return RestoreOutcome{}, apperr.New(apperr.BackupFailed).WithParam(reasonParam, "backup_root_missing")
	}

	outcome := RestoreOutcome{Root: scope.root, Restored: rels}
	owner := backup.Owner{ID: scope.ownerID, Name: scope.rule.Name, Dir: scope.root}
	undo, err := backup.CreateSnapshot(scope.backupRoot, owner, rels, backup.ReasonRestore, 0)
	if err != nil {
		return RestoreOutcome{}, err
	}
	if undo != nil {
		outcome.UndoBackupID = relativeBackupID(e.BackupRoot, undo.Path)
	}

	copyErr := backup.ApplyRestore(files)
	e.settleRestored(scope, rels)
	if err := e.saveState(); err != nil && copyErr == nil {
		copyErr = err
	}
	return outcome, copyErr
}

func relativeBackupID(backupRoot, snapshotDir string) string {
	rel, err := filepath.Rel(backupRoot, snapshotDir)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// settleRestored keeps the baselines honest after files were put back. A restored destination
// file that differs from the source loses its baseline, so it shows as a difference to review
// (never as an automatic source update or capture); one identical to the source is recorded as
// synced. Transform destinations keep their baselines: a restored file whose managed entries
// differ from the last write is reported as drift, which a sync never overwrites. Source
// restores leave destination baselines alone, so the restored source appears as a pending
// source change in the next sync preview.
func (e *Engine) settleRestored(scope *restoreScope, rels []string) {
	if scope.dest == nil || scope.dest.Transform != nil {
		return
	}
	key := BaselineKey(scope.rule.ID, scope.dest.ID)
	pairs := make(map[string]rules.Pair, len(scope.resolved.Pairs))
	for _, p := range scope.resolved.Pairs {
		pairs[p.RelPath] = p
	}
	for _, rel := range rels {
		pair, managed := pairs[rel]
		if !managed {
			e.removeBaseline(key, rel)
			continue
		}
		restoredHash, err := fsops.HashFile(pair.TargetAbs)
		if err != nil {
			continue
		}
		if sourceHash, err := fsops.HashFile(pair.SourceAbs); err == nil && sourceHash == restoredHash {
			e.setBaseline(key, rel, restoredHash)
			continue
		}
		e.removeBaseline(key, rel)
	}
}
