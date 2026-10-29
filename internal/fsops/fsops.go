package fsops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
)

const (
	TempFileSuffix   = ".agentsync.tmp"
	DefaultDirPerms  = 0o755
	DefaultFilePerms = 0o644
)

var ignoredFileNames = map[string]bool{
	".git":        true,
	".gitkeep":    true,
	".ds_store":   true,
	"thumbs.db":   true,
	"desktop.ini": true,
}

// ShouldIgnore returns true if a file or directory name should be ignored during synchronization.
func ShouldIgnore(name string) bool {
	lower := strings.ToLower(name)
	if ignoredFileNames[lower] {
		return true
	}
	if strings.HasPrefix(lower, ".") {
		return true
	}
	if strings.HasSuffix(lower, TempFileSuffix) {
		return true
	}
	return false
}

// WalkManaged walks the profile directory recursively and returns a map of relPath (normalized to '/') -> absPath.
func WalkManaged(profilePath string) (map[string]string, error) {
	absProfile, err := filepath.Abs(profilePath)
	if err != nil {
		return nil, apperr.Wrap(apperr.RootNotFound, err)
	}

	info, err := os.Stat(absProfile)
	if err != nil {
		return nil, apperr.Wrap(apperr.RootNotFound, err)
	}
	if !info.IsDir() {
		return nil, apperr.Wrap(apperr.RootNotFound, fmt.Errorf("%s is not a directory", absProfile))
	}

	managed := make(map[string]string)

	err = filepath.Walk(absProfile, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == absProfile {
			return nil
		}

		name := fi.Name()
		if ShouldIgnore(name) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Handle symlinks safely: verify target is within profilePath
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, evalErr := filepath.EvalSymlinks(path)
			if evalErr != nil {
				return nil // Skip broken symlink
			}
			absResolved, err := filepath.Abs(resolved)
			if err != nil {
				return nil
			}
			relToProfile, err := filepath.Rel(absProfile, absResolved)
			if err != nil || strings.HasPrefix(relToProfile, "..") {
				// Symlink escapes profile: ignore for safety
				return nil
			}
			resInfo, err := os.Stat(absResolved)
			if err != nil {
				return nil
			}
			if resInfo.IsDir() {
				// We don't follow directory symlinks to avoid recursion loops
				return nil
			}
		}

		if fi.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(absProfile, path)
		if err != nil {
			return err
		}

		// Normalize relative path with forward slashes
		normalizedRel := filepath.ToSlash(rel)
		managed[normalizedRel] = path
		return nil
	})

	if err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	return managed, nil
}

// HashFile calculates SHA-256 of the file content in streaming mode.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// AtomicCopy copies src to dst using an atomic temp file in dst's directory, preserving permissions.
func AtomicCopy(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	defer srcFile.Close()

	tmpDst := dst + TempFileSuffix
	tmpFile, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpDst)
	}

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		cleanup()
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpDst)
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := os.Rename(tmpDst, dst); err != nil {
		// On Windows, if destination exists, rename might fail unless removed first.
		_ = os.Remove(dst)
		if retryErr := os.Rename(tmpDst, dst); retryErr != nil {
			os.Remove(tmpDst)
			return apperr.Wrap(apperr.WriteFailed, retryErr)
		}
	}

	return nil
}

// AtomicWrite writes data to dst using a temporary file in the same directory.
func AtomicWrite(dst string, data []byte, perm os.FileMode) error {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	tmpDst := dst + TempFileSuffix
	tmpFile, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpDst)
	}

	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpDst)
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	if err := os.Rename(tmpDst, dst); err != nil {
		_ = os.Remove(dst)
		if retryErr := os.Rename(tmpDst, dst); retryErr != nil {
			os.Remove(tmpDst)
			return apperr.Wrap(apperr.WriteFailed, retryErr)
		}
	}

	return nil
}
