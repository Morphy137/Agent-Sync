package rulesync

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
)

// Sync applies every non-conflicting change. Captures (two-way target edits flowing back into
// writable sources) run first; the rules are then re-evaluated so that sibling destinations of
// a captured source show up as source_changed and are written in the same pass.
func (e *Engine) Sync(all []rules.Rule) (ApplyResult, error) {
	result := ApplyResult{Backups: []string{}, CapturedRepoPaths: []string{}}

	ev := e.evaluate(all)
	captured, err := e.captureAll(ev, &result)
	if err != nil {
		return e.finish(result, err)
	}
	if captured > 0 {
		ev = e.evaluate(all)
	}

	for _, de := range ev.dests {
		switch de.state {
		case DestBlocked:
			result.Skipped++
		case DestReady:
			if err := e.applyDest(de, &result); err != nil {
				return e.finish(result, err)
			}
		}
	}
	return e.finish(result, nil)
}

// finish saves state and normalizes the result; the first error wins.
func (e *Engine) finish(result ApplyResult, err error) (ApplyResult, error) {
	sort.Strings(result.CapturedRepoPaths)
	if saveErr := e.saveState(); saveErr != nil && err == nil {
		err = saveErr
	}
	return result, err
}

type captureCandidate struct {
	de *destEval
	fe fileEval
}

// captureAll copies two-way target edits into their sources. When several destinations edited
// the same source file differently nothing is captured (they surface as conflicts afterwards).
func (e *Engine) captureAll(ev *evaluation, result *ApplyResult) (int, error) {
	groups := make(map[string][]captureCandidate)
	for _, de := range ev.dests {
		if de.state != DestReady || !de.uploadAllowed() {
			continue
		}
		for _, fe := range de.files {
			if fe.Status == StatusTargetChanged {
				key := strings.ToLower(filepath.Clean(fe.SourceAbs))
				groups[key] = append(groups[key], captureCandidate{de: de, fe: fe})
			}
		}
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	captured := 0
	for _, k := range keys {
		group := groups[k]
		if !sameTargetHash(group) {
			continue
		}
		if err := e.captureFile(group, result); err != nil {
			return captured, err
		}
		captured++
	}
	return captured, nil
}

func sameTargetHash(group []captureCandidate) bool {
	for _, c := range group[1:] {
		if c.fe.targetHash != group[0].fe.targetHash {
			return false
		}
	}
	return true
}

func (e *Engine) captureFile(group []captureCandidate, result *ApplyResult) error {
	first := group[0]
	if err := e.backupSource(first.de.rule, first.de.sourceAbs, first.fe.SourceAbs, result); err != nil {
		return err
	}
	if err := fsops.AtomicCopy(first.fe.TargetAbs, first.fe.SourceAbs); err != nil {
		return err
	}
	newHash, err := fsops.HashFile(first.fe.SourceAbs)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	for _, c := range group {
		e.setBaseline(c.de.key(), c.fe.RelPath, newHash)
	}
	result.Captured++
	if repoPath := e.repoRelPath(first.de.rule, first.fe.SourceAbs); repoPath != "" {
		result.CapturedRepoPaths = appendUnique(result.CapturedRepoPaths, repoPath)
	}
	return nil
}

func appendUnique(list []string, value string) []string {
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}

// repoRelPath returns the repo-relative forward-slash path of a repo source file, or "".
func (e *Engine) repoRelPath(r rules.Rule, abs string) string {
	if r.Source.Origin != rules.OriginRepo || e.Env.RepoRoot == "" {
		return ""
	}
	rel, err := filepath.Rel(e.Env.RepoRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// applyDest writes source changes to one ready destination.
func (e *Engine) applyDest(de *destEval, result *ApplyResult) error {
	for _, rel := range de.stale {
		e.removeBaseline(de.key(), rel)
	}
	if de.isTransform() {
		return e.applyTransformDest(de, result)
	}

	var writes, deletes []fileEval
	var backupRels []string
	for _, fe := range de.files {
		switch fe.Status {
		case StatusNewInSource, StatusSourceChanged, StatusTargetMissing:
			writes = append(writes, fe)
			if fe.targetExists {
				backupRels = append(backupRels, fe.RelPath)
			}
		case StatusSourceDeleted:
			deletes = append(deletes, fe)
			backupRels = append(backupRels, fe.RelPath)
		case StatusInSync:
			if fe.sourceHash != "" && fe.sourceHash == fe.targetHash && fe.baseline != fe.sourceHash {
				e.setBaseline(de.key(), fe.RelPath, fe.sourceHash)
			}
		case StatusIgnored:
		default:
			result.Skipped++
		}
	}

	if err := e.backupTargets(de.rule, de.dest, de.resolved.TargetRoot, backupRels, result); err != nil {
		return err
	}
	for _, fe := range writes {
		if err := e.copyToTarget(de, fe.RelPath, fe.SourceAbs, fe.TargetAbs); err != nil {
			return err
		}
		result.Written++
	}
	for _, fe := range deletes {
		if err := e.deleteTarget(de, fe.RelPath, fe.TargetAbs); err != nil {
			return err
		}
		result.Deleted++
	}
	return nil
}

func (e *Engine) copyToTarget(de *destEval, rel, src, dst string) error {
	if err := fsops.AtomicCopy(src, dst); err != nil {
		return err
	}
	hash, err := fsops.HashFile(dst)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	e.setBaseline(de.key(), rel, hash)
	return nil
}

func (e *Engine) deleteTarget(de *destEval, rel, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	e.removeBaseline(de.key(), rel)
	removeEmptyParents(filepath.Dir(dst), de.resolved.TargetRoot)
	return nil
}

// removeEmptyParents removes empty folders left by deletions, never going above root.
func removeEmptyParents(dir, root string) {
	cleanRoot := filepath.Clean(root)
	for dir = filepath.Clean(dir); dir != cleanRoot && strings.HasPrefix(dir, cleanRoot); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

func (e *Engine) applyTransformDest(de *destEval, result *ApplyResult) error {
	for _, fe := range de.files {
		switch fe.Status {
		case StatusNewInSource, StatusSourceChanged, StatusTargetMissing:
		case StatusInSync:
			if fe.baseline != "" {
				continue
			}
		case StatusIgnored:
			continue
		default:
			result.Skipped++
			continue
		}
		// Adopting identical pre-existing entries (in_sync without baseline) takes them over.
		adopt := fe.Status == StatusInSync
		wrote, err := e.renderToTarget(de, fe.RelPath, fe.SourceAbs, fe.TargetAbs, adopt, result)
		if err != nil {
			if isTransformError(err) {
				result.Skipped++
				continue
			}
			return err
		}
		if wrote {
			result.Written++
		}
	}
	return nil
}

func isTransformError(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.TransformFailed
}

const (
	entryConflictReason = "entry_conflict"
	entryParam          = "entry"
	reasonParam         = "reason"
	// maxTakeoverEntries bounds how many unowned entries an explicit use_source may take over.
	maxTakeoverEntries = 256
)

// render runs the transformer. With takeover, unowned entries that clash with source entries
// are adopted (explicit user choice) instead of failing.
func (e *Engine) render(spec rules.Transform, src, existing []byte, prev []string, takeover bool) ([]byte, []string, error) {
	if e.Transformer == nil {
		return nil, nil, apperr.New(apperr.TransformFailed).WithParam(reasonParam, "transformer_unavailable")
	}
	owned := append([]string(nil), prev...)
	for attempt := 0; ; attempt++ {
		out, managed, _, err := e.Transformer.Render(spec, src, existing, owned)
		if err == nil {
			return out, managed, nil
		}
		entry, isConflict := conflictEntry(err)
		if !takeover || !isConflict || attempt >= maxTakeoverEntries || containsString(owned, entry) {
			return nil, nil, asTransformError(err)
		}
		owned = append(owned, entry)
	}
}

func conflictEntry(err error) (string, bool) {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Params[reasonParam] != entryConflictReason {
		return "", false
	}
	entry := appErr.Params[entryParam]
	return entry, entry != ""
}

func asTransformError(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		if appErr.Code == apperr.TransformFailed {
			return appErr
		}
	}
	return apperr.Wrap(apperr.TransformFailed, err)
}

func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// renderToTarget renders the source into the destination file (backing it up first when it
// changes) and records the transform baselines. It reports whether the file was written.
func (e *Engine) renderToTarget(de *destEval, rel, srcAbs, tgtAbs string, takeover bool, result *ApplyResult) (bool, error) {
	spec := *de.dest.Transform
	src, err := os.ReadFile(srcAbs)
	if err != nil {
		return false, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", srcAbs)
	}
	existing, exists, err := readExisting(tgtAbs)
	if err != nil {
		return false, apperr.Wrap(apperr.WriteFailed, err).WithParam("path", tgtAbs)
	}
	prev := e.managedEntries(de.rule.ID, de.dest.ID)
	out, managed, err := e.render(spec, src, existing, prev, takeover)
	if err != nil {
		return false, err
	}
	srcView, err := e.Transformer.SourceView(spec, src)
	if err != nil {
		return false, asTransformError(err)
	}
	targetView, err := e.Transformer.TargetView(spec, out, managed)
	if err != nil {
		return false, asTransformError(err)
	}

	wrote := false
	if !exists || !bytes.Equal(out, existing) {
		if exists {
			if err := e.backupTargets(de.rule, de.dest, de.resolved.TargetRoot, []string{rel}, result); err != nil {
				return false, err
			}
		}
		if err := fsops.AtomicWrite(tgtAbs, out, filePerm(tgtAbs)); err != nil {
			return false, err
		}
		wrote = true
	}
	e.recordTransformBaselines(de.rule.ID, de.dest.ID, rel, hashBytes(srcView), managed, hashBytes(targetView))
	return wrote, nil
}

func filePerm(p string) os.FileMode {
	if info, err := os.Stat(p); err == nil {
		return info.Mode().Perm()
	}
	return fsops.DefaultFilePerms
}

// backupTargets snapshots destination files (relative to root) before they are overwritten or deleted.
func (e *Engine) backupTargets(r rules.Rule, d rules.Destination, root string, rels []string, result *ApplyResult) error {
	if len(rels) == 0 {
		return nil
	}
	return e.snapshot(BackupRootFor(e.BackupRoot, r.ID, d.ID), BaselineKey(r.ID, d.ID), r.Name, root, rels, result)
}

// backupSource snapshots a source file before a capture overwrites it.
func (e *Engine) backupSource(r rules.Rule, sourceRoot, fileAbs string, result *ApplyResult) error {
	root := sourceRoot
	if filepath.Clean(sourceRoot) == filepath.Clean(fileAbs) {
		root = filepath.Dir(fileAbs)
	}
	rel, err := filepath.Rel(root, fileAbs)
	if err != nil {
		return apperr.Wrap(apperr.BackupFailed, err)
	}
	return e.snapshot(BackupRootFor(e.BackupRoot, r.ID, SourceBackupID), BaselineKey(r.ID, SourceBackupID), r.Name, root, []string{filepath.ToSlash(rel)}, result)
}

func (e *Engine) snapshot(backupRoot, connID, profile, root string, rels []string, result *ApplyResult) error {
	if !anyFileExists(root, rels) {
		return nil
	}
	if e.BackupRoot == "" {
		return apperr.New(apperr.BackupFailed).WithParam(reasonParam, "backup_root_missing")
	}
	conn := config.Connection{ID: connID, Profile: profile, TargetPath: root}
	info, err := backup.CreateBackup(backupRoot, conn, nil, rels, backup.ReasonSync, e.Retention)
	if err != nil {
		return err
	}
	if info != nil && result != nil {
		result.Backups = append(result.Backups, info.Path)
	}
	return nil
}

func anyFileExists(root string, rels []string) bool {
	for _, rel := range rels {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
