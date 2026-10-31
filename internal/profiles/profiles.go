package profiles

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
)

// Profile represents a top-level agent configuration folder within the root.
type Profile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	FileCount int    `json:"fileCount"`
}

// Discover finds all valid profile subdirectories in the given root path.
func Discover(rootPath string) ([]Profile, error) {
	if rootPath == "" {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}
	if !info.IsDir() {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", rootPath)
	}

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	result := make([]Profile, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if fsops.ShouldIgnore(name) {
			continue
		}

		profileAbs := filepath.Join(absRoot, name)
		managed, err := fsops.WalkManaged(profileAbs)
		if err != nil {
			// If error reading profile folder, still list it with count 0
			result = append(result, Profile{
				Name:      name,
				Path:      profileAbs,
				FileCount: 0,
			})
			continue
		}

		result = append(result, Profile{
			Name:      name,
			Path:      profileAbs,
			FileCount: len(managed),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}
