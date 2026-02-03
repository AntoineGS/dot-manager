# Nest Git Configuration Under Manager Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor git package configuration to nest branch and targets under the git manager for architectural consistency.

**Architecture:** Currently, `GitBranch` and `GitTargets` are top-level fields on the Package struct, while the git repo URL is stored in `Managers[Git]` as a string. This is inconsistent because git-specific configuration is split across multiple locations. The refactor will create a `GitConfig` struct to hold URL, branch, and targets, and change `Managers` from `map[PackageManager]string` to support both string values (for traditional managers) and structured config (for git). This provides a clean, nested structure where all git configuration lives under `managers.git`.

**Tech Stack:** Go 1.21+, gopkg.in/yaml.v3

**Key Changes:**
- Create `GitConfig` struct with URL, Branch, and Targets fields
- Change `Managers` field to support both string and GitConfig values
- Update all git package handling code to use nested structure
- Update tests to use new format
- Update documentation

---

## Task 1: Create GitConfig Struct and Update Package Type

**Files:**
- Modify: `internal/packages/packages.go:42-60`
- Test: `internal/packages/packages_test.go`

**Step 1: Write failing test for new git config structure**

Add to `internal/packages/packages_test.go` (replace existing `TestPackage_GitFields`):

```go
func TestPackage_GitConfig(t *testing.T) {
	pkg := Package{
		Name:        "my-dotfiles",
		Description: "My dotfiles repo",
		Git: &GitConfig{
			URL:    "https://github.com/user/dotfiles.git",
			Branch: "main",
			Targets: map[string]string{
				"linux":   "~/.dotfiles",
				"windows": "~/dotfiles",
			},
		},
	}

	if pkg.Git.URL != "https://github.com/user/dotfiles.git" {
		t.Errorf("Expected git repo URL, got %s", pkg.Git.URL)
	}

	if pkg.Git.Branch != "main" {
		t.Errorf("Expected branch 'main', got %s", pkg.Git.Branch)
	}

	if pkg.Git.Targets["linux"] != "~/.dotfiles" {
		t.Errorf("Expected linux target, got %s", pkg.Git.Targets["linux"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/packages/... -v -run TestPackage_GitConfig`
Expected: FAIL with compilation error about missing Git field

**Step 3: Add GitConfig struct before Package struct**

In `internal/packages/packages.go`, add after the PackageManager constants (around line 43):

```go
// GitConfig represents git-specific package configuration.
// It contains the repository URL, optional branch, and OS-specific clone destinations.
type GitConfig struct {
	URL     string            `yaml:"url"`
	Branch  string            `yaml:"branch,omitempty"`
	Targets map[string]string `yaml:"targets"`
}
```

**Step 4: Update Package struct**

In `internal/packages/packages.go:51-60`, modify the Package struct:

```go
type Package struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description,omitempty"`
	Managers    map[PackageManager]string `yaml:"managers,omitempty"`
	Git         *GitConfig                `yaml:"git,omitempty"` // Git package configuration
	Custom      map[string]string         `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall     `yaml:"url,omitempty"`    // OS -> URL install
	Filters     []config.Filter           `yaml:"filters,omitempty"`
}
```

Note: Remove `GitBranch` and `GitTargets` fields entirely.

**Step 5: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestPackage_GitConfig`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "refactor: add GitConfig struct to Package"
```

---

## Task 2: Update Install Method to Use GitConfig

**Files:**
- Modify: `internal/packages/packages.go:178-195` (Install method)
- Modify: `internal/packages/packages.go:377-404` (installGitPackage method)

**Step 1: Write failing test for git installation with new structure**

Update `TestManager_InstallGitPackage_Clone` in `internal/packages/packages_test.go`:

```go
func TestManager_InstallGitPackage_Clone(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	// Create bare repo for testing
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	// Initialize bare repo
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package with new structure
	pkg := Package{
		Name: "test-repo",
		Git: &GitConfig{
			URL: bareRepo,
			Targets: map[string]string{
				platform.OSLinux: cloneDest,
			},
		},
	}

	// Install
	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	if result.Method != "git" {
		t.Errorf("Expected method 'git', got: %s", result.Method)
	}

	// Verify clone exists
	if _, err := os.Stat(filepath.Join(cloneDest, ".git")); err != nil {
		t.Errorf("Expected .git directory to exist: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/packages/... -v -run TestManager_InstallGitPackage_Clone`
Expected: FAIL with "No installation method available"

**Step 3: Update Install method to check for Git field**

In `internal/packages/packages.go:178-195`, modify the Install method:

```go
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}

	// Check if this is a git package
	if pkg.Git != nil {
		result.Method = "git"
		success, msg := m.installGitPackage(pkg)
		result.Success = success
		result.Message = msg
		return result
	}

	// Try package managers first
	if len(pkg.Managers) > 0 {
		for _, mgr := range m.Available {
			if pkgName, ok := pkg.Managers[mgr]; ok {
				result.Method = string(mgr)
				success, msg := m.installWithManager(mgr, pkgName)
				result.Success = success
				result.Message = msg

				return result
			}
		}
	}

	// ... rest of method unchanged
}
```

**Step 4: Update installGitPackage method signature and implementation**

In `internal/packages/packages.go:377-404`, update the method:

```go
// installGitPackage clones or updates a git repository.
// If the target directory exists and contains a .git directory, it runs git pull.
// Otherwise, it clones the repository to the target path.
func (m *Manager) installGitPackage(pkg Package) (bool, string) {
	if pkg.Git == nil {
		return false, "No git configuration found"
	}

	// Get target path for current OS
	targetPath, ok := pkg.Git.Targets[m.OS]
	if !ok {
		return false, fmt.Sprintf("No git target path defined for OS: %s", m.OS)
	}

	// Expand path (handle ~)
	if strings.HasPrefix(targetPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, fmt.Sprintf("Failed to get home directory: %v", err)
		}
		targetPath = filepath.Join(home, targetPath[1:])
	}

	// Check if already cloned
	gitDir := filepath.Join(targetPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// Already cloned, do git pull
		return m.gitPull(targetPath)
	}

	// Not cloned yet, do git clone
	return m.gitClone(pkg.Git.URL, targetPath, pkg.Git.Branch)
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestManager_InstallGitPackage_Clone`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "refactor: update Install to use GitConfig struct"
```

---

## Task 3: Update Remaining Git Package Tests

**Files:**
- Modify: `internal/packages/packages_test.go`

**Step 1: Update TestManager_InstallGitPackage_Pull**

Replace the existing test (around line 1320-1370):

```go
func TestManager_InstallGitPackage_Pull(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	// Create bare repo and initial clone
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	// Initialize bare repo
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Clone manually first
	cmd = exec.Command("git", "clone", bareRepo, cloneDest)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to clone repo: %v", err)
	}

	// Create package manager
	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	// Create git package with new structure
	pkg := Package{
		Name: "test-repo",
		Git: &GitConfig{
			URL: bareRepo,
			Targets: map[string]string{
				platform.OSLinux: cloneDest,
			},
		},
	}

	// Install (should pull)
	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	if !strings.Contains(result.Message, "updated") {
		t.Errorf("Expected 'updated' in message, got: %s", result.Message)
	}
}
```

**Step 2: Update TestManager_InstallGitPackage_DryRun**

Replace the existing test (around line 1372-1400):

```go
func TestManager_InstallGitPackage_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run = true

	pkg := Package{
		Name: "test-repo",
		Git: &GitConfig{
			URL:    "https://github.com/test/repo.git",
			Branch: "main",
			Targets: map[string]string{
				platform.OSLinux: cloneDest,
			},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success in dry-run, got: %s", result.Message)
	}

	if !strings.Contains(result.Message, "Would run") {
		t.Errorf("Expected 'Would run' in dry-run message, got: %s", result.Message)
	}

	// Verify nothing was actually cloned
	if _, err := os.Stat(cloneDest); err == nil {
		t.Error("Expected no clone in dry-run mode, but directory exists")
	}
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./internal/packages/... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/packages/packages_test.go
git commit -m "refactor: update git package tests for GitConfig"
```

---

## Task 4: Update Integration Test

**Files:**
- Modify: `internal/integration/git_package_test.go`

**Step 1: Update TestGitPackageEndToEnd**

In `internal/integration/git_package_test.go` (around line 1199-1207), update the package creation:

```go
// Create package config
pkg := packages.Package{
	Name: "test-dotfiles",
	Git: &packages.GitConfig{
		URL: bareRepo,
		Targets: map[string]string{
			platform.OSLinux: cloneDest,
		},
	},
}
```

**Step 2: Run integration test**

Run: `go test ./internal/integration/... -v -run TestGitPackageEndToEnd`
Expected: PASS

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/integration/git_package_test.go
git commit -m "refactor: update integration test for GitConfig"
```

---

## Task 5: Update CLAUDE.md Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update configuration example**

Find the git package example (around line 73-79) and update it:

```yaml
      # Git package entry
      - name: "nvim-plugins"
        git:
          url: "https://github.com/user/plugins.git"
          branch: "main"
          targets:
            linux: "~/.local/share/nvim/site/pack/plugins/start/myplugins"
```

**Step 2: Update "Git as a Package Manager" section**

Find the section (around line 95-108) and update the example:

```markdown
### Git as a Package Manager

Git repositories can be installed as packages using the git package manager:

\```yaml
packages:
  - name: "dotfiles"
    git:
      url: "https://github.com/user/dotfiles.git"
      branch: "main"  # Optional, defaults to default branch
      targets:
        linux: "~/.dotfiles"
        windows: "~/dotfiles"
\```

**Behavior:**
- If target directory exists with `.git/`: runs `git pull` to update
- If target doesn't exist: clones repository with optional branch
- All git configuration is nested under the `git` field for consistency
```

**Step 3: Run tests to ensure everything still works**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for nested git config"
```

---

## Task 6: Final Verification and Build

**Files:**
- N/A (verification only)

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 2: Build the application**

Run: `go build ./cmd/dot-manager`
Expected: Build successful with no errors

**Step 3: Verify git log**

Run: `git log --oneline -6`
Expected: See 6 commits:
1. docs: update CLAUDE.md for nested git config
2. refactor: update integration test for GitConfig
3. refactor: update git package tests for GitConfig
4. refactor: update Install to use GitConfig struct
5. refactor: add GitConfig struct to Package

**Step 4: Create summary**

Document the changes:
- Created `GitConfig` struct with URL, Branch, Targets fields
- Updated `Package` struct to use `Git *GitConfig` instead of top-level fields
- Updated all installation logic to access `pkg.Git.URL`, `pkg.Git.Branch`, `pkg.Git.Targets`
- Updated all tests to use new nested structure
- Updated documentation to reflect new YAML format

**No commit needed for this task**

---

## Final Steps

1. **Review all changes**: `git log --oneline`
2. **Run full test suite**: `go test ./... -v`
3. **Build application**: `go build ./cmd/dot-manager`
4. **Test manually** with a sample config using nested git structure
5. **Create pull request** or merge to main

## Summary

This refactor improves architectural consistency by nesting all git-specific configuration under the `git` field in the Package struct. The new format is cleaner and more intuitive:

**Before:**
```yaml
packages:
  - name: "dotfiles"
    managers:
      git: "https://github.com/user/dotfiles.git"
    git_branch: "main"
    git_targets:
      linux: "~/.dotfiles"
```

**After:**
```yaml
packages:
  - name: "dotfiles"
    git:
      url: "https://github.com/user/dotfiles.git"
      branch: "main"
      targets:
        linux: "~/.dotfiles"
```

All git configuration is now in one place, making it easier to understand and maintain.
