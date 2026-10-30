package apperr

import (
	"encoding/json"
	"fmt"
)

// Standard error codes.
const (
	RootNotFound            = "root_not_found"
	RootNotRepo             = "root_not_repo"
	GitNotInstalled         = "git_not_installed"
	GitNoRemote             = "git_no_remote"
	GitAuthFailed           = "git_auth_failed"
	GitNetwork              = "git_network"
	GitNotFastForward       = "git_not_fast_forward"
	GitRebaseConflict       = "git_rebase_conflict"
	GitDirtyOutsideProfiles = "git_dirty_outside_profiles"
	GitDirtyWorktree        = "git_dirty_worktree"
	GitIdentityMissing      = "git_identity_missing"
	GitPushRejected         = "git_push_rejected"
	TargetNotFound          = "target_not_found"
	TargetInsideRoot        = "target_inside_root"
	TargetAlreadyConnected  = "target_already_connected"
	ProfileNotFound         = "profile_not_found"
	ProfileAlreadyConnected = "profile_already_connected"
	PlanChanged             = "plan_changed"
	UnresolvedConflicts     = "unresolved_conflicts"
	BackupFailed            = "backup_failed"
	WriteFailed             = "write_failed"
	OperationInProgress     = "operation_in_progress"
	InvalidFolderName       = "invalid_folder_name"
	FolderExists            = "folder_exists"
	PathNotAbsolute         = "path_not_absolute"
	CloneFailed             = "clone_failed"
	DraftNotFound           = "draft_not_found"
	Unknown                 = "unknown"
)

// Error represents a structured domain error passed to the frontend.
type Error struct {
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
	Cause  error             `json:"-"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %v", e.Code, e.Cause)
	}
	if len(e.Params) > 0 {
		paramsJSON, _ := json.Marshal(e.Params)
		return fmt.Sprintf("[%s] %s", e.Code, string(paramsJSON))
	}
	return fmt.Sprintf("[%s]", e.Code)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error with the given code.
func New(code string) *Error {
	return &Error{
		Code:   code,
		Params: make(map[string]string),
	}
}

// Wrap creates a new Error wrapping an underlying cause.
func Wrap(code string, cause error) *Error {
	return &Error{
		Code:   code,
		Params: make(map[string]string),
		Cause:  cause,
	}
}

// WithParam adds a key-value parameter to the error.
func (e *Error) WithParam(key, value string) *Error {
	if e.Params == nil {
		e.Params = make(map[string]string)
	}
	e.Params[key] = value
	return e
}
