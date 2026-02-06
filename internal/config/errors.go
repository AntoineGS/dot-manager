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

// Add appends an error to the ValidationErrors collection.
// If the provided error is nil, it is not added.
func (e *ValidationErrors) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if the ValidationErrors collection contains any errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Err   error
	Entry string
	Field string
	Value string
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
