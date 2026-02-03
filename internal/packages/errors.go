package packages

import (
	"errors"
	"fmt"
)

// Sentinel errors for package operations
var (
	ErrNoManagerAvailable = errors.New("no package manager available")
	ErrInstallFailed      = errors.New("package installation failed")
	ErrManagerNotFound    = errors.New("package manager not found")
	ErrDownloadFailed     = errors.New("download failed")
	ErrInvalidInstallSpec = errors.New("invalid installation specification")
)

// InstallError records a package installation failure
type InstallError struct {
	Package string         // Package name
	Manager PackageManager // Manager used
	Err     error          // Underlying error
}

func (e *InstallError) Error() string {
	return fmt.Sprintf("install %s via %s: %v", e.Package, e.Manager, e.Err)
}

func (e *InstallError) Unwrap() error {
	return e.Err
}

// NewInstallError creates a new InstallError
func NewInstallError(pkg string, mgr PackageManager, err error) *InstallError {
	return &InstallError{
		Package: pkg,
		Manager: mgr,
		Err:     err,
	}
}
