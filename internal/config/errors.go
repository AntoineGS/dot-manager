package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for config operations
var (
	ErrUnsupportedVersion = errors.New("unsupported config version")
	ErrInvalidConfig      = errors.New("invalid configuration")
)

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
