package config

import (
	"errors"
	"testing"
)

func TestFieldError(t *testing.T) {
	baseErr := errors.New("invalid format")
	fieldErr := NewFieldError("myapp", "repo", "not-a-url", baseErr)

	var fe *FieldError
	if !errors.As(fieldErr, &fe) {
		t.Fatal("Should be FieldError type")
	}

	if fe.Entry != "myapp" {
		t.Errorf("Entry = %q, want %q", fe.Entry, "myapp")
	}

	if fe.Field != "repo" {
		t.Errorf("Field = %q, want %q", fe.Field, "repo")
	}

	if fe.Value != "not-a-url" {
		t.Errorf("Value = %q, want %q", fe.Value, "not-a-url")
	}

	if !errors.Is(fieldErr, baseErr) {
		t.Error("FieldError should wrap underlying error")
	}
}

func TestConfigSentinelErrors(t *testing.T) {
	tests := []struct {
		err  error
		want error
		name string
	}{
		{
			name: "unsupported_version",
			err:  ErrUnsupportedVersion,
			want: ErrUnsupportedVersion,
		},
		{
			name: "invalid_config",
			err:  ErrInvalidConfig,
			want: ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.want) {
				t.Errorf("errors.Is() = false, want true")
			}
		})
	}
}
