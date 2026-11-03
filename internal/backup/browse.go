package backup

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
)

const (
	// ReasonRestore marks the snapshot of the current files taken before a restore overwrites them.
	ReasonRestore = "restore"

	// RulesFolder is the folder under the backup root holding the snapshots of rule destinations.
	RulesFolder = "rules"
	// SourceSlotID is the destination slot used by snapshots of rule sources.
	SourceSlotID = "_source"

	// MaxListedBackups bounds how many backups a listing returns (newest first).
	MaxListedBackups = 500
)

// Backup kinds.
const (
	KindRule   = "rule"
	KindSource = "source"
	KindRepo   = "repo"
	// KindLegacy is a snapshot written by a version that predates sync rules.
	KindLegacy = "legacy"
)

const (
	idSeparator       = "/"
	ruleIDSegments    = 4 // rules/<rule>__<dest>/<folder>/<timestamp>
	plainIDSegments   = 2 // <group>/<timestamp>
	ownerKeySeparator = ":"
	parentSegment     = ".."
	currentSegment    = "."
	gitFolder         = ".git"
)

// Entry is one backup found on disk, with its files validated against the snapshot folder.
type Entry struct {
	// ID is the snapshot folder relative to the backup root, with forward slashes.
	ID         string
	Kind       string
	OwnerID    string
	Name       string
	TargetPath string
	CreatedAt  string
	Reason     string
	// Files are the relative paths (forward slashes) that exist in the snapshot.
	Files []string
	Size  int64
	// Path is the absolute snapshot folder.
	Path string
}

// RestoreFile is one file ready to be copied from a snapshot to its destination.
type RestoreFile struct {
	Rel string
	Src string
	Dst string
}

// CleanRelPath validates a stored relative path: forward slashes, no parent segments, no
// absolute or drive-qualified forms.
func CleanRelPath(rel string) (string, bool) {
	if rel == "" || strings.ContainsAny(rel, "\\:") || strings.HasPrefix(rel, idSeparator) {
		return "", false
	}
	cleaned := path.Clean(rel)
	if cleaned == currentSegment || cleaned == parentSegment || strings.HasPrefix(cleaned, parentSegment+idSeparator) {
		return "", false
	}
	return cleaned, true
}

func validIDSegment(seg string) bool {
	return seg != "" && seg != currentSegment && seg != parentSegment && !strings.ContainsAny(seg, "\\:")
}

func kindOf(group string, ownerID string) string {
	switch {
	case group == RepoSnapshotID:
		return KindRepo
	case group != RulesFolder:
		return KindLegacy
	case strings.HasSuffix(ownerID, ownerKeySeparator+SourceSlotID):
		return KindSource
	}
	return KindRule
}

// buildEntry validates the files of one snapshot and totals their size; ok is false when nothing
// restorable remains.
func buildEntry(id, group string, info BackupInfo) (Entry, bool) {
	entry := Entry{
		ID:         id,
		Kind:       kindOf(group, info.OwnerID),
		OwnerID:    info.OwnerID,
		Name:       info.Name,
		TargetPath: info.TargetPath,
		CreatedAt:  info.CreatedAt,
		Reason:     info.Reason,
		Path:       info.Path,
		Files:      []string{},
	}
	for _, rel := range info.Files {
		cleaned, ok := CleanRelPath(rel)
		if !ok {
			continue
		}
		stat, err := os.Lstat(filepath.Join(info.Path, filepath.FromSlash(cleaned)))
		if err != nil || !stat.Mode().IsRegular() {
			continue
		}
		entry.Files = append(entry.Files, cleaned)
		entry.Size += stat.Size()
	}
	return entry, len(entry.Files) > 0
}

func collectGroup(groupDir, group string, idPrefix []string, out *[]Entry) {
	infos, err := listGroup(groupDir)
	if err != nil {
		return
	}
	for _, info := range infos {
		id := strings.Join(append(append([]string{}, idPrefix...), info.ID), idSeparator)
		if entry, ok := buildEntry(id, group, info); ok {
			*out = append(*out, entry)
		}
	}
}

func subDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// ListAll returns every readable backup under backupRoot (rules, sources, repository discards and
// snapshots of earlier versions), newest first, bounded by MaxListedBackups. Corrupt or partial
// snapshots are skipped.
func ListAll(backupRoot string) ([]Entry, error) {
	entries := []Entry{}
	for _, name := range subDirs(backupRoot) {
		top := filepath.Join(backupRoot, name)
		if name != RulesFolder {
			collectGroup(top, name, []string{name}, &entries)
			continue
		}
		for _, ruleGroup := range subDirs(top) {
			ruleDir := filepath.Join(top, ruleGroup)
			for _, folder := range subDirs(ruleDir) {
				collectGroup(filepath.Join(ruleDir, folder), RulesFolder, []string{RulesFolder, ruleGroup, folder}, &entries)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt != entries[j].CreatedAt {
			return entries[i].CreatedAt > entries[j].CreatedAt
		}
		return entries[i].ID > entries[j].ID
	})
	if len(entries) > MaxListedBackups {
		entries = entries[:MaxListedBackups]
	}
	return entries, nil
}

// Open loads one backup by the ID returned by ListAll. The ID is validated segment by segment so
// it can never leave the backup root.
func Open(backupRoot, id string) (Entry, error) {
	notFound := apperr.New(apperr.BackupNotFound).WithParam("id", id)
	segments := strings.Split(id, idSeparator)
	for _, seg := range segments {
		if !validIDSegment(seg) {
			return Entry{}, notFound
		}
	}
	isRules := segments[0] == RulesFolder
	if (isRules && len(segments) != ruleIDSegments) || (!isRules && len(segments) != plainIDSegments) {
		return Entry{}, notFound
	}

	dir := filepath.Join(append([]string{backupRoot}, segments...)...)
	if stat, err := os.Lstat(dir); err != nil || !stat.IsDir() {
		return Entry{}, notFound
	}
	manifest, err := readManifest(dir)
	if err != nil {
		return Entry{}, notFound
	}
	info := BackupInfo{
		ID:         segments[len(segments)-1],
		OwnerID:    manifest.OwnerID,
		Name:       manifest.Name,
		TargetPath: manifest.TargetPath,
		CreatedAt:  manifest.CreatedAt,
		Reason:     manifest.Reason,
		Files:      manifest.Files,
		Path:       dir,
	}
	entry, ok := buildEntry(id, segments[0], info)
	if !ok {
		return Entry{}, notFound
	}
	return entry, nil
}

// Select returns the files to restore: all of them when requested is empty, otherwise the
// requested ones, each of which must be part of the backup.
func (e Entry) Select(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string{}, e.Files...), nil
	}
	known := make(map[string]bool, len(e.Files))
	for _, f := range e.Files {
		known[f] = true
	}
	seen := make(map[string]bool, len(requested))
	selected := make([]string, 0, len(requested))
	for _, rel := range requested {
		cleaned, ok := CleanRelPath(rel)
		if !ok || !known[cleaned] {
			return nil, apperr.New(apperr.BackupNotFound).WithParam("id", e.ID).WithParam("path", rel)
		}
		if !seen[cleaned] {
			seen[cleaned] = true
			selected = append(selected, cleaned)
		}
	}
	return selected, nil
}

// SafeJoin joins rel under root and refuses anything that would write outside it: parent segments,
// absolute paths, a symlinked folder leading elsewhere, or a destination that is a symlink or a folder.
func SafeJoin(root, rel string) (string, error) {
	unsafe := apperr.New(apperr.BackupUnsafePath).WithParam("path", rel)
	cleaned, ok := CleanRelPath(rel)
	if !ok {
		return "", unsafe
	}
	target := filepath.Join(root, filepath.FromSlash(cleaned))
	if !config.IsSubdirectory(target, root) {
		return "", unsafe
	}

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}
	ancestor := filepath.Dir(target)
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", unsafe
		}
		ancestor = parent
	}
	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil || !config.IsSubdirectory(realAncestor, realRoot) {
		return "", unsafe
	}
	if stat, err := os.Lstat(target); err == nil && !stat.Mode().IsRegular() {
		return "", unsafe
	}
	return target, nil
}

// PrepareRestore validates every file before anything is written, so an unsafe entry aborts the
// whole restore. Repository restores also refuse the .git folder.
func PrepareRestore(entry Entry, root string, rels []string) ([]RestoreFile, error) {
	files := make([]RestoreFile, 0, len(rels))
	for _, rel := range rels {
		if entry.Kind == KindRepo && (rel == gitFolder || strings.HasPrefix(rel, gitFolder+idSeparator)) {
			return nil, apperr.New(apperr.BackupUnsafePath).WithParam("path", rel)
		}
		dst, err := SafeJoin(root, rel)
		if err != nil {
			return nil, err
		}
		files = append(files, RestoreFile{Rel: rel, Src: filepath.Join(entry.Path, filepath.FromSlash(rel)), Dst: dst})
	}
	return files, nil
}

// ApplyRestore copies the prepared files to their destinations.
func ApplyRestore(files []RestoreFile) error {
	for _, f := range files {
		if err := fsops.AtomicCopy(f.Src, f.Dst); err != nil {
			return err
		}
	}
	return nil
}
