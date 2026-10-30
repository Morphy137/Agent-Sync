package rulesync

import (
	"os"
	"path/filepath"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
)

// findRuleDest locates a rule and one of its destinations by ID.
func findRuleDest(all []rules.Rule, ruleID, destID string) (rules.Rule, int, error) {
	for _, r := range all {
		if r.ID != ruleID {
			continue
		}
		for i, d := range r.Destinations {
			if d.ID == destID {
				return r, i, nil
			}
		}
		return rules.Rule{}, 0, apperr.New(apperr.RuleNotFound).WithParam("ruleId", ruleID).WithParam("destId", destID)
	}
	return rules.Rule{}, 0, apperr.New(apperr.RuleNotFound).WithParam("ruleId", ruleID)
}

// locate resolves the destination and the pair for relPath. A file removed from a folder
// source is synthesized so that source_deleted conflicts can still be resolved.
func (e *Engine) locate(all []rules.Rule, ruleID, destID, relPath string) (*destEval, rules.Pair, error) {
	r, idx, err := findRuleDest(all, ruleID, destID)
	if err != nil {
		return nil, rules.Pair{}, err
	}
	d := r.Destinations[idx]
	rel, err := rules.NormalizeRelPath(relPath)
	if err != nil || rel == "." {
		return nil, rules.Pair{}, apperr.New(apperr.InvalidRule).WithParam(reasonParam, rules.ReasonInvalidRelPath).WithParam("path", relPath)
	}
	resolved, err := rules.Resolve(e.Env, r, d)
	if err != nil {
		return nil, rules.Pair{}, err
	}
	if resolved.Unmapped {
		return nil, rules.Pair{}, apperr.New(apperr.LocationUnmapped).WithParam("alias", d.Location)
	}
	srcAbs, err := rules.SourcePath(e.Env, r)
	if err != nil {
		return nil, rules.Pair{}, err
	}
	de := &destEval{plannedDest: plannedDest{rule: r, dest: d, resolved: resolved, sourceAbs: srcAbs, active: r.Enabled && d.Enabled}}

	for _, p := range resolved.Pairs {
		if p.RelPath == rel {
			return de, p, nil
		}
	}
	if resolved.SourceMissing || !resolved.SourceIsDir {
		return nil, rules.Pair{}, apperr.New(apperr.SourceNotFound).WithParam("path", relPath)
	}
	pairSrc := filepath.Join(srcAbs, filepath.FromSlash(rel))
	if _, statErr := os.Lstat(pairSrc); statErr == nil {
		// The file exists but is excluded or ignored: never expose it through a resolution.
		return nil, rules.Pair{}, apperr.New(apperr.SourceNotFound).WithParam("path", relPath)
	}
	return de, rules.Pair{
		RelPath:   rel,
		SourceAbs: pairSrc,
		TargetAbs: filepath.Join(resolved.TargetRoot, filepath.FromSlash(rel)),
	}, nil
}

// isBlocked reports whether the destination is involved in a collision.
func (e *Engine) isBlocked(all []rules.Rule, ruleID, destID string) bool {
	planned, _ := e.plan(all)
	for _, pd := range planned {
		if pd.rule.ID == ruleID && pd.dest.ID == destID {
			return pd.blocked
		}
	}
	return false
}

// Resolve applies one resolution. It returns the modified rule when Choice == KeepLocal
// (the caller saves it), plus the repo-relative path touched when the source is in the repo.
func (e *Engine) Resolve(all []rules.Rule, res Resolution) (*rules.Rule, string, error) {
	switch res.Choice {
	case KeepLocal:
		return e.keepLocal(all, res)
	case UseSource, UseTarget, UseCustom:
	default:
		return nil, "", apperr.New(apperr.InvalidRule).WithParam(reasonParam, "invalid_choice").WithParam("choice", res.Choice)
	}

	de, pair, err := e.locate(all, res.RuleID, res.DestID, res.RelPath)
	if err != nil {
		return nil, "", err
	}
	if !de.active {
		return nil, "", apperr.New(apperr.InvalidRule).WithParam(reasonParam, "destination_disabled")
	}
	if e.isBlocked(all, res.RuleID, res.DestID) {
		return nil, "", apperr.New(apperr.CollisionDetected).WithParam("ruleId", res.RuleID).WithParam("destId", res.DestID)
	}

	repoPath := ""
	switch res.Choice {
	case UseSource:
		err = e.resolveUseSource(de, pair)
	case UseTarget:
		err = e.resolveUseTarget(de, pair)
		repoPath = e.repoRelPath(de.rule, pair.SourceAbs)
	case UseCustom:
		err = e.resolveCustom(de, pair, []byte(res.Content))
		repoPath = e.repoRelPath(de.rule, pair.SourceAbs)
	}
	if err != nil {
		_ = e.saveState()
		return nil, "", err
	}
	if err := e.saveState(); err != nil {
		return nil, "", err
	}
	return nil, repoPath, nil
}

func (e *Engine) keepLocal(all []rules.Rule, res Resolution) (*rules.Rule, string, error) {
	r, idx, err := findRuleDest(all, res.RuleID, res.DestID)
	if err != nil {
		return nil, "", err
	}
	rel, err := rules.NormalizeRelPath(res.RelPath)
	if err != nil || rel == "." {
		return nil, "", apperr.New(apperr.InvalidRule).WithParam(reasonParam, rules.ReasonInvalidRelPath).WithParam("path", res.RelPath)
	}

	updated := r
	updated.Destinations = append([]rules.Destination(nil), r.Destinations...)
	d := updated.Destinations[idx]
	d.Ignore = append([]string(nil), d.Ignore...)
	if !rules.IsIgnoredBy(d, rel) {
		d.Ignore = append(d.Ignore, rel)
	}
	updated.Destinations[idx] = d

	e.removeBaseline(BaselineKey(r.ID, d.ID), rel)
	if err := e.saveState(); err != nil {
		return nil, "", err
	}
	return &updated, "", nil
}

func (e *Engine) resolveUseSource(de *destEval, pair rules.Pair) error {
	if de.isTransform() {
		_, err := e.renderToTarget(de, pair.RelPath, pair.SourceAbs, pair.TargetAbs, true, nil)
		return err
	}
	if err := e.backupTargets(de.rule, de.dest, de.resolved.TargetRoot, []string{pair.RelPath}, nil); err != nil {
		return err
	}
	if _, err := os.Stat(pair.SourceAbs); os.IsNotExist(err) {
		return e.deleteTarget(de, pair.RelPath, pair.TargetAbs)
	}
	return e.copyToTarget(de, pair.RelPath, pair.SourceAbs, pair.TargetAbs)
}

func (e *Engine) resolveUseTarget(de *destEval, pair rules.Pair) error {
	if de.isTransform() {
		return apperr.New(apperr.TransformFailed).WithParam(reasonParam, "use_target_not_supported")
	}
	if !de.rule.HasWritableSource() {
		return apperr.New(apperr.SourceReadOnly).WithParam("ruleId", de.rule.ID)
	}
	info, err := os.Stat(pair.TargetAbs)
	if err != nil || info.IsDir() {
		return apperr.New(apperr.TargetNotFound).WithParam("path", pair.TargetAbs)
	}
	if err := e.backupSourceIfExists(de, pair); err != nil {
		return err
	}
	if err := fsops.AtomicCopy(pair.TargetAbs, pair.SourceAbs); err != nil {
		return err
	}
	hash, err := fsops.HashFile(pair.SourceAbs)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	e.setBaseline(de.key(), pair.RelPath, hash)
	return nil
}

func (e *Engine) resolveCustom(de *destEval, pair rules.Pair, content []byte) error {
	if !de.rule.HasWritableSource() {
		return apperr.New(apperr.SourceReadOnly).WithParam("ruleId", de.rule.ID)
	}
	if err := e.backupSourceIfExists(de, pair); err != nil {
		return err
	}
	if err := fsops.AtomicWrite(pair.SourceAbs, content, filePerm(pair.SourceAbs)); err != nil {
		return err
	}
	if de.isTransform() {
		_, err := e.renderToTarget(de, pair.RelPath, pair.SourceAbs, pair.TargetAbs, false, nil)
		return err
	}
	if err := e.backupTargets(de.rule, de.dest, de.resolved.TargetRoot, []string{pair.RelPath}, nil); err != nil {
		return err
	}
	if err := fsops.AtomicWrite(pair.TargetAbs, content, filePerm(pair.TargetAbs)); err != nil {
		return err
	}
	e.setBaseline(de.key(), pair.RelPath, hashBytes(content))
	return nil
}

func (e *Engine) backupSourceIfExists(de *destEval, pair rules.Pair) error {
	if _, err := os.Stat(pair.SourceAbs); err != nil {
		return nil
	}
	return e.backupSource(de.rule, de.sourceAbs, pair.SourceAbs, nil)
}
