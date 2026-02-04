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

# Run golangci-lint (REQUIRED after each feature change)
golangci-lint run
```

**IMPORTANT:** After implementing any feature change, bug fix, or refactoring, ALWAYS run `golangci-lint run` to ensure code quality and catch potential issues before committing.

## Architecture

**dot-manager** is a cross-platform dotfile management tool written in Go. It manages configuration files through symlinks, clones git repositories, and handles package installation across multiple package managers.

### Core Components

- **cmd/dot-manager/main.go** - Cobra CLI entry point defining all commands (init, restore, backup, list, install, list-packages)
- **internal/config/** - Two-level YAML configuration: app config (`~/.config/dot-manager/config.yaml`) and repo config (`dot-manager.yaml`)
- **internal/config/entry.go** - Entry type for config (symlinks) management
- **internal/config/filter.go** - Filter system with include/exclude conditions for os, distro, hostname, user
- **internal/manager/** - Core operations (backup, restore, adopt, list) with platform-aware path selection
- **internal/platform/** - OS/distro detection (Linux/Windows), hostname/user detection
- **internal/tui/** - Bubble Tea-based interactive terminal UI with Lipgloss styling
- **internal/packages/** - Multi-package-manager support (pacman, yay, paru, apt, dnf, brew, winget, scoop, choco, git)

### Key Patterns

- **Unified entries**: Single `entries` array with `sudo: true` flag for entries requiring elevated privileges
- **Entry types**: Config entries (have `backup`) manage symlinks
- **Filter-based selection**: Entries filtered by os, distro, hostname, user with regex support
- **Symlink-based restoration**: Configs are symlinked from the dotfiles repo rather than copied
- **Dry-run mode**: All operations support `-n` flag for safe preview
- **Table-driven tests**: Tests use `t.TempDir()` for filesystem isolation

### Configuration Format (dot-manager.yaml)

**Version 3 (Current)**

```yaml
version: 3

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
          git:
            url: "https://github.com/user/plugins.git"
            branch: "main"
            targets:
              linux: "~/.local/share/nvim/site/pack/plugins/start/myplugins"
            sudo: false

    filters:
      - include:
          os: "linux"

  # System-level application with sudo
  - name: "system-config"
    sudo: true
    configs:
      - name: "hosts"
        backup: "./system/hosts"
        targets:
          linux: "/etc/hosts"
    filters:
      - include:
          distro: "arch"
```

### Git as a Package Manager

Git repositories can be installed as packages by adding git to the managers map:

```yaml
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
```

**Fields:**
- `url`: Repository URL (required)
- `branch`: Branch to clone (optional, defaults to default branch)
- `targets`: OS-specific clone destinations (required)
- `sudo`: Run git commands with sudo (optional, default false)

**Behavior:**
- If target directory exists with `.git/`: runs `git pull` to update
- If target doesn't exist: clones repository
- Git configuration is nested under `managers.git` for consistency with other package managers

### CLI Flags

- `-d, --dir` - Override configuration directory
- `-o, --os` - Override OS detection (linux/windows)
- `-n, --dry-run` - Preview without changes
- `-v, --verbose` - Verbose output
- `-i` - Interactive TUI mode (for restore, backup, install)

### TUI Patterns (internal/tui/)

The TUI uses Bubble Tea with consistent interaction patterns across all components:

**Two-Phase Editing for Text Fields**
All text inputs use a two-phase approach:
1. **Navigation mode**: Field is focused but not editable. Use `↑/k` and `↓/j` to navigate between fields.
2. **Edit mode**: Press `enter` or `e` to enter edit mode. The text input becomes active and accepts typing. Press `enter` to save or `esc` to cancel.

This applies to: name, description, targets, backup path, repo URL, branch, file list items, and filter values.

**Consistent Keybindings**
- `↑/k`, `↓/j` - Navigate between fields/items (vim-style)
- `←/h`, `→/l` - Navigate horizontally (for selections like include/exclude)
- `enter`, `e` - Enter edit mode or activate item
- `esc` - Cancel/go back (exits edit mode first, then exits screen)
- `tab`, `shift+tab` - Navigate forward/backward through fields
- `space` - Toggle boolean fields
- `d`, `delete`, `backspace` - Delete list items
- `s`, `ctrl+s` - Save form

**Visual States** (see `styles.go`)
- **Unfocused**: Plain text or `MutedTextStyle` for placeholders
- **Focused (not editing)**: `SelectedMenuItemStyle` highlight
- **Editing**: Show the `textinput.Model.View()` with cursor

**List Field Pattern** (files, filters)
List fields have their own cursor (`filesCursor`, `filtersCursor`) separate from `focusIndex`:
- When focused on a list field, `↑/k` and `↓/j` navigate within the list
- At list boundaries, navigation moves to adjacent form fields
- `enter`/`e` on an item enters edit mode for that item
- `enter`/`e` on "Add" button starts adding a new item

**Help Text**
Use `RenderHelp()` to show context-sensitive help. Update help based on current state (editing vs navigating). Include vim-style keys alongside arrows (e.g., `"↑/k ↓/j", "navigate"`).
