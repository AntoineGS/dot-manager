package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
	if err := os.MkdirAll(workDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Initialize repo
	cmd := exec.CommandContext(context.Background(), "git", "init")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Add test file
	testFile := filepath.Join(workDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Commit
	cmd = exec.CommandContext(context.Background(), "git", "add", ".")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.CommandContext(context.Background(), "git", "commit", "-m", "Initial commit")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Clone to bare
	cmd = exec.CommandContext(context.Background(), "git", "clone", "--bare", workDir, bareRepo) //nolint:gosec // test command
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create package config
	pkg := packages.Package{
		Name: "test-dotfiles",
		Managers: map[packages.PackageManager]string{
			packages.Git: bareRepo,
		},
		Git: &packages.GitConfig{
			URL: bareRepo,
			Targets: map[string]string{
				platform.OSLinux: cloneDest,
			},
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
	content, err := os.ReadFile(clonedFile) //nolint:gosec // test file
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
