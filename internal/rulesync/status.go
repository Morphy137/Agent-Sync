package rulesync

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
	"agentsync/internal/syncer"
)

// plannedDest is one destination of one rule, resolved but not yet hashed.
type plannedDest struct {
	rule      rules.Rule
	dest      rules.Destination
	resolved  rules.ResolvedDestination
	sourceAbs string
	err       error
	active    bool
	blocked   bool
}

// fileEval is the evaluation of one file, with the hashes used to decide its status.
type fileEval struct {
	FileStatus
	sourceHash   string
	targetHash   string
	baseline     string
	targetExists bool
}

// destEval is an evaluated destination.
type destEval struct {
	plannedDest
	state string
	files []fileEval
	stale []string // baselines to drop on the next Sync
}

func (d *destEval) key() string { return BaselineKey(d.rule.ID, d.dest.ID) }

func (d *destEval) isTransform() bool { return d.dest.Transform != nil }

// uploadAllowed reports whether target edits may flow back into the source.
func (d *destEval) uploadAllowed() bool {
	return d.rule.HasWritableSource() && d.dest.EffectiveMode() == rules.ModeTwoWay && !d.isTransform()
}

type evaluation struct {
	report Report
	dests  []*destEval
}

// plan resolves every destination and detects collisions without hashing anything.
func (e *Engine) plan(all []rules.Rule) ([]*plannedDest, []rules.Collision) {
	var planned []*plannedDest
	var resolved []rules.ResolvedDestination
	var sources []string

	for _, r := range all {
		srcAbs, srcErr := rules.SourcePath(e.Env, r)
		if r.Enabled && srcErr == nil {
			sources = append(sources, srcAbs)
		}
		for _, d := range r.Destinations {
			pd := &plannedDest{rule: r, dest: d, sourceAbs: srcAbs, active: r.Enabled && d.Enabled}
			if pd.active {
				pd.resolved, pd.err = rules.Resolve(e.Env, r, d)
				if pd.err == nil && !pd.resolved.Unmapped {
					resolved = append(resolved, pd.resolved)
				}
			}
			planned = append(planned, pd)
		}
	}

	collisions := rules.DetectCollisions(e.Env, resolved, sources, e.Legacy)
	blocked := make(map[rules.DestRef]bool)
	for _, c := range collisions {
		for _, ref := range c.Refs {
			if ref.RuleID != "" {
				blocked[rules.DestRef{RuleID: ref.RuleID, DestID: ref.DestID}] = true
			}
		}
	}
	for _, pd := range planned {
		pd.blocked = blocked[rules.DestRef{RuleID: pd.rule.ID, DestID: pd.dest.ID}]
	}
	if collisions == nil {
		collisions = []rules.Collision{}
	}
	return planned, collisions
}

// Status computes the full report (no writes).
func (e *Engine) Status(all []rules.Rule) Report {
	return e.evaluate(all).report
}

func (e *Engine) evaluate(all []rules.Rule) *evaluation {
	planned, collisions := e.plan(all)
	ev := &evaluation{report: Report{
		Rules:      make([]RuleStatus, 0, len(all)),
		Collisions: collisions,
		Unmapped:   []string{},
	}}
	unmapped := make(map[string]bool)

	idx := 0
	for _, r := range all {
		rs := RuleStatus{
			Rule:         r,
			Scope:        r.Scope,
			DetectedKind: e.detectKind(r),
			Destinations: make([]DestinationStatus, 0, len(r.Destinations)),
		}
		for range r.Destinations {
			de := e.evaluateDest(planned[idx])
			idx++
			ev.dests = append(ev.dests, de)
			if de.state == DestUnmapped && de.resolved.Unmapped {
				unmapped[de.dest.Location] = true
			}
			rs.Destinations = append(rs.Destinations, de.status())
			e.countReport(&ev.report, de)
		}
		ev.report.Rules = append(ev.report.Rules, rs)
	}

	for alias := range unmapped {
		ev.report.Unmapped = append(ev.report.Unmapped, alias)
	}
	sort.Strings(ev.report.Unmapped)
	return ev
}

func (e *Engine) countReport(rep *Report, de *destEval) {
	if de.state != DestReady {
		return
	}
	for _, f := range de.files {
		switch f.Status {
		case StatusNewInSource, StatusSourceChanged, StatusTargetMissing, StatusSourceDeleted, StatusTargetChanged:
			rep.Pending++
		case StatusConflict, StatusUntrackedDiff, StatusDrift:
			rep.Conflicts++
		}
	}
}

func (d *destEval) status() DestinationStatus {
	ds := DestinationStatus{
		RuleID:     d.rule.ID,
		DestID:     d.dest.ID,
		State:      d.state,
		TargetRoot: d.resolved.TargetRoot,
		Files:      make([]FileStatus, 0, len(d.files)),
	}
	for _, f := range d.files {
		ds.Files = append(ds.Files, f.FileStatus)
	}
	return ds
}

func (e *Engine) detectKind(r rules.Rule) string {
	if kind := r.EffectiveKind(); kind != rules.KindAuto {
		return kind
	}
	srcAbs, err := rules.SourcePath(e.Env, r)
	if err != nil {
		return ""
	}
	info, err := os.Stat(srcAbs)
	if err != nil {
		return ""
	}
	if e.DetectKind != nil {
		if kind := e.DetectKind(srcAbs); kind != "" {
			return kind
		}
	}
	if info.IsDir() {
		return rules.KindFolder
	}
	return rules.KindFile
}

// destStateForError maps a resolution error to a destination state.
func destStateForError(err error) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && (appErr.Code == apperr.LocationInvalid || appErr.Code == apperr.LocationUnmapped) {
		return DestUnmapped
	}
	return DestSourceMissing
}

func (e *Engine) evaluateDest(pd *plannedDest) *destEval {
	de := &destEval{plannedDest: *pd}
	switch {
	case !pd.active:
		de.state = DestDisabled
	case pd.err != nil:
		de.state = destStateForError(pd.err)
	case pd.resolved.Unmapped:
		de.state = DestUnmapped
	case pd.blocked:
		de.state = DestBlocked
	case pd.resolved.SourceMissing:
		de.state = DestSourceMissing
	default:
		de.state = DestReady
		e.evaluateFiles(de)
	}
	return de
}

func (e *Engine) evaluateFiles(de *destEval) {
	present := make(map[string]bool, len(de.resolved.Pairs))
	for _, p := range de.resolved.Pairs {
		present[p.RelPath] = true
		if de.isTransform() {
			de.files = append(de.files, e.evalTransformPair(de, p))
		} else {
			de.files = append(de.files, e.evalPlainPair(de, p))
		}
	}
	e.evaluateOrphanBaselines(de, present)
	e.appendIgnored(de)
	sort.Slice(de.files, func(i, j int) bool { return de.files[i].RelPath < de.files[j].RelPath })
}

// readTarget hashes a destination file. exists is false when it is missing.
func readTargetHash(p string) (hash string, exists bool, err error) {
	info, statErr := os.Stat(p)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return "", false, nil
		}
		return "", false, statErr
	}
	if info.IsDir() {
		return "", true, errors.New("destination is a directory")
	}
	hash, err = fsops.HashFile(p)
	return hash, true, err
}

func newFileEval(p rules.Pair) fileEval {
	return fileEval{FileStatus: FileStatus{RelPath: p.RelPath, SourceAbs: p.SourceAbs, TargetAbs: p.TargetAbs}}
}

func (e *Engine) evalPlainPair(de *destEval, p rules.Pair) fileEval {
	fe := newFileEval(p)
	fe.baseline = e.baseline(de.key(), p.RelPath)

	srcHash, srcErr := fsops.HashFile(p.SourceAbs)
	tgtHash, tgtExists, tgtErr := readTargetHash(p.TargetAbs)
	fe.sourceHash, fe.targetHash, fe.targetExists = srcHash, tgtHash, tgtExists
	if srcErr != nil || tgtErr != nil {
		fe.Status = StatusUnreadable
		return fe
	}

	if de.dest.EffectiveMode() == rules.ModeCreateOnly && tgtExists {
		fe.Status = StatusInSync
		return fe
	}

	status := syncer.EvaluateFileStatus(srcHash, tgtHash, fe.baseline)
	if !de.uploadAllowed() && (status == StatusTargetChanged || status == StatusConflict) {
		status = StatusDrift
	}
	fe.Status = status
	return fe
}

// evalTransformPair compares the canonical source view and managed target view.
// The source side is compared to the source baseline and the target side to the
// target-view baseline, so transforms that normalize values never look like drift.
func (e *Engine) evalTransformPair(de *destEval, p rules.Pair) fileEval {
	fe := newFileEval(p)
	fe.Status = StatusUnreadable
	if e.Transformer == nil {
		return fe
	}
	spec := *de.dest.Transform
	src, err := os.ReadFile(p.SourceAbs)
	if err != nil {
		return fe
	}
	srcView, err := e.Transformer.SourceView(spec, src)
	if err != nil {
		return fe
	}
	existing, exists, err := readExisting(p.TargetAbs)
	if err != nil {
		return fe
	}
	fe.sourceHash = hashBytes(srcView)
	fe.targetExists = exists
	fe.baseline = e.baseline(de.key(), p.RelPath)
	prevManaged := e.managedEntries(de.rule.ID, de.dest.ID)

	if fe.baseline == "" {
		fe.Status = e.firstTransformStatus(spec, src, srcView, existing, exists, prevManaged)
		return fe
	}
	if !exists {
		fe.Status = StatusSourceChanged
		if fe.sourceHash == fe.baseline {
			fe.Status = StatusTargetMissing
		}
		return fe
	}

	targetView, err := e.Transformer.TargetView(spec, existing, prevManaged)
	if err != nil {
		return fe
	}
	fe.targetHash = hashBytes(targetView)
	targetBaseline := e.baseline(ManagedKey(de.rule.ID, de.dest.ID), TargetViewRel)
	sourceChanged := fe.sourceHash != fe.baseline
	targetChanged := targetBaseline != "" && fe.targetHash != targetBaseline

	switch {
	case targetChanged:
		fe.Status = StatusDrift
	case sourceChanged:
		fe.Status = StatusSourceChanged
	default:
		fe.Status = StatusInSync
	}
	return fe
}

// firstTransformStatus classifies a transformed destination that was never synced, using a
// dry-run render: identical output, or a destination that already holds exactly the source
// entries, is in_sync (Sync adopts them); a render failure (e.g. an unowned entry with the
// same name but different content) is a conflict; anything else is a normal merge.
func (e *Engine) firstTransformStatus(spec rules.Transform, src, srcView, existing []byte, exists bool, prevManaged []string) string {
	out, _, _, err := e.Transformer.Render(spec, src, existing, prevManaged)
	if err != nil {
		if exists && e.targetHoldsSourceEntries(spec, srcView, existing) {
			return StatusInSync
		}
		return StatusConflict
	}
	if exists && bytes.Equal(out, existing) {
		return StatusInSync
	}
	return StatusNewInSource
}

func readExisting(p string) ([]byte, bool, error) {
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if info.IsDir() {
		return nil, true, errors.New("destination is a directory")
	}
	data, err := os.ReadFile(p)
	return data, true, err
}

// evaluateOrphanBaselines handles baselines whose file is no longer produced by the source:
// a deleted source file with an untouched destination becomes source_deleted; everything
// that cannot be acted on safely is only marked stale (its baseline is dropped on Sync).
func (e *Engine) evaluateOrphanBaselines(de *destEval, present map[string]bool) {
	baselines := e.baselines(de.key())
	rels := make([]string, 0, len(baselines))
	for rel := range baselines {
		if !present[rel] {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)

	for _, rel := range rels {
		fe, ok := e.evalOrphan(de, rel, baselines[rel])
		if !ok {
			de.stale = append(de.stale, rel)
			continue
		}
		de.files = append(de.files, fe)
	}
}

func (e *Engine) evalOrphan(de *destEval, rel, base string) (fileEval, bool) {
	if !de.resolved.SourceIsDir || de.dest.EffectiveMode() == rules.ModeCreateOnly || rules.IsIgnoredBy(de.dest, rel) {
		return fileEval{}, false
	}
	cleaned, err := rules.NormalizeRelPath(rel)
	if err != nil || cleaned != rel {
		return fileEval{}, false
	}
	srcAbs := filepath.Join(de.sourceAbs, filepath.FromSlash(rel))
	if _, err := os.Lstat(srcAbs); err == nil {
		// Still present but no longer managed (excluded or ignored by the walker).
		return fileEval{}, false
	}
	tgtAbs := filepath.Join(de.resolved.TargetRoot, filepath.FromSlash(rel))
	tgtHash, exists, err := readTargetHash(tgtAbs)
	if !exists {
		return fileEval{}, false
	}

	fe := fileEval{
		FileStatus:   FileStatus{RelPath: rel, SourceAbs: srcAbs, TargetAbs: tgtAbs},
		targetHash:   tgtHash,
		baseline:     base,
		targetExists: true,
	}
	switch {
	case err != nil:
		fe.Status = StatusUnreadable
	case tgtHash == base:
		fe.Status = StatusSourceDeleted
	case de.uploadAllowed():
		fe.Status = StatusConflict
	default:
		fe.Status = StatusDrift
	}
	return fe, true
}

// appendIgnored lists the files the user chose to keep local.
func (e *Engine) appendIgnored(de *destEval) {
	for _, entry := range de.dest.Ignore {
		rel, err := rules.NormalizeRelPath(entry)
		if err != nil || rel == "." {
			continue
		}
		srcAbs := de.sourceAbs
		if de.resolved.SourceIsDir {
			srcAbs = filepath.Join(de.sourceAbs, filepath.FromSlash(rel))
		}
		de.files = append(de.files, fileEval{FileStatus: FileStatus{
			RelPath:   rel,
			SourceAbs: srcAbs,
			TargetAbs: filepath.Join(de.resolved.TargetRoot, filepath.FromSlash(rel)),
			Status:    StatusIgnored,
		}})
	}
}

// targetHoldsSourceEntries reports whether the destination already contains every source
// entry with identical content.
func (e *Engine) targetHoldsSourceEntries(spec rules.Transform, srcView, existing []byte) bool {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(srcView, &entries); err != nil {
		return false
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	targetView, err := e.Transformer.TargetView(spec, existing, names)
	return err == nil && bytes.Equal(targetView, srcView)
}
