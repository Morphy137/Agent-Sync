package ops

import (
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

// ImportIntoRepo copies a local source into the repository without overwriting existing paths.
func (a *API) ImportIntoRepo(sourcePath, repoRelDir string) (string, error) {
	return a.svc.ImportIntoRepo(sourcePath, repoRelDir)
}

// SubscriptionFileDiff compares the local clone with the last fetched remote version.
func (a *API) SubscriptionFileDiff(id, path string) (*rulesync.Diff, error) {
	return a.svc.GetSubscriptionIncomingDiff(id, path)
}

// ListRuleSourceFiles lists only files selected by an existing rule.
func (a *API) ListRuleSourceFiles(ruleID string) ([]service.RuleSourceFile, error) {
	return a.svc.ListRuleSourceFiles(ruleID)
}

// ReadRuleSourceFile reads a selected source file and its concurrency hash.
func (a *API) ReadRuleSourceFile(ruleID, path string) (*service.RuleSourceContent, error) {
	return a.svc.ReadRuleSourceFile(ruleID, path)
}

// SaveRuleSourceFile edits a selected source using the service's scope and hash guards.
func (a *API) SaveRuleSourceFile(ruleID, path, content, expectedHash string) (string, error) {
	if strings.TrimSpace(expectedHash) == "" {
		return "", apperr.New(apperr.InvalidArgument).WithParam("field", "expectedHash")
	}
	return a.svc.SaveRuleSourceFile(ruleID, path, content, expectedHash)
}

// ListFileRoots returns the configured browser roots, not arbitrary filesystem paths.
func (a *API) ListFileRoots() ([]service.FileRoot, error) { return a.svc.ListFileRoots() }

// ListFileDirectory lists a directory under a configured browser root.
func (a *API) ListFileDirectory(rootID, path string) ([]service.FileDirectoryEntry, error) {
	return a.svc.ListFileDirectory(rootID, path)
}

// ReadBrowserFile reads a text file and returns the hash and resolved path needed to save it.
func (a *API) ReadBrowserFile(rootID, path string) (*service.RuleSourceContent, error) {
	return a.svc.ReadBrowserFile(rootID, path)
}

// SaveBrowserFile requires both concurrency guards returned by ReadBrowserFile.
func (a *API) SaveBrowserFile(rootID, path, content, expectedHash, expectedPath string) (string, error) {
	if strings.TrimSpace(expectedHash) == "" {
		return "", apperr.New(apperr.InvalidArgument).WithParam("field", "expectedHash")
	}
	if strings.TrimSpace(expectedPath) == "" {
		return "", apperr.New(apperr.InvalidArgument).WithParam("field", "expectedPath")
	}
	return a.svc.SaveBrowserFile(rootID, path, content, expectedHash, expectedPath)
}
