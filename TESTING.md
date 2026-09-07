# Testing Guide

This document describes the testing strategy and practices for tidydots.

## Test Types

### Unit Tests

Standard Go unit tests covering individual functions and components.

**Run all unit tests:**
```bash
go test ./...
```

**Run tests for a specific package:**
```bash
go test ./internal/manager/...
```

**Run with verbose output:**
```bash
go test ./... -v
```

### Race Detection

On Linux, run the full suite with the Go race detector:

```bash
CGO_ENABLED=1 go test -race ./...
```

The race detector requires CGO and a C compiler (for example, GCC). CI runs
this as a separate required Linux job; the cross-platform test and coverage
jobs continue to use `CGO_ENABLED=0`.

### Snapshot Tests (TUI)

Golden file snapshot tests for the Bubble Tea TUI to catch visual regressions.

**Location:** `internal/tui/snapshot_test.go`

**What they test:**
- Text layout (ANSI codes stripped for stability)
- Different TUI states: basic list, expanded apps, multi-select, search
- Scrolling scenarios: middle, bottom, expanded apps

**Running snapshot tests:**
```bash
# Run snapshot tests
go test ./internal/tui -run TestScreenResults_Snapshots

# Update golden files (after intentional UI changes)
go test ./internal/tui -run TestScreenResults_Snapshots -update
```

**When to update golden files:**
- ✅ Intentional UI changes (new features, layout improvements)
- ✅ Bug fixes that change output (e.g., fixing scrolling)
- ❌ NOT for refactoring that doesn't change output

**Reviewing golden file changes:**
1. Check the diff in `git diff` - plain text changes should be visible
2. Ask: "Is this change intentional?"
3. If yes, commit the updated golden files
4. If no, fix the regression

### Integration Tests

Tests that verify multiple components working together.

**Examples:**
- `internal/integration/git_package_test.go` - Git package management
- `internal/manager/merge_integration_test.go` - Config merging

**Run integration tests:**
```bash
go test ./internal/integration/...
```

### Benchmark Tests

Performance benchmarks for critical paths.

**Run benchmarks:**
```bash
go test ./... -bench=.
```

## Test Patterns

### Table-Driven Tests

Use table-driven tests for multiple test cases:

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {"case1", "input1", "output1"},
        {"case2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Foo(tt.input)
            if got != tt.want {
                t.Errorf("Foo(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

### Filesystem Isolation

Use `t.TempDir()` for filesystem tests:

```go
func TestFileOperations(t *testing.T) {
    tmpDir := t.TempDir() // Automatically cleaned up
    testFile := filepath.Join(tmpDir, "test.txt")
    // ... test code ...
}
```

### Platform-Specific Tests

Copy-template fixtures must use `testutil.CanonicalTempDir(t)` when they need
symlink-free target ancestors. macOS's default temp root is under the `/var`
symlink; passing that lexical path would correctly fail copy-template safety
preflight before the behavior under test is reached. Canonicalize fixture roots
before creating intentional test symlinks; do not canonicalize deployment paths
in production to bypass the safety check.

Setting a fixture's platform to Linux does not change `runtime.GOOS`. Tests that
expect elevated Linux commands must use `skipIfNoSudo(t)`; unsupported-runtime
tests should assert rejection and no mutations, including during dry-run.

For Windows-portable fixtures, set both `HOME` and `USERPROFILE` with `t.Setenv`
when overriding the home directory: `os.UserHomeDir` uses the runtime OS, not the
fixture's platform. Build expected filesystem paths with `filepath.Join` and
normalize paths with `filepath.Clean` in filesystem error-injection wrappers;
database state keys should still be asserted with forward slashes. Avoid trailing
spaces in Windows fixture filenames while retaining Unix coverage for them.
Skip only POSIX-specific native permission tests on Windows, where `os.Chmod`
controls the read-only attribute rather than full permission bits; keep MemFS
permission tests and native error-propagation tests enabled.

From Linux, `GOOS=windows CGO_ENABLED=0 go test -exec /usr/bin/true ./...` checks
Windows test compilation only. It does not execute tests; Windows CI is still
required to verify runtime behavior.

Use build tags for platform-specific tests:

```go
//go:build linux
// +build linux

package manager

func TestLinuxSpecific(t *testing.T) {
    // ... linux-only test ...
}
```

## TUI Snapshot Testing Details

### How It Works

1. **Render** - Read `m.View().Content` to get the rendered TUI content
2. **Strip ANSI** - Remove color codes with `stripAnsiCodes()`
3. **Normalize** - Trim whitespace, consistent line endings with `normalizeOutput()`
4. **Compare** - Use goldie to compare against `.golden.txt` files

### Golden File Format

Golden files are plain text snapshots stored in `internal/tui/testdata/`:
- `basic_list.golden.txt` - Basic application list
- `app_expanded.golden.txt` - Expanded application
- `multi_select.golden.txt` - Multi-selection active
- `search_active.golden.txt` - Search filtering
- `scroll_middle.golden.txt` - Scrolling (middle position)
- `scroll_bottom.golden.txt` - Scrolling (bottom position)
- `scroll_with_expanded.golden.txt` - Scrolling with expanded app

### Gotchas to Avoid

1. **Color Profile Issues** - Tests set `NO_COLOR=1` with `t.Setenv` and strip ANSI codes before comparison. Snapshot tests are skipped on Windows because terminal rendering differs.

2. **Terminal Width Variations** - All setup functions set fixed dimensions:
   ```go
   m.width = 100
   m.height = 30
   ```

3. **Line Endings** - `.gitattributes` forces LF line endings:
   ```
   *.golden.txt text eol=lf
   ```

4. **Time-Dependent Output** - If UI adds timestamps, mock time in tests

### Adding New Snapshot Tests

1. Add test case to `TestScreenResults_Snapshots`:
   ```go
   {"my_new_test", setupMyNewTest},
   ```

2. Create setup function:
   ```go
   func setupMyNewTest(m *Model) {
       m.width = 100
       m.height = 30
       // ... customize model ...
   }
   ```

3. Generate golden file:
   ```bash
   go test ./internal/tui -run TestScreenResults_Snapshots/my_new_test -update
   ```

4. Verify test passes:
   ```bash
   go test ./internal/tui -run TestScreenResults_Snapshots/my_new_test
   ```

5. Review golden file content, then commit

## Test Infrastructure

### In-memory Filesystem (MemFS)

The `internal/fsys` package provides a `MemFS` implementation for testing filesystem operations without touching the real disk:

```go
import "github.com/AntoineGS/tidydots/internal/fsys"

func TestSomething(t *testing.T) {
    mem := fsys.NewMemFS()
    mem.WriteFile("/test.txt", []byte("hello"), 0600)
    // ... use mem as an fsys.FS
}
```

### Stub Command Runner

The `internal/cmdexec` package provides a `StubRunner` for testing code that executes external commands:

```go
import "github.com/AntoineGS/tidydots/internal/cmdexec"

func TestCommand(t *testing.T) {
    stub := cmdexec.NewStubRunner()
    stub.AddResult("git", cmdexec.Result{Stdout: []byte("v2.40.0"), ExitCode: 0})
    // ... use stub as a cmdexec.Runner
    // Verify stub.Calls to check what commands were executed
}
```

### Coverage Threshold

CI enforces a coverage floor via `coverage-threshold.txt`. To raise the floor after improving coverage, update the file to the new minimum and commit.

## Code Quality

### Linting

**REQUIRED:** Run golangci-lint after every change:

```bash
golangci-lint run
```

Use Go 1.26.1 or newer, as required by `go.mod`. The Makefile fallback installation
and CI pin golangci-lint to **v2.13.2**, compatible with Go 1.26 and the version-2
`.golangci.yml` configuration. To install that version explicitly:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
```

Ensure `$(go env GOPATH)/bin` (or your custom `GOBIN`) is on `PATH`.
`make lint` runs golangci-lint and govulncheck; `make lint-fix` runs
golangci-lint with `--fix`. Both install the pinned linter only when it is
missing, so update an existing older binary explicitly. Keep the Makefile
and CI version pins in sync when upgrading. Linting is enforced in CI.

### Test Coverage

Aim for high test coverage, especially for:
- Core business logic (manager, config)
- Public APIs
- Error paths

**Check coverage:**
```bash
go test ./... -cover
```

**Generate coverage report:**
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Continuous Integration

CI runs:
1. Tests with coverage on Linux, Windows, and macOS (`CGO_ENABLED=0`), with the coverage threshold enforced and Codecov upload attempted on Linux.
2. Linux race detection (`CGO_ENABLED=1 go test -race ./...`).
3. Linting with golangci-lint v2.13.2.
4. Vulnerability checks (`govulncheck ./...`).
5. GoReleaser configuration validation (`goreleaser check`).

The `CI Gate` requires every job above to succeed, including race detection;
failed, cancelled, or skipped dependencies fail the gate.
