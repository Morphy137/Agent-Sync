package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

type FileRootsOutput struct {
	Roots []service.FileRoot `json:"roots"`
}
type FileDirectoryOutput struct {
	Entries []service.FileDirectoryEntry `json:"entries"`
}
type FileHashOutput struct {
	Hash string `json:"hash"`
}

type BrowserFileInput struct {
	RootID string `json:"rootId" jsonschema:"configured root id from list_file_roots; repo, a subscription id, or rule:ID for an existing local rule source"`
	Path   string `json:"path" jsonschema:"relative file path within the configured root"`
}
type FileDirectoryInput struct {
	RootID string `json:"rootId" jsonschema:"configured root id"`
	Path   string `json:"path,omitempty" jsonschema:"relative directory path; omit for root"`
}
type SaveBrowserFileInput struct {
	RootID       string `json:"rootId" jsonschema:"configured root id"`
	Path         string `json:"path" jsonschema:"relative file path returned by the browser"`
	Content      string `json:"content" jsonschema:"complete replacement text; may be empty"`
	ExpectedHash string `json:"expectedHash" jsonschema:"hash from read_file; required to reject concurrent edits"`
	ExpectedPath string `json:"expectedPath" jsonschema:"absolutePath from read_file; required to detect a changed root or resolved path"`
}

type RuleSourceFilesOutput struct {
	Files []service.RuleSourceFile `json:"files"`
}
type RuleSourceFileInput struct {
	RuleID string `json:"ruleId" jsonschema:"existing rule id"`
	Path   string `json:"path" jsonschema:"selected relative path from list_rule_source_files; dot for a single-file source"`
}
type SaveRuleSourceFileInput struct {
	RuleID       string `json:"ruleId" jsonschema:"existing rule id"`
	Path         string `json:"path" jsonschema:"selected relative path from list_rule_source_files"`
	Content      string `json:"content" jsonschema:"complete replacement text; may be empty"`
	ExpectedHash string `json:"expectedHash" jsonschema:"hash from read_rule_source_file"`
}

type ImportIntoRepoInput struct {
	SourcePath string `json:"sourcePath" jsonschema:"absolute local file or folder to copy into the selected repository"`
	RepoRelDir string `json:"repoRelDir,omitempty" jsonschema:"destination directory relative to repository root; omit for root"`
}
type ImportedFileOutput struct {
	Path string `json:"path"`
}
type SubscriptionFileDiffInput struct {
	ID   string `json:"id" jsonschema:"subscription id"`
	Path string `json:"path" jsonschema:"relative file path from subscription changes"`
}

func (s *Server) registerFileTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "import_into_repo", Description: "Copy a local file or folder into the selected repository directory. Existing targets are never overwritten; creates no rule and does not push.", Annotations: additive}, handler(func(in ImportIntoRepoInput) (ImportedFileOutput, error) {
		path, err := a.ImportIntoRepo(in.SourcePath, in.RepoRelDir)
		return ImportedFileOutput{Path: path}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "subscription_file_diff", Description: "Compare a subscription file in its local clone with the last fetched remote version. Call check_subscription first for current incoming changes. No fetch or write.", Annotations: readOnly}, handler(func(in SubscriptionFileDiffInput) (rulesync.Diff, error) {
		return deref(a.SubscriptionFileDiff(in.ID, in.Path))
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "list_rule_source_files", Description: "List source files selected by an existing rule, respecting its scope and filters.", Annotations: readOnly}, handler(func(in RuleIDInput) (RuleSourceFilesOutput, error) {
		files, err := a.ListRuleSourceFiles(in.RuleID)
		return RuleSourceFilesOutput{Files: files}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "read_rule_source_file", Description: "Read a selected rule source file and its concurrency hash. Subscription rule sources are read-only.", Annotations: readOnly}, handler(func(in RuleSourceFileInput) (service.RuleSourceContent, error) {
		return deref(a.ReadRuleSourceFile(in.RuleID, in.Path))
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "save_rule_source_file", Description: "Replace a selected rule source file only if expectedHash matches its last read. Subscription rule sources cannot be saved; use the file browser for explicit local clone edits.", Annotations: writesFiles}, handler(func(in SaveRuleSourceFileInput) (FileHashOutput, error) {
		hash, err := a.SaveRuleSourceFile(in.RuleID, in.Path, in.Content, in.ExpectedHash)
		return FileHashOutput{Hash: hash}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "list_file_roots", Description: "List configured repository and subscription browser roots.", Annotations: readOnly}, handler(func(NoInput) (FileRootsOutput, error) {
		roots, err := a.ListFileRoots()
		return FileRootsOutput{Roots: roots}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "list_file_directory", Description: "List a directory within a configured browser root. Traversal and escaping symlinks are rejected.", Annotations: readOnly}, handler(func(in FileDirectoryInput) (FileDirectoryOutput, error) {
		entries, err := a.ListFileDirectory(in.RootID, in.Path)
		return FileDirectoryOutput{Entries: entries}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "read_file", Description: "Read browser text and its hash and absolutePath for a guarded save. Subscription browser files permit local edits.", Annotations: readOnly}, handler(func(in BrowserFileInput) (service.RuleSourceContent, error) {
		return deref(a.ReadBrowserFile(in.RootID, in.Path))
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{Name: "save_file", Description: "Replace browser file text only if expectedHash and expectedPath still match read_file. Subscription edits stay local and may block subscription updates until resolved.", Annotations: writesFiles}, handler(func(in SaveBrowserFileInput) (FileHashOutput, error) {
		hash, err := a.SaveBrowserFile(in.RootID, in.Path, in.Content, in.ExpectedHash, in.ExpectedPath)
		return FileHashOutput{Hash: hash}, err
	}))
}
