# Testing Guide

This document describes the testing strategy and practices for dot-manager.

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

1. **Render** - Call `m.View()` to render the TUI state
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

1. **Color Profile Issues** - Tests force ASCII color profile with `lipgloss.SetColorProfile(termenv.Ascii)` to ensure consistent rendering across environments

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

## Code Quality

### Linting

**REQUIRED:** Run golangci-lint after every change:

```bash
golangci-lint run
```

This is enforced in CI and pre-commit hooks.

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
1. All tests (`go test ./...`)
2. Linting (`golangci-lint run`)
3. Build verification (`go build ./cmd/dot-manager`)
