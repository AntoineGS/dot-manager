package manager

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
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

	// Debug message should appear
	m.logVerbose("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Error("verbose logging not working")
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

func TestManager_SetVerbose(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Test enabling verbose
	m.SetVerbose(true)
	if !m.Verbose {
		t.Error("Verbose flag should be set to true")
	}

	// Test disabling verbose
	m.SetVerbose(false)
	if m.Verbose {
		t.Error("Verbose flag should be set to false")
	}
}

func TestManager_LogLevels(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	m = m.WithLogger(slog.New(handler))

	// Test different log levels
	m.log("info message")
	m.logWarn("warning message")
	m.logError("error message")

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
