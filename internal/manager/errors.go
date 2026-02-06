package manager

import (
	"errors"
	"fmt"
)

// Sentinel errors for common manager operations
var (
	ErrBackupNotFound = errors.New("backup not found")
	ErrTargetExists   = errors.New("target already exists")
)

// PathError records an error and the operation and path that caused it.
type PathError struct {
	Err  error
	Op   string
	Path string
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
