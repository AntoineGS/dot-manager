# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
go build ./cmd/dot-manager

# Run tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run a single package's tests
go test ./internal/manager/...
```

## Architecture

**dot-manager** is a cross-platform dotfile management tool written in Go. It manages configuration files through symlinks, supports backup/restore operations, and handles package installation across multiple package managers.

### Core Components

- **cmd/dot-manager/main.go** - Cobra CLI entry point defining all commands (init, restore, backup, list, install, list-packages)
- **internal/config/** - Two-level YAML configuration: app config (`~/.config/dot-manager/config.yaml`) and repo config (`dot-manager.yaml`)
- **internal/manager/** - Core operations (backup, restore, adopt, list) with platform-aware path selection
- **internal/platform/** - OS detection (Linux/Windows), root/sudo detection, Arch Linux detection
- **internal/tui/** - Bubble Tea-based interactive terminal UI with Lipgloss styling
- **internal/packages/** - Multi-package-manager support (pacman, yay, paru, apt, dnf, brew, winget, scoop, choco)

### Key Patterns

- **Platform-aware execution**: Uses `paths` for regular users, `root_paths` for root/sudo operations
- **Symlink-based restoration**: Configs are symlinked from the dotfiles repo rather than copied
- **Dry-run mode**: All operations support `-n` flag for safe preview
- **Table-driven tests**: Tests use `t.TempDir()` for filesystem isolation

### Configuration Format (dot-manager.yaml)

```yaml
version: 1
backup_root: "."

paths:
  - name: "config-name"
    files: []  # Empty = entire folder
    backup: "./path/to/backup"
    targets:
      linux: "~/.config/app"
      windows: "~/AppData/Local/app"

packages:
  default_manager: "pacman"
  items:
    - name: "package"
      managers:
        pacman: "package-name"
      tags: ["dev"]
```

### CLI Flags

- `-d, --dir` - Override configuration directory
- `-o, --os` - Override OS detection (linux/windows)
- `-n, --dry-run` - Preview without changes
- `-v, --verbose` - Verbose output
- `-i` - Interactive TUI mode (for restore, backup, install)
- `-t, --tags` - Filter packages by tags
