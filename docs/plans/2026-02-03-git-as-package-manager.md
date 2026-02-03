# Git as Package Manager Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove git entries and implement git as a package manager, treating repository clones as package installations.

**Architecture:** Git is fundamentally a package installation method, not a configuration management tool. This refactor completely removes git as an entry type and implements it as a package manager alongside pacman, apt, brew, etc. The Package struct will be extended to support git-specific fields (git_branch, git_targets), and the package manager will handle git clone/pull operations. All git entry fields, methods, and logic will be completely removed from the config and manager code with no backward compatibility.

**Tech Stack:** Go 1.21+, gopkg.in/yaml.v3, git command-line tool

**Key Changes:**
- Add `Git` as a `PackageManager` constant
- Extend `Package` struct with git fields (git_branch, git_targets)
- Add git installation logic in package manager (clone/pull)
- Remove all git entry handling from manager operations
- Remove git entry fields, methods, and constants from config types
- Remove all git entry tests, add comprehensive git package tests
- Update platform detection to include git
- Update CLAUDE.md documentation to reflect new architecture

---

## Task 1: Add Git Package Manager Constant and Platform Detection

**Files:**
- Modify: `internal/packages/packages.go:20-40`
- Modify: `internal/platform/platform.go:187-191`
- Test: `internal/platform/platform_test.go`

**Step 1: Write failing test for git package manager detection**

```go
// Add to internal/platform/platform_test.go
func TestDetectAvailableManagers_Git(t *testing.T) {
	if !IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	managers := DetectAvailableManagers()

	found := false
	for _, mgr := range managers {
		if mgr == "git" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected git to be in available managers, but it was not found")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/platform/... -v -run TestDetectAvailableManagers_Git`
Expected: FAIL with "Expected git to be in available managers"

**Step 3: Add Git constant to packages.go**

In `internal/packages/packages.go`, add after line 39 (after `Choco`):

```go
	// Git is the git package manager for repository clones
	Git PackageManager = "git"
```

**Step 4: Add git to KnownPackageManagers in platform.go**

In `internal/platform/platform.go:187-191`, modify the `KnownPackageManagers` slice:

```go
var KnownPackageManagers = []string{
	"pacman", "yay", "paru", "apt", "dnf", "brew",
	"winget", "scoop", "choco", "git",
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/platform/... -v -run TestDetectAvailableManagers_Git`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/packages/packages.go internal/platform/platform.go internal/platform/platform_test.go
git commit -m "feat: add git as package manager constant"
```

---

## Task 2: Extend Package Struct with Git Fields

**Files:**
- Modify: `internal/packages/packages.go:42-55`
- Test: `internal/packages/packages_test.go`

**Step 1: Write failing test for git package structure**

Create or add to `internal/packages/packages_test.go`:

```go
func TestPackage_GitFields(t *testing.T) {
	pkg := Package{
		Name:        "my-dotfiles",
		Description: "My dotfiles repo",
		Managers: map[PackageManager]string{
			Git: "https://github.com/user/dotfiles.git",
		},
		GitBranch: "main",
		GitTargets: map[string]string{
			"linux":   "~/.dotfiles",
			"windows": "~/dotfiles",
		},
	}

	if pkg.Managers[Git] != "https://github.com/user/dotfiles.git" {
		t.Errorf("Expected git repo URL, got %s", pkg.Managers[Git])
	}

	if pkg.GitBranch != "main" {
		t.Errorf("Expected branch 'main', got %s", pkg.GitBranch)
	}

	if pkg.GitTargets["linux"] != "~/.dotfiles" {
		t.Errorf("Expected linux target, got %s", pkg.GitTargets["linux"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/packages/... -v -run TestPackage_GitFields`
Expected: FAIL with compilation error about missing fields

**Step 3: Add git fields to Package struct**

In `internal/packages/packages.go:42-55`, modify the `Package` struct:

```go
type Package struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description,omitempty"`
	Managers    map[PackageManager]string `yaml:"managers,omitempty"`
	Custom      map[string]string         `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall     `yaml:"url,omitempty"`    // OS -> URL install
	GitBranch   string                    `yaml:"git_branch,omitempty"` // Optional branch for git repos
	GitTargets  map[string]string         `yaml:"git_targets,omitempty"` // OS -> clone destination path
	Filters     []config.Filter           `yaml:"filters,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestPackage_GitFields`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "feat: add git-specific fields to Package struct"
```

---

## Task 3: Implement Git Installation Logic in Package Manager

**Files:**
- Modify: `internal/packages/packages.go:178-219` (Install method)
- Modify: `internal/packages/packages.go:221-260` (installWithManager method)
- Test: `internal/packages/packages_test.go`

**Step 1: Write failing test for git clone operation**

Add to `internal/packages/packages_test.go`:

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

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]string{
			Git: bareRepo,
		},
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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

**Step 3: Modify Install method to handle git packages**

In `internal/packages/packages.go:178-219`, modify the `Install` method to check for git packages first:

```go
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}

	// Check if this is a git package
	if repoURL, ok := pkg.Managers[Git]; ok {
		result.Method = "git"
		success, msg := m.installGitPackage(pkg, repoURL)
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

	// Try custom command
	if cmd, ok := pkg.Custom[m.OS]; ok {
		result.Method = "custom"
		success, msg := m.runCustomCommand(cmd)
		result.Success = success
		result.Message = msg

		return result
	}

	// Try URL install
	if urlInstall, ok := pkg.URL[m.OS]; ok {
		result.Method = "url"
		success, msg := m.installFromURL(urlInstall)
		result.Success = success
		result.Message = msg

		return result
	}

	result.Success = false
	result.Message = "No installation method available for this OS/system"

	return result
}
```

**Step 4: Add installGitPackage method**

Add new method after `installFromURL` in `internal/packages/packages.go` (around line 361):

```go
// installGitPackage clones or updates a git repository.
// If the target directory exists and contains a .git directory, it runs git pull.
// Otherwise, it clones the repository to the target path.
func (m *Manager) installGitPackage(pkg Package, repoURL string) (bool, string) {
	// Get target path for current OS
	targetPath, ok := pkg.GitTargets[m.OS]
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
	return m.gitClone(repoURL, targetPath, pkg.GitBranch)
}

func (m *Manager) gitClone(repoURL, targetPath, branch string) (bool, string) {
	var cmd *exec.Cmd

	if branch != "" {
		cmd = exec.Command("git", "clone", "-b", branch, repoURL, targetPath)
	} else {
		cmd = exec.Command("git", "clone", repoURL, targetPath)
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Git clone failed: %v", err)
	}

	return true, "Repository cloned successfully"
}

func (m *Manager) gitPull(repoPath string) (bool, string) {
	cmd := exec.Command("git", "-C", repoPath, "pull")

	if m.DryRun {
		return true, fmt.Sprintf("Would run: git -C %s pull", repoPath)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Git pull failed: %v", err)
	}

	return true, "Repository updated successfully"
}
```

**Step 5: Add necessary imports**

At the top of `internal/packages/packages.go`, ensure these imports exist:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)
```

**Step 6: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestManager_InstallGitPackage_Clone`
Expected: PASS

**Step 7: Write test for git pull operation**

Add to `internal/packages/packages_test.go`:

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

	// Create git package
	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]string{
			Git: bareRepo,
		},
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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

**Step 8: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestManager_InstallGitPackage_Pull`
Expected: PASS

**Step 9: Add test for dry-run mode**

Add to `internal/packages/packages_test.go`:

```go
func TestManager_InstallGitPackage_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run = true

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]string{
			Git: "https://github.com/test/repo.git",
		},
		GitBranch: "main",
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
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

**Step 10: Run test to verify it passes**

Run: `go test ./internal/packages/... -v -run TestManager_InstallGitPackage_DryRun`
Expected: PASS

**Step 11: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "feat: implement git clone/pull in package manager"
```

---

## Task 4: Remove Git Entry Handling from Restore Operation

**Files:**
- Modify: `internal/manager/restore.go:68-103` (Remove git entry processing from v2)
- Modify: `internal/manager/restore.go:342-417` (Remove restoreGitEntry function)
- Modify: `internal/manager/restore.go:645-718` (Remove restoreGitSubEntry function)
- Modify: `internal/manager/restore.go:419-458` (Remove git entry processing from v3)

**Step 1: Write test to verify git entries are no longer processed**

Create `internal/manager/restore_no_git_test.go`:

```go
package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestRestore_IgnoresGitEntries(t *testing.T) {
	tmpDir := t.TempDir()
	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    2,
		BackupRoot: backupRoot,
		Entries: []config.Entry{
			{
				Name:   "config-entry",
				Backup: "./config",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "target"),
				},
			},
			{
				Name: "git-entry",
				Repo: "https://github.com/test/repo.git",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "git-target"),
				},
			},
		},
	}

	// Create backup for config entry
	configBackup := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(configBackup, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, "test.conf"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfg, platform.OSLinux, platform.DistroArch, false, false)
	results := mgr.Restore()

	// Verify config entry was processed
	configProcessed := false
	gitProcessed := false

	for _, r := range results {
		if r.Entry == "config-entry" {
			configProcessed = true
		}
		if r.Entry == "git-entry" {
			gitProcessed = true
		}
	}

	if !configProcessed {
		t.Error("Expected config entry to be processed")
	}

	if gitProcessed {
		t.Error("Did not expect git entry to be processed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manager/... -v -run TestRestore_IgnoresGitEntries`
Expected: FAIL with "Did not expect git entry to be processed"

**Step 3: Remove git entry handling from v2 restore**

In `internal/manager/restore.go:68-103`, remove the git entry processing section. The code should only process config entries:

```go
func (m *Manager) Restore() []RestoreResult {
	if m.Verbose {
		m.logVerbose("Starting restore operation")
	}

	// Handle v2 format
	if m.Config.Version == 2 {
		results := []RestoreResult{}

		// Process config entries only
		configEntries := m.GetEntries()
		for _, entry := range configEntries {
			if !entry.IsConfig() {
				continue // Skip non-config entries
			}

			osType := m.DetectOS()
			expandedTarget := entry.GetTarget(osType)
			if expandedTarget == "" {
				results = append(results, RestoreResult{
					Entry:   entry.Name,
					Success: false,
					Message: fmt.Sprintf("no target defined for OS: %s", osType),
				})
				continue
			}

			result := m.restoreEntry(entry, expandedTarget)
			results = append(results, result)
		}

		return results
	}

	// Handle v3 format
	return m.restoreV3()
}
```

**Step 4: Remove restoreGitEntry function**

Delete the entire `restoreGitEntry` function from `internal/manager/restore.go:342-417`.

**Step 5: Remove git entry handling from v3 restore**

In `internal/manager/restore.go:419-458`, find the v3 restore logic and remove git entry processing. The `restoreV3` function should only handle config entries:

```go
func (m *Manager) restoreV3() []RestoreResult {
	results := []RestoreResult{}

	for _, app := range m.GetApplications() {
		subEntries := m.GetFilteredSubEntries(app)

		for _, subEntry := range subEntries {
			// Only process config entries
			if !subEntry.IsConfig() {
				if m.Verbose {
					m.logVerbosef("Skipping %s/%s: not a config entry", app.Name, subEntry.Name)
				}
				continue
			}

			osType := m.DetectOS()
			expandedTarget := subEntry.GetTarget(osType)
			if expandedTarget == "" {
				results = append(results, RestoreResult{
					Entry:   fmt.Sprintf("%s/%s", app.Name, subEntry.Name),
					Success: false,
					Message: fmt.Sprintf("no target defined for OS: %s", osType),
				})
				continue
			}

			result := m.restoreConfigSubEntry(app, subEntry, expandedTarget)
			results = append(results, result)
		}
	}

	return results
}
```

**Step 6: Remove restoreGitSubEntry function**

Delete the entire `restoreGitSubEntry` function from `internal/manager/restore.go:645-718`.

**Step 7: Run test to verify it passes**

Run: `go test ./internal/manager/... -v -run TestRestore_IgnoresGitEntries`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/manager/restore.go internal/manager/restore_no_git_test.go
git commit -m "refactor: remove git entry handling from restore operation"
```

---

## Task 5: Update Backup Operation to Skip Non-Config Entries

**Files:**
- Modify: `internal/manager/backup.go:75-77`
- Test: Already skips git entries, verify behavior is correct

**Step 1: Write test to verify git entries are skipped in backup**

Add to `internal/manager/backup_test.go`:

```go
func TestBackup_SkipsGitEntries(t *testing.T) {
	tmpDir := t.TempDir()
	backupRoot := filepath.Join(tmpDir, "backup")

	cfg := &config.Config{
		Version:    2,
		BackupRoot: backupRoot,
		Entries: []config.Entry{
			{
				Name: "git-entry",
				Repo: "https://github.com/test/repo.git",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "source"),
				},
			},
		},
	}

	mgr := NewManager(cfg, platform.OSLinux, platform.DistroArch, false, false)
	results := mgr.Backup()

	// Should have no results (git entry skipped)
	if len(results) != 0 {
		t.Errorf("Expected no results for git entry, got %d", len(results))
	}
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./internal/manager/... -v -run TestBackup_SkipsGitEntries`
Expected: PASS (already working due to IsConfig() check)

**Step 3: Update log message for clarity**

In `internal/manager/backup.go:75-77`, update the message:

```go
if !subEntry.IsConfig() {
	m.logVerbosef("Skipping %s/%s: only config entries can be backed up", app.Name, subEntry.Name)
	continue
}
```

**Step 4: Commit**

```bash
git add internal/manager/backup.go internal/manager/backup_test.go
git commit -m "refactor: clarify that backup skips non-config entries"
```

---

## Task 6: Remove Git Entry Display from List Operation

**Files:**
- Modify: `internal/manager/list.go:59`

**Step 1: Write test to verify git entries are not listed**

Add to `internal/manager/list_test.go` (create if doesn't exist):

```go
package manager

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestList_SkipsGitEntries(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Version:    3,
		BackupRoot: tmpDir,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Type: "config",
						Name: "config-entry",
						Backup: "./config",
					},
					{
						Type: "git",
						Name: "git-entry",
						Repo: "https://github.com/test/repo.git",
					},
				},
			},
		},
	}

	mgr := NewManager(cfg, platform.OSLinux, platform.DistroArch, false, false)

	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mgr.List()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain config-entry
	if !strings.Contains(output, "config-entry") {
		t.Error("Expected config-entry in output")
	}

	// Should not contain git-entry
	if strings.Contains(output, "git-entry") {
		t.Error("Did not expect git-entry in output")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/manager/... -v -run TestList_SkipsGitEntries`
Expected: FAIL with "Did not expect git-entry in output"

**Step 3: Modify List method to skip non-config entries**

In `internal/manager/list.go`, find the v3 listing logic and add config check:

```go
func (m *Manager) List() {
	if m.Config.Version == 2 {
		// V2 format listing
		entries := m.GetEntries()
		for _, entry := range entries {
			if !entry.IsConfig() {
				continue // Skip non-config entries
			}
			// ... existing display logic
		}
		return
	}

	// V3 format listing
	for _, app := range m.GetApplications() {
		fmt.Printf("\n%s", app.Name)
		if app.Description != "" {
			fmt.Printf(" - %s", app.Description)
		}
		fmt.Println()

		subEntries := m.GetFilteredSubEntries(app)
		for _, entry := range subEntries {
			if !entry.IsConfig() {
				continue // Skip non-config entries
			}

			// Display config entry details
			fmt.Printf("├─ %s [%s]\n", entry.Name, entry.Type)
			// ... rest of display logic
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/manager/... -v -run TestList_SkipsGitEntries`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manager/list.go internal/manager/list_test.go
git commit -m "refactor: list only displays config entries"
```

---

## Task 7: Remove Git Entry Tests

**Files:**
- Modify: `internal/manager/restore_test.go` (remove all git-related tests)

**Step 1: Identify and remove git entry tests**

In `internal/manager/restore_test.go`, remove these test functions:
- `TestRestoreGitEntryDryRun` (line 238)
- `TestRestoreGitEntrySkipsExistingNonGit` (line 269)
- `TestRestoreGitEntry_Clone` (line 1274)
- `TestRestoreGitEntry_PullExisting` (line 1374)
- `TestRestoreV3_GitSubEntryDryRun` (line 548)
- `TestRestoreV3_MixedConfigAndGit` (line 1163)
- `TestRestoreGitSubEntry_Clone` (line 1462)
- `TestRestoreGitSubEntry_PullExisting` (line 1554)
- `TestRestoreGitSubEntry_SkipsNonGit` (line 1646)

**Step 2: Run all tests to verify no breakage**

Run: `go test ./internal/manager/... -v`
Expected: All tests PASS

**Step 3: Commit**

```bash
git add internal/manager/restore_test.go
git commit -m "refactor: remove git entry tests from manager"
```

---

## Task 8: Remove Git Entry Methods and Fields from Config

**Files:**
- Modify: `internal/config/config.go:129-150` (Remove GetGitEntries, GetFilteredGitEntries)
- Modify: `internal/config/config.go` (Remove GetAllGitSubEntries if exists)
- Modify: `internal/config/entry.go:3-8` (Remove SubEntryTypeGit constant)
- Modify: `internal/config/entry.go:14-25` (Remove Repo, Branch fields from Entry)
- Modify: `internal/config/entry.go:39-42` (Remove IsGit method from Entry)
- Modify: `internal/config/entry.go:140-150` (Remove Repo, Branch fields from SubEntry)
- Modify: `internal/config/entry.go:157-160` (Remove IsGit method from SubEntry)

**Step 1: Remove git entry methods from config.go**

In `internal/config/config.go:129-150`, remove the `GetGitEntries` and `GetFilteredGitEntries` methods entirely.

**Step 2: Search for GetAllGitSubEntries and remove it**

Run: `grep -n "GetAllGitSubEntries" internal/config/config.go`

If found, remove the method.

**Step 3: Search for usages of removed methods**

Run: `grep -r "GetGitEntries\|GetAllGitSubEntries" internal/`

If any usages are found, remove or refactor them.

**Step 4: Remove SubEntryTypeGit constant**

In `internal/config/entry.go:3-8`, remove:

```go
	// SubEntryTypeGit represents a git type sub-entry
	SubEntryTypeGit = "git"
```

Only keep `SubEntryTypeConfig`.

**Step 5: Remove git fields from Entry struct**

In `internal/config/entry.go:14-25`, remove these fields:

```go
	Repo        string            `yaml:"repo,omitempty"`
	Branch      string            `yaml:"branch,omitempty"`
```

**Step 6: Remove IsGit method from Entry**

In `internal/config/entry.go:39-42`, delete the entire `IsGit()` method.

**Step 7: Remove git fields from SubEntry struct**

In `internal/config/entry.go:140-150`, remove these fields:

```go
	Repo    string            `yaml:"repo,omitempty"`
	Branch  string            `yaml:"branch,omitempty"`
```

Also remove `Type` field since it's only used for distinguishing git vs config, and we only have config now.

**Step 8: Remove IsGit method from SubEntry**

In `internal/config/entry.go:157-160`, delete the entire `IsGit()` method.

**Step 9: Run all tests to verify no breakage**

Run: `go test ./internal/config/... -v`
Expected: All tests PASS

**Step 10: Commit**

```bash
git add internal/config/config.go internal/config/entry.go
git commit -m "refactor: completely remove git entry support from config"
```

---

## Task 9: Update CLAUDE.md Documentation

**Files:**
- Modify: `CLAUDE.md:28` (Remove git entry mention)
- Modify: `CLAUDE.md:33` (Update packages section to mention git)
- Modify: `CLAUDE.md:38` (Remove git entry type from key patterns)
- Modify: `CLAUDE.md:65-71` (Remove git entry example from v3 config)

**Step 1: Update architecture description**

In `CLAUDE.md:28`, change:

```markdown
- **internal/config/entry.go** - Entry type for config (symlinks) management
```

In `CLAUDE.md:33`, change:

```markdown
- **internal/packages/** - Multi-package-manager support (pacman, yay, paru, apt, dnf, brew, winget, scoop, choco, git)
```

**Step 2: Update key patterns section**

In `CLAUDE.md:38`, change to only mention config entries:

```markdown
- **Entry types**: Config entries (have `backup`) manage symlinks
```

Remove any line about git entries.

**Step 3: Update configuration format example**

In `CLAUDE.md:65-71`, update the example to show git as a package:

```yaml
# Application-level settings
applications:
  - name: "nvim"
    description: "Neovim text editor"

    configs:
      # Config entry (symlink management)
      - name: "nvim-config"
        files: []  # Empty = entire folder
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"

    packages:
      # Package entry
      - name: "neovim"
        managers:
          pacman: "neovim"
          apt: "neovim"
          brew: "neovim"

      # Git package entry
      - name: "nvim-plugins"
        managers:
          git: "https://github.com/user/plugins.git"
        git_branch: "main"
        git_targets:
          linux: "~/.local/share/nvim/site/pack/plugins/start/myplugins"

    filters:
      - include:
          os: "linux"
```

**Step 4: Add new section explaining git as package manager**

Add after the v3 configuration example in `CLAUDE.md`:

```markdown
### Git as a Package Manager

Git repositories can be installed as packages using the git package manager:

```yaml
packages:
  - name: "dotfiles"
    managers:
      git: "https://github.com/user/dotfiles.git"
    git_branch: "main"  # Optional, defaults to default branch
    git_targets:
      linux: "~/.dotfiles"
      windows: "~/dotfiles"
```

**Behavior:**
- If target directory exists with `.git/`: runs `git pull` to update
- If target doesn't exist: clones repository with optional branch
```

**Step 5: Run tests to ensure everything still works**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md to reflect git as package manager"
```

---

## Task 10: Final Integration Test

**Files:**
- Create: `internal/integration/git_package_test.go`

**Step 1: Create end-to-end integration test**

Create `internal/integration/git_package_test.go`:

```go
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/packages"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestGitPackageEndToEnd(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	// Create bare repo with a test file
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Add test file
	testFile := filepath.Join(workDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Clone to bare
	cmd = exec.Command("git", "clone", "--bare", workDir, bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create package config
	pkg := packages.Package{
		Name: "test-dotfiles",
		Managers: map[packages.PackageManager]string{
			packages.Git: bareRepo,
		},
		GitTargets: map[string]string{
			platform.OSLinux: cloneDest,
		},
	}

	// Create manager and install
	cfg := &packages.Config{Packages: []packages.Package{pkg}}
	mgr := packages.NewManager(cfg, platform.OSLinux, false, false)

	result := mgr.Install(pkg)

	// Verify success
	if !result.Success {
		t.Errorf("Installation failed: %s", result.Message)
	}

	if result.Method != "git" {
		t.Errorf("Expected method 'git', got: %s", result.Method)
	}

	// Verify clone
	clonedFile := filepath.Join(cloneDest, "test.txt")
	content, err := os.ReadFile(clonedFile)
	if err != nil {
		t.Errorf("Failed to read cloned file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Expected 'test content', got: %s", string(content))
	}

	// Test update (pull)
	result = mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Update failed: %s", result.Message)
	}
}
```

**Step 2: Run integration test**

Run: `go test ./internal/integration/... -v -run TestGitPackageEndToEnd`
Expected: PASS

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 4: Build the application**

Run: `go build ./cmd/dot-manager`
Expected: Build successful with no errors

**Step 5: Commit**

```bash
git add internal/integration/git_package_test.go
git commit -m "test: add end-to-end integration test for git packages"
```

---

## Final Steps

1. **Review all changes**: `git log --oneline`
2. **Run full test suite**: `go test ./... -v`
3. **Build application**: `go build ./cmd/dot-manager`
4. **Test manually** with a sample config using git packages
5. **Create pull request** or merge to main

## Summary

This refactor completely removes git entry support and implements git as a standard package manager. Git repositories are now installed via `dot-manager install` alongside other packages, providing a cleaner and more consistent architecture.
