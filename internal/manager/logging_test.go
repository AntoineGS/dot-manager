package manager

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestManager_StructuredLogging(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	m = m.WithLogger(slog.New(handler))

	// Trigger some logging
	m.logger.Info("test message",
		slog.String("entry", "test-entry"),
		slog.String("target", "/test/path"),
	)

	output := buf.String()

	// Verify structured format
	if !strings.Contains(output, "test message") {
		t.Error("missing message")
	}

	if !strings.Contains(output, "entry=test-entry") {
		t.Error("missing entry attribute")
	}

	if !strings.Contains(output, "target=/test/path") {
		t.Error("missing target attribute")
	}
}

func TestManager_VerboseLogging(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	m = m.WithLogger(slog.New(handler))

	// Debug message should appear with structured logging
	m.logger.Debug("debug message", slog.String("test", "value"))

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Error("verbose logging not working")
	}

	if !strings.Contains(output, "test=value") {
		t.Error("missing structured attribute")
	}
}

func TestManager_LogEntryRestore_Success(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	m = m.WithLogger(slog.New(handler))

	entry := config.Entry{Name: "test-entry", Backup: "./test"}
	m.logEntryRestore(entry, "/test/target", nil)

	output := buf.String()
	if !strings.Contains(output, "restore complete") {
		t.Error("missing success message")
	}

	if !strings.Contains(output, "entry=test-entry") {
		t.Error("missing entry attribute")
	}

	if !strings.Contains(output, "target=/test/target") {
		t.Error("missing target attribute")
	}
}

func TestManager_LogEntryRestore_Error(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	m = m.WithLogger(slog.New(handler))

	entry := config.Entry{Name: "test-entry", Backup: "./test"}
	testErr := NewPathError("restore", "/test/target", nil)
	m.logEntryRestore(entry, "/test/target", testErr)

	output := buf.String()
	if !strings.Contains(output, "restore failed") {
		t.Error("missing error message")
	}

	if !strings.Contains(output, "entry=test-entry") {
		t.Error("missing entry attribute")
	}

	if !strings.Contains(output, "target=/test/target") {
		t.Error("missing target attribute")
	}

	if !strings.Contains(output, "error=") {
		t.Error("missing error attribute")
	}
}

func TestManager_WithVerbose(t *testing.T) {
	t.Parallel()
	// Create a manager without setupTestManager to avoid Verbose=true
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux}
	m := New(cfg, plat)

	// Verify starting state
	if m.Verbose {
		t.Error("Manager should start with Verbose=false")
	}

	// Test enabling verbose
	m2 := m.WithVerbose(true)

	if !m2.Verbose {
		t.Error("Verbose flag should be set to true")
	}

	// Verify original manager is unchanged (immutable pattern)
	if m.Verbose {
		t.Error("Original manager should be unchanged")
	}

	// Test disabling verbose
	m3 := m2.WithVerbose(false)

	if m3.Verbose {
		t.Error("Verbose flag should be set to false")
	}
}

func TestManager_LogLevels(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	m = m.WithLogger(slog.New(handler))

	// Test different log levels with structured logging
	m.logger.Info("info message", slog.String("level", "info"))
	m.logger.Warn("warning message", slog.String("level", "warn"))
	m.logger.Error("error message", slog.String("level", "error"))

	output := buf.String()
	if !strings.Contains(output, "info message") {
		t.Error("missing info message")
	}

	if !strings.Contains(output, "warning message") {
		t.Error("missing warning message")
	}

	if !strings.Contains(output, "error message") {
		t.Error("missing error message")
	}
}
