package config

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for config operations
var (
	ErrUnsupportedVersion = errors.New("unsupported config version")
	ErrInvalidConfig      = errors.New("invalid configuration")
	ErrFilterMismatch     = errors.New("filter criteria not met")
)

// ValidationErrors holds multiple validation errors
type ValidationErrors struct {
	Errors []error
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(msgs, "; "))
}

func (e *ValidationErrors) Add(err error) {
	e.Errors = append(e.Errors, err)
}

func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Entry string // Entry name
	Field string // Field name
	Value string // Invalid value
	Err   error  // Underlying error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("entry %s: field %s (%s): %v", e.Entry, e.Field, e.Value, e.Err)
}

func (e *FieldError) Unwrap() error {
	return e.Err
}

// NewFieldError creates a new FieldError
func NewFieldError(entry, field, value string, err error) *FieldError {
	return &FieldError{
		Entry: entry,
		Field: field,
		Value: value,
		Err:   err,
	}
}
