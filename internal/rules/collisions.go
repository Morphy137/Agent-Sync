package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// destTargets is a resolved destination reduced to the absolute paths it writes.
type destTargets struct {
	ref       DestRef
	root      string
	isDir     bool
	files     []string
	transform *Transform
	entries   []string
}

// points returns every absolute path the destination owns: its folder root (folder sources)
// followed by its file targets in sorted order.
func (d destTargets) points() []string {
	if d.isDir {
		return append([]string{d.root}, d.files...)
	}
	return d.files
}

// collisionSet accumulates collisions de-duplicated by type, path and refs.
type collisionSet struct {
	byKey map[string]Collision
}

func (s *collisionSet) add(kind, p string, refs ...DestRef) {
	unique := dedupeRefs(refs)
	key := kind + "\x00" + p + "\x00" + refsKey(unique)
	if _, ok := s.byKey[key]; ok {
		return
	}
	s.byKey[key] = Collision{Type: kind, Path: p, Refs: unique}
}

func (s *collisionSet) sorted() []Collision {
	out := make([]Collision, 0, len(s.byKey))
	for _, c := range s.byKey {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return refsKey(out[i].Refs) < refsKey(out[j].Refs)
	})
	return out
}

func refLess(a, b DestRef) bool {
	if a.RuleID != b.RuleID {
		return a.RuleID < b.RuleID
	}
	return a.DestID < b.DestID
}

func dedupeRefs(refs []DestRef) []DestRef {
	seen := make(map[DestRef]bool, len(refs))
	out := make([]DestRef, 0, len(refs))
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return refLess(out[i], out[j]) })
	return out
}

func refsKey(refs []DestRef) string {
	parts := make([]string, 0, len(refs))
	for _, r := range refs {
		parts = append(parts, r.RuleID+"/"+r.DestID)
	}
	return strings.Join(parts, "|")
}

// foldPath normalizes an absolute path for case-insensitive comparisons.
func foldPath(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

// isWithin reports whether child equals parent or is inside it, ignoring case.
func isWithin(child, parent string) bool {
	if child == "" || parent == "" {
		return false
	}
	c, p := foldPath(child), foldPath(parent)
	if c == p {
		return true
	}
	sep := string(os.PathSeparator)
	if !strings.HasSuffix(p, sep) {
		p += sep
	}
	return strings.HasPrefix(c, p)
}

// DetectCollisions checks all resolved enabled destinations.
// Every DestRef involved in any collision must be treated as blocked by callers.
// Unmapped destinations are ignored. Output is sorted by type, path and refs.
func DetectCollisions(env Env, resolved []ResolvedDestination, sources []string) []Collision {
	set := &collisionSet{byKey: make(map[string]Collision)}
	dests := buildDestTargets(resolved)

	detectSameTargets(set, dests)
	detectFolderOverlaps(set, dests)
	detectCycles(set, dests, sources)
	detectInsideRepo(set, dests, env.RepoRoot)

	return set.sorted()
}

func buildDestTargets(resolved []ResolvedDestination) []destTargets {
	dests := make([]destTargets, 0, len(resolved))
	for _, rd := range resolved {
		if rd.Unmapped {
			continue
		}
		dt := destTargets{
			ref:       DestRef{RuleID: rd.RuleID, DestID: rd.DestID},
			isDir:     rd.SourceIsDir,
			transform: rd.Transform,
			entries:   rd.MergeEntries,
		}
		if rd.TargetRoot != "" {
			dt.root = filepath.Clean(rd.TargetRoot)
		}
		for _, p := range rd.Pairs {
			dt.files = append(dt.files, filepath.Clean(p.TargetAbs))
		}
		sort.Strings(dt.files)
		if dt.isDir && dt.root == "" {
			continue
		}
		dests = append(dests, dt)
	}
	sort.Slice(dests, func(i, j int) bool { return refLess(dests[i].ref, dests[j].ref) })
	return dests
}

// detectSameTargets reports same_target for identical file targets across destinations and
// case_conflict when targets differ only by letter case (even inside one destination).
func detectSameTargets(set *collisionSet, dests []destTargets) {
	type target struct {
		path string
		dest destTargets
	}
	groups := make(map[string][]target)
	for _, d := range dests {
		for _, file := range d.files {
			groups[foldPath(file)] = append(groups[foldPath(file)], target{file, d})
		}
	}
	for _, group := range groups {
		for i, a := range group {
			for _, b := range group[i+1:] {
				if a.path != b.path {
					set.add(CollisionCaseConflict, min(a.path, b.path), a.dest.ref, b.dest.ref)
				} else if a.dest.ref != b.dest.ref && !compatibleMerges(a.dest, b.dest) {
					set.add(CollisionSameTarget, a.path, a.dest.ref, b.dest.ref)
				}
			}
		}
	}
}

func compatibleMerges(a, b destTargets) bool {
	if a.transform == nil || b.transform == nil || *a.transform != *b.transform || a.entries == nil || b.entries == nil {
		return false
	}
	for _, name := range a.entries {
		for _, other := range b.entries {
			if name == other {
				return false
			}
		}
	}
	return true
}

// detectFolderOverlaps reports file-versus-directory path conflicts, not shared folders.
func detectFolderOverlaps(set *collisionSet, dests []destTargets) {
	type target struct {
		path string
		ref  DestRef
	}
	files := make(map[string][]target)
	for _, d := range dests {
		for _, p := range d.files {
			files[foldPath(p)] = append(files[foldPath(p)], target{p, d.ref})
		}
	}
	for _, d := range dests {
		points := d.files
		if d.isDir {
			points = append(append([]string{}, points...), d.root)
		}
		for _, point := range points {
			for parent := point; ; parent = filepath.Dir(parent) {
				if parent != point || (d.isDir && point == d.root) {
					for _, file := range files[foldPath(parent)] {
						if file.ref != d.ref {
							set.add(CollisionFolderOverlap, file.path, file.ref, d.ref)
						}
					}
				}
				next := filepath.Dir(parent)
				if next == parent {
					break
				}
			}
		}
	}
}

// detectCycles reports destinations writing into (or containing) a rule source.
func detectCycles(set *collisionSet, dests []destTargets, sources []string) {
	cleaned := make([]string, 0, len(sources))
	for _, s := range sources {
		if s != "" {
			cleaned = append(cleaned, filepath.Clean(s))
		}
	}
	sort.Strings(cleaned)
	for _, d := range dests {
		for _, src := range cleaned {
			reported := false
			for _, p := range d.points() {
				if isWithin(p, src) {
					set.add(CollisionCycle, p, d.ref)
					reported = true
					break
				}
			}
			if !reported && d.isDir && isWithin(src, d.root) {
				set.add(CollisionCycle, src, d.ref)
			}
		}
	}
}

// detectInsideRepo reports destinations writing inside the repository root.
func detectInsideRepo(set *collisionSet, dests []destTargets, repoRoot string) {
	if repoRoot == "" {
		return
	}
	for _, d := range dests {
		for _, p := range d.points() {
			if isWithin(p, repoRoot) {
				set.add(CollisionInsideRepo, p, d.ref)
				break
			}
		}
	}
}
