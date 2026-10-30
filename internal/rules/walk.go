package rules

import (
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
)

// walkSkippedNames are always skipped inside folder sources (compared case-insensitively).
// Unlike fsops.ShouldIgnore, dot-prefixed names such as ".claude" or ".agents" are kept.
var walkSkippedNames = map[string]bool{
	".git":        true,
	".ds_store":   true,
	"thumbs.db":   true,
	"desktop.ini": true,
}

// shouldSkipName reports whether a file or folder name is never synced from a folder source.
func shouldSkipName(name string) bool {
	lower := strings.ToLower(name)
	return walkSkippedNames[lower] || strings.HasSuffix(lower, strings.ToLower(fsops.TempFileSuffix))
}

// WalkSource lists the files of a folder source as forward-slash relPath -> absolute path.
// It keeps dot-prefixed names, skips VCS/OS metadata and temp files, never follows
// symlinks to directories and ignores symlinks that resolve outside the source.
func WalkSource(root string) (map[string]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", root)
	}
	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		return nil, apperr.New(apperr.SourceNotFound).WithParam("path", root)
	}

	files := make(map[string]string)
	walkErr := filepath.Walk(absRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == absRoot {
			return nil
		}
		if shouldSkipName(fi.Name()) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.Mode()&os.ModeSymlink != 0 && !isSafeFileSymlink(absRoot, p) {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = p
		return nil
	})
	if walkErr != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, walkErr).WithParam("path", root)
	}
	return files, nil
}

// isSafeFileSymlink accepts symlinks that resolve to a regular file inside root.
func isSafeFileSymlink(root, link string) bool {
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		return false
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	rel, err := filepath.Rel(resolvedRoot, absResolved)
	if err != nil || rel == parentDir || strings.HasPrefix(rel, parentDir+string(os.PathSeparator)) {
		return false
	}
	info, err := os.Stat(absResolved)
	return err == nil && !info.IsDir()
}
