# Git Config Under Managers Map Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Properly nest git configuration under `managers.git` to match the structure of other package managers.

**Architecture:** The current implementation has `Git *GitConfig` as a separate top-level field. The correct design is to nest git configuration under `managers.git` alongside other package managers like pacman, apt, etc. Since `Managers` is currently `map[PackageManager]string`, we need to change it to support both string values (for traditional managers) and structured config (for git). We'll use a custom YAML unmarshaler to handle both types seamlessly.

**Tech Stack:** Go 1.21+, gopkg.in/yaml.v3

**Target Format:**
```yaml
packages:
  - name: "dotfiles"
    managers:
      git:
        url: "https://github.com/user/dotfiles.git"
        branch: "main"
        targets:
          linux: "~/.dotfiles"
        sudo: false
      pacman: "some-package"
      apt: "some-package"
```

**Key Changes:**
- Change `Managers` from `map[PackageManager]string` to `map[PackageManager]interface{}`
- Add custom UnmarshalYAML to handle both string and GitConfig values
- Remove separate `Git *GitConfig` field
- Add `Sudo` field to GitConfig for privilege escalation
- Update all code to handle managers.git
- Update tests and documentation

---

## Task 1: Add Sudo Field to GitConfig and Update Managers Type

**Files:**
- Modify: `internal/packages/packages.go:45-68`
- Test: `internal/packages/packages_test.go`

**Step 1: Write failing test for managers.git structure**

In `internal/packages/packages_test.go`, replace `TestPackage_GitConfig`:

```go
func TestPackage_GitConfigInManagers(t *testing.T) {
	pkg := Package{
		Name:        "my-dotfiles",
		Description: "My dotfiles repo",
		Managers: map[PackageManager]interface{}{
			Pacman: "neovim",
			Git: GitConfig{
				URL:    "https://github.com/user/dotfiles.git",
				Branch: "main",
				Targets: map[string]string{
					"linux":   "~/.dotfiles",
					"windows": "~/dotfiles",
				},
				Sudo: false,
			},
		},
	}

	// Check traditional manager (string)
	if pkg.Managers[Pacman] != "neovim" {
		t.Errorf("Expected pacman package name, got %v", pkg.Managers[Pacman])
	}

	// Check git manager (GitConfig)
	gitCfg, ok := pkg.Managers[Git].(GitConfig)
	if !ok {
		t.Fatal("Expected git manager to be GitConfig")
	}

	if gitCfg.URL != "https://github.com/user/dotfiles.git" {
		t.Errorf("Expected git repo URL, got %s", gitCfg.URL)
	}

	if gitCfg.Branch != "main" {
		t.Errorf("Expected branch 'main', got %s", gitCfg.Branch)
	}

	if gitCfg.Targets["linux"] != "~/.dotfiles" {
		t.Errorf("Expected linux target, got %s", gitCfg.Targets["linux"])
	}
}
```

**Step 2: Run test - should fail with compilation error**

**Step 3: Add Sudo field to GitConfig**

In `internal/packages/packages.go:45-51`, update GitConfig:

```go
// GitConfig represents git-specific package configuration.
// It contains the repository URL, optional branch, OS-specific clone destinations, and sudo flag.
type GitConfig struct {
	URL     string            `yaml:"url"`
	Branch  string            `yaml:"branch,omitempty"`
	Targets map[string]string `yaml:"targets"`
	Sudo    bool              `yaml:"sudo,omitempty"`
}
```

**Step 4: Change Managers type to interface{}**

In `internal/packages/packages.go:61-68`, update Package struct:

```go
type Package struct {
	Name        string                       `yaml:"name"`
	Description string                       `yaml:"description,omitempty"`
	Managers    map[PackageManager]interface{} `yaml:"managers,omitempty"`
	Custom      map[string]string            `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall        `yaml:"url,omitempty"`    // OS -> URL install
	Filters     []config.Filter              `yaml:"filters,omitempty"`
}
```

Note: Remove the `Git *GitConfig` field entirely.

**Step 5: Run test - should still fail with YAML unmarshaling issues**

**Step 6: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "refactor: change Managers to support interface{} values"
```

---

## Task 2: Implement Custom YAML Unmarshaling for Package

**Files:**
- Modify: `internal/packages/packages.go` (add UnmarshalYAML method)
- Test: `internal/packages/packages_test.go`

**Step 1: Write test for YAML unmarshaling**

Add to `internal/packages/packages_test.go`:

```go
func TestPackage_UnmarshalYAML(t *testing.T) {
	yamlData := `
name: "test-pkg"
managers:
  pacman: "neovim"
  git:
    url: "https://github.com/user/repo.git"
    branch: "main"
    targets:
      linux: "~/.dotfiles"
    sudo: true
`

	var pkg Package
	err := yaml.Unmarshal([]byte(yamlData), &pkg)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify pacman manager (string)
	pacmanPkg, ok := pkg.Managers[Pacman].(string)
	if !ok {
		t.Fatal("Expected pacman to be string")
	}
	if pacmanPkg != "neovim" {
		t.Errorf("Expected 'neovim', got %s", pacmanPkg)
	}

	// Verify git manager (GitConfig)
	gitCfg, ok := pkg.Managers[Git].(GitConfig)
	if !ok {
		t.Fatalf("Expected git to be GitConfig, got %T", pkg.Managers[Git])
	}

	if gitCfg.URL != "https://github.com/user/repo.git" {
		t.Errorf("Expected URL, got %s", gitCfg.URL)
	}

	if !gitCfg.Sudo {
		t.Error("Expected sudo to be true")
	}
}
```

**Step 2: Run test - should fail**

**Step 3: Add UnmarshalYAML method to Package**

Add after the Package struct definition in `internal/packages/packages.go`:

```go
// UnmarshalYAML implements custom YAML unmarshaling for Package.
// It handles the Managers field specially to support both string values (for traditional
// package managers) and GitConfig structs (for git repositories).
func (p *Package) UnmarshalYAML(node *yaml.Node) error {
	// Define a temporary struct that matches Package but with a different Managers type
	type packageAlias struct {
		Name        string                  `yaml:"name"`
		Description string                  `yaml:"description,omitempty"`
		Managers    map[string]yaml.Node    `yaml:"managers,omitempty"`
		Custom      map[string]string       `yaml:"custom,omitempty"`
		URL         map[string]URLInstall   `yaml:"url,omitempty"`
		Filters     []config.Filter         `yaml:"filters,omitempty"`
	}

	var alias packageAlias
	if err := node.Decode(&alias); err != nil {
		return err
	}

	// Copy simple fields
	p.Name = alias.Name
	p.Description = alias.Description
	p.Custom = alias.Custom
	p.URL = alias.URL
	p.Filters = alias.Filters

	// Process managers map
	p.Managers = make(map[PackageManager]interface{})
	for key, valueNode := range alias.Managers {
		pm := PackageManager(key)

		// Special handling for git manager
		if pm == Git {
			var gitCfg GitConfig
			if err := valueNode.Decode(&gitCfg); err != nil {
				return fmt.Errorf("failed to decode git config: %w", err)
			}
			p.Managers[pm] = gitCfg
		} else {
			// Traditional managers are strings
			var pkgName string
			if err := valueNode.Decode(&pkgName); err != nil {
				return fmt.Errorf("failed to decode manager %s: %w", key, err)
			}
			p.Managers[pm] = pkgName
		}
	}

	return nil
}
```

**Step 4: Add necessary import**

Ensure `gopkg.in/yaml.v3` is imported as `yaml`.

**Step 5: Run test - should pass**

**Step 6: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "feat: add custom YAML unmarshaling for Package"
```

---

## Task 3: Update Install Method to Use managers.git

**Files:**
- Modify: `internal/packages/packages.go:178-229` (Install method)
- Modify: `internal/packages/packages.go:387-417` (installGitPackage method)
- Modify: `internal/packages/packages.go:419-447` (gitClone method)
- Test: `internal/packages/packages_test.go`

**Step 1: Update test to use managers.git**

Update `TestManager_InstallGitPackage_Clone`:

```go
func TestManager_InstallGitPackage_Clone(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]interface{}{
			Git: GitConfig{
				URL: bareRepo,
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: false,
			},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	if result.Method != "git" {
		t.Errorf("Expected method 'git', got: %s", result.Method)
	}

	if _, err := os.Stat(filepath.Join(cloneDest, ".git")); err != nil {
		t.Errorf("Expected .git directory to exist: %v", err)
	}
}
```

**Step 2: Run test - should fail**

**Step 3: Update Install method to check managers.git**

In `internal/packages/packages.go:178-229`:

```go
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}

	// Check if this is a git package
	if gitValue, ok := pkg.Managers[Git]; ok {
		if gitCfg, ok := gitValue.(GitConfig); ok {
			result.Method = "git"
			success, msg := m.installGitPackage(gitCfg)
			result.Success = success
			result.Message = msg
			return result
		}
	}

	// Try package managers first
	if len(pkg.Managers) > 0 {
		for _, mgr := range m.Available {
			if pkgValue, ok := pkg.Managers[mgr]; ok {
				// Skip git manager (already handled above)
				if mgr == Git {
					continue
				}

				// Traditional managers have string values
				pkgName, ok := pkgValue.(string)
				if !ok {
					continue
				}

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

**Step 4: Update installGitPackage signature**

In `internal/packages/packages.go:387-417`:

```go
// installGitPackage clones or updates a git repository.
func (m *Manager) installGitPackage(gitCfg GitConfig) (bool, string) {
	// Get target path for current OS
	targetPath, ok := gitCfg.Targets[m.OS]
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
		return m.gitPull(targetPath, gitCfg.Sudo)
	}

	return m.gitClone(gitCfg.URL, targetPath, gitCfg.Branch, gitCfg.Sudo)
}
```

**Step 5: Update gitClone and gitPull to support sudo**

Update `gitClone` in `internal/packages/packages.go:419-447`:

```go
func (m *Manager) gitClone(repoURL, targetPath, branch string, sudo bool) (bool, string) {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, repoURL, targetPath)

	var cmd *exec.Cmd
	if sudo {
		// Prepend sudo to the command
		args = append([]string{"git"}, args...)
		cmd = exec.Command("sudo", args...)
	} else {
		cmd = exec.Command("git", args...)
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

func (m *Manager) gitPull(repoPath string, sudo bool) (bool, string) {
	var cmd *exec.Cmd
	if sudo {
		cmd = exec.Command("sudo", "git", "-C", repoPath, "pull")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "pull")
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Git pull failed: %v", err)
	}

	return true, "Repository updated successfully"
}
```

**Step 6: Run test - should pass**

**Step 7: Commit**

```bash
git add internal/packages/packages.go internal/packages/packages_test.go
git commit -m "refactor: update Install to use managers.git with sudo support"
```

---

## Task 4: Update Remaining Tests

**Files:**
- Modify: `internal/packages/packages_test.go`
- Modify: `internal/integration/git_package_test.go`

**Step 1: Update TestManager_InstallGitPackage_Pull**

```go
func TestManager_InstallGitPackage_Pull(t *testing.T) {
	if !platform.IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "test-repo.git")
	cloneDest := filepath.Join(tmpDir, "cloned")

	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	cmd = exec.Command("git", "clone", bareRepo, cloneDest)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to clone repo: %v", err)
	}

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, false, false)

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]interface{}{
			Git: GitConfig{
				URL: bareRepo,
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
			},
		},
	}

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

```go
func TestManager_InstallGitPackage_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false)

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]interface{}{
			Git: GitConfig{
				URL:    "https://github.com/test/repo.git",
				Branch: "main",
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
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

	if _, err := os.Stat(cloneDest); err == nil {
		t.Error("Expected no clone in dry-run mode, but directory exists")
	}
}
```

**Step 3: Add sudo test**

```go
func TestManager_InstallGitPackage_WithSudo(t *testing.T) {
	tmpDir := t.TempDir()
	cloneDest := filepath.Join(tmpDir, "cloned")

	cfg := &Config{Packages: []Package{}}
	mgr := NewManager(cfg, platform.OSLinux, true, false) // dry-run to avoid actual sudo

	pkg := Package{
		Name: "test-repo",
		Managers: map[PackageManager]interface{}{
			Git: GitConfig{
				URL: "https://github.com/test/repo.git",
				Targets: map[string]string{
					platform.OSLinux: cloneDest,
				},
				Sudo: true,
			},
		},
	}

	result := mgr.Install(pkg)

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	// Verify sudo is in the command
	if !strings.Contains(result.Message, "sudo") {
		t.Errorf("Expected 'sudo' in command, got: %s", result.Message)
	}
}
```

**Step 4: Update integration test**

In `internal/integration/git_package_test.go`:

```go
pkg := packages.Package{
	Name: "test-dotfiles",
	Managers: map[packages.PackageManager]interface{}{
		packages.Git: packages.GitConfig{
			URL: bareRepo,
			Targets: map[string]string{
				platform.OSLinux: cloneDest,
			},
		},
	},
}
```

**Step 5: Run all tests**

```bash
go test ./... -v
```

**Step 6: Commit**

```bash
git add internal/packages/packages_test.go internal/integration/git_package_test.go
git commit -m "refactor: update all tests for managers.git structure"
```

---

## Task 5: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update configuration examples**

Update the v3 config example:

```yaml
    packages:
      - name: "neovim"
        managers:
          pacman: "neovim"
          apt: "neovim"
          brew: "neovim"

      # Git package entry
      - name: "nvim-plugins"
        managers:
          git:
            url: "https://github.com/user/plugins.git"
            branch: "main"
            targets:
              linux: "~/.local/share/nvim/site/pack/plugins/start/myplugins"
            sudo: false
```

**Step 2: Update "Git as a Package Manager" section**

```markdown
### Git as a Package Manager

Git repositories can be installed as packages by adding git to the managers map:

\```yaml
packages:
  - name: "dotfiles"
    managers:
      git:
        url: "https://github.com/user/dotfiles.git"
        branch: "main"  # Optional
        targets:
          linux: "~/.dotfiles"
          windows: "~/dotfiles"
        sudo: false  # Optional, use true for system-level installs
\```

**Fields:**
- `url`: Repository URL (required)
- `branch`: Branch to clone (optional, defaults to default branch)
- `targets`: OS-specific clone destinations (required)
- `sudo`: Run git commands with sudo (optional, default false)

**Behavior:**
- If target directory exists with `.git/`: runs `git pull` to update
- If target doesn't exist: clones repository
- Git configuration is nested under `managers.git` for consistency with other package managers
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for managers.git structure"
```

---

## Task 6: Final Verification

**Files:**
- N/A (verification only)

**Step 1: Run full test suite**

```bash
go test ./... -v
```

**Step 2: Build application**

```bash
go build ./cmd/dot-manager
```

**Step 3: Verify structure**

Create a test config and verify it works:

```yaml
packages:
  - name: "test"
    managers:
      git:
        url: "https://github.com/test/repo.git"
        targets:
          linux: "/tmp/test-repo"
      pacman: "neovim"
```

**Step 4: Document changes**

Final structure achieved:
- Git config properly nested under `managers.git`
- Supports both string (traditional) and GitConfig (git) manager values
- Custom YAML unmarshaling handles both types
- Sudo support for system-level git installs
- All tests passing

---

## Summary

This plan properly implements git as a package manager with configuration nested under `managers.git`:

**Final Format:**
```yaml
packages:
  - name: "my-package"
    managers:
      git:
        url: "https://github.com/user/repo.git"
        branch: "main"
        targets:
          linux: "~/.local/share/repo"
        sudo: false
      pacman: "neovim"
      apt: "neovim"
```

**Benefits:**
- Git is truly just another package manager
- Consistent structure with other managers
- Sudo support for system-level installs
- Type-safe with custom unmarshaling
- Clean, intuitive YAML format
