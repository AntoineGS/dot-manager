package manager

import (
	"errors"
	"fmt"
)

// Sentinel errors for common manager operations
var (
	ErrBackupNotFound   = errors.New("backup not found")
	ErrTargetExists     = errors.New("target already exists")
	ErrSymlinkFailed    = errors.New("symlink creation failed")
	ErrInvalidPath      = errors.New("invalid path")
	ErrPermissionDenied = errors.New("permission denied")
)

// PathError records an error and the operation and path that caused it.
type PathError struct {
	Op   string // Operation being performed (e.g., "restore", "backup")
	Path string // Path that caused the error
	Err  error  // Underlying error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *PathError) Unwrap() error {
	return e.Err
}

// NewPathError creates a new PathError
func NewPathError(op, path string, err error) *PathError {
	return &PathError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

// GitError records an error during git operations
type GitError struct {
	Repo   string // Repository URL
	Branch string // Branch name
	Op     string // Operation (clone, pull, checkout)
	Err    error  // Underlying error
}

func (e *GitError) Error() string {
	if e.Branch != "" {
		return fmt.Sprintf("git %s %s@%s: %v", e.Op, e.Repo, e.Branch, e.Err)
	}
	return fmt.Sprintf("git %s %s: %v", e.Op, e.Repo, e.Err)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

// NewGitError creates a new GitError
func NewGitError(op, repo, branch string, err error) *GitError {
	return &GitError{
		Op:     op,
		Repo:   repo,
		Branch: branch,
		Err:    err,
	}
}
