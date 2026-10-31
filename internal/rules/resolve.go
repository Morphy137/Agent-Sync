package rules

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"agentsync/internal/apperr"
)

// LocationRoot returns the absolute folder mapped to alias on this PC. The builtin "home"
// alias resolves to env.HomeDir. ok is false when the alias is not mapped.
func LocationRoot(env Env, alias string) (string, bool) {
	if alias == BuiltinHomeAlias {
		return env.HomeDir, env.HomeDir != ""
	}
	root, ok := env.Locations[alias]
	if !ok || root == "" {
		return "", false
	}
	return root, true
}

// SourcePath returns the absolute path of the rule source on this PC.
func SourcePath(env Env, r Rule) (string, error) {
	switch r.Source.Origin {
	case OriginRepo:
		if env.RepoRoot == "" {
			return "", apperr.New(apperr.RootNotFound)
		}
		return joinSourceInside(env.RepoRoot, r.Source)
	case OriginSubscription:
		root, ok := env.SubscriptionRoots[r.Source.Subscription]
		if !ok || root == "" {
			return "", apperr.New(apperr.SubscriptionNotFound).WithParam("id", r.Source.Subscription)
		}
		return joinSourceInside(root, r.Source)
	case OriginLocal:
		if !filepath.IsAbs(r.Source.Path) {
			return "", invalid(ReasonInvalidSourcePath).WithParam("path", r.Source.Path)
		}
		return filepath.Clean(r.Source.Path), nil
	}
	return "", invalid(ReasonInvalidOrigin).WithParam("origin", r.Source.Origin)
}

// joinSourceInside resolves a repository or subscription source; selections may use the root.
func joinSourceInside(root string, src Source) (string, error) {
	if src.HasSelection() && isRootPath(src.Path) {
		return filepath.Clean(root), nil
	}
	return joinInside(root, src.Path)
}

// joinInside joins a relative path to root and refuses results escaping root.
func joinInside(root, rel string) (string, error) {
	if hasParentSegment(rel) {
		return "", invalid(ReasonInvalidSourcePath).WithParam("path", rel)
	}
	cleaned, err := NormalizeRelPath(rel)
	if err != nil || cleaned == currentDir {
		return "", invalid(ReasonInvalidSourcePath).WithParam("path", rel)
	}
	return filepath.Join(filepath.Clean(root), filepath.FromSlash(cleaned)), nil
}

// Resolve expands one destination of one rule. For a file source there is one Pair with
// RelPath = base name of the destination path and TargetRoot = its parent folder.
// For a folder source, every file found by WalkSource (dot-prefixed names kept; .git, OS
// metadata, temp files and unsafe symlinks skipped) minus rule Exclude and DefaultSecretExcludes becomes a Pair under TargetAbs = <location>/<dest.Path>/<rel>,
// TargetRoot = <location>/<dest.Path>. Files in dest.Ignore are skipped.
// An unmapped location sets Unmapped and a missing source sets SourceMissing; neither is an error.
func Resolve(env Env, r Rule, d Destination) (ResolvedDestination, error) {
	res := ResolvedDestination{RuleID: r.ID, DestID: d.ID, Pairs: []Pair{}}

	locRoot, ok := LocationRoot(env, d.Location)
	if !ok {
		res.Unmapped = true
		return res, nil
	}
	if !filepath.IsAbs(locRoot) {
		return res, apperr.New(apperr.LocationInvalid).WithParam("alias", d.Location).WithParam("path", locRoot)
	}
	locRoot = filepath.Clean(locRoot)

	destRel, err := NormalizeRelPath(d.Path)
	if err != nil {
		return res, invalid(ReasonInvalidDestPath).WithParam("destId", d.ID).WithParam("path", d.Path)
	}
	destBase := locRoot
	if destRel != currentDir {
		destBase = filepath.Join(locRoot, filepath.FromSlash(destRel))
	}

	srcAbs, err := SourcePath(env, r)
	if err != nil {
		return res, err
	}

	info, statErr := os.Stat(srcAbs)
	if statErr != nil {
		if !os.IsNotExist(statErr) {
			return res, apperr.Wrap(apperr.SourceNotFound, statErr).WithParam("path", srcAbs)
		}
		res.SourceMissing = true
		res.SourceIsDir = isFolderKind(r.Kind) || r.Source.HasSelection()
		res.TargetRoot = destBase
		if !res.SourceIsDir {
			res.TargetRoot = filepath.Dir(destBase)
		}
		return res, nil
	}

	ignored := normalizedIgnoreSet(d.Ignore)
	if !info.IsDir() {
		if r.Source.HasSelection() {
			return res, invalid(ReasonSelectionNeedsFolder).WithParam("path", srcAbs)
		}
		return resolveFile(res, d, srcAbs, destRel, destBase, ignored)
	}
	return resolveFolder(res, r, d, srcAbs, destBase, ignored)
}

func resolveFile(res ResolvedDestination, d Destination, srcAbs, destRel, destBase string, ignored map[string]bool) (ResolvedDestination, error) {
	if destRel == currentDir {
		return res, invalid(ReasonFileDestNeedsPath).WithParam("destId", d.ID)
	}
	res.TargetRoot = filepath.Dir(destBase)
	rel := filepath.Base(destBase)
	if isIgnored(rel, ignored) {
		return res, nil
	}
	res.Pairs = append(res.Pairs, Pair{RelPath: rel, SourceAbs: srcAbs, TargetAbs: destBase})
	return res, nil
}

func resolveFolder(res ResolvedDestination, r Rule, d Destination, srcAbs, destBase string, ignored map[string]bool) (ResolvedDestination, error) {
	if d.Transform != nil {
		return res, invalid(ReasonTransformNeedsFile).WithParam("destId", d.ID)
	}
	res.SourceIsDir = true
	res.TargetRoot = destBase

	managed, err := WalkSource(srcAbs)
	if err != nil {
		return res, err
	}
	selected := selectionSet(r.Source.Include)
	for rel, abs := range managed {
		if selected != nil && !selected[rel] {
			continue
		}
		if IsExcluded(rel, r.Exclude) || isIgnored(rel, ignored) {
			continue
		}
		res.Pairs = append(res.Pairs, Pair{
			RelPath:   rel,
			SourceAbs: abs,
			TargetAbs: filepath.Join(destBase, filepath.FromSlash(rel)),
		})
	}
	sort.Slice(res.Pairs, func(i, j int) bool { return res.Pairs[i].RelPath < res.Pairs[j].RelPath })
	return res, nil
}

// IsExcluded reports whether a forward-slash relative path matches a user pattern or a
// DefaultSecretExcludes pattern. Patterns are matched against the full relative path,
// the base name, every leading directory prefix and every directory name.
func IsExcluded(rel string, patterns []string) bool {
	candidates := exclusionCandidates(rel)
	for _, group := range [][]string{patterns, DefaultSecretExcludes} {
		for _, pattern := range group {
			for _, candidate := range candidates {
				if matched, err := path.Match(pattern, candidate); err == nil && matched {
					return true
				}
			}
		}
	}
	return false
}

func exclusionCandidates(rel string) []string {
	segments := strings.Split(rel, "/")
	candidates := []string{rel, segments[len(segments)-1]}
	for i := 1; i < len(segments); i++ {
		candidates = append(candidates, strings.Join(segments[:i], "/"), segments[i-1])
	}
	return candidates
}

// selectionSet returns the normalized Include entries, or nil when the whole folder is synced.
func selectionSet(include []string) map[string]bool {
	if len(include) == 0 {
		return nil
	}
	set := make(map[string]bool, len(include))
	for _, entry := range include {
		if normalized, err := NormalizeRelPath(entry); err == nil {
			set[normalized] = true
		}
	}
	return set
}

func normalizedIgnoreSet(entries []string) map[string]bool {
	set := make(map[string]bool, len(entries))
	for _, entry := range entries {
		cleaned, err := NormalizeRelPath(entry)
		if err != nil || cleaned == currentDir {
			continue
		}
		set[cleaned] = true
	}
	return set
}

// isIgnored matches an exact ignored file or any ignored parent folder.
func isIgnored(rel string, ignored map[string]bool) bool {
	if ignored[rel] {
		return true
	}
	for i := strings.Index(rel, "/"); i >= 0; {
		if ignored[rel[:i]] {
			return true
		}
		next := strings.Index(rel[i+1:], "/")
		if next < 0 {
			break
		}
		i += next + 1
	}
	return false
}

// IsIgnoredBy reports whether rel is covered by the destination's Ignore list.
func IsIgnoredBy(d Destination, rel string) bool {
	return isIgnored(rel, normalizedIgnoreSet(d.Ignore))
}
