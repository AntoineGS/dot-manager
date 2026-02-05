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

# Run snapshot tests
go test ./internal/tui -run TestScreenResults_Snapshots

# Update golden files (after intentional UI changes)
go test ./internal/tui -run TestScreenResults_Snapshots -update

# Run all tests including snapshots
go test ./...
```

**IMPORTANT:** After implementing any feature change, bug fix, or refactoring, ALWAYS run `golangci-lint run` to ensure code quality and catch potential issues before committing.

## Testing

See [TESTING.md](TESTING.md) for comprehensive testing documentation including:
- Unit, integration, and snapshot testing
- TUI golden file workflow
- Test patterns and best practices
- Troubleshooting guide

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

### File Picker Feature (internal/tui/)

The SubEntryForm includes an interactive file picker for adding files to config entries:

**Three-Mode Workflow**
When adding files to a config entry (Files mode), users progress through three modes:

1. **ModeChoosing**: Choose how to add file
   - `Browse Files` - Launch interactive file picker
   - `Type Path` - Enter path manually (legacy text input)
   - Navigate with `↑/↓` or `k/j`, select with `enter`, cancel with `esc`

2. **ModePicker**: Interactive file browser (Browse mode)
   - Navigate filesystem with arrow keys or vim keys (`↑/k`, `↓/j`)
   - Toggle file selection with `space` or `tab` (multi-select supported)
   - Selected files shown with lighter purple background (`SelectedRowStyle`)
   - Current cursor position shown with darker purple (`SelectedMenuItemStyle`)
   - Confirm selections with `enter`, cancel with `esc`

3. **ModeTextInput**: Manual text entry (Type mode)
   - Type relative file path directly
   - Confirm with `enter`, cancel with `esc`

**Path Resolution**
- File picker starts in target directory (or nearest existing parent)
- Selected files are stored as absolute paths internally (`selectedFiles` map)
- On confirmation, absolute paths are converted to relative paths (relative to target)
- Only files within target directory hierarchy are accepted
- Uses `path_utils.go` functions: `expandTargetPath`, `resolvePickerStartDirectory`, `convertToRelativePaths`

**Visual Feedback**
- Selection count shown at bottom: "N file(s) selected"
- Success message after adding files: "Added N file(s)"
- Error messages for invalid paths or files outside target
- Context-sensitive help text updates based on current mode

**Implementation Notes**
- `selectedFiles` map tracks absolute paths during picker session
- `addFileMode` controls current workflow state (ModeNone/ModeChoosing/ModePicker/ModeTextInput)
- `modeMenuCursor` tracks position in mode selection menu
- File picker initialized lazily when entering ModePicker
- Selections cleared after confirmation or cancellation

### Multi-Selection Feature (internal/tui/)

The TUI supports batch operations through multi-selection mode:

**Selection Keybindings**
- `tab`, `space` - Toggle selection of current item (app or sub-entry)
- Selecting an app recursively selects all its visible sub-entries (configs/packages)
- Deselecting an app deselects all its sub-entries
- Sub-entries can be individually toggled even when parent app is not selected

**Visual Indicators**
- **Banner**: Shows at top when selections exist: "N app(s), M item(s) selected"
- **Row Styling**: Selected rows highlighted with `SelectedRowStyle` (lighter purple background)
- **Persistent**: Selections persist across search filtering, screen navigation, and edit operations

**Batch Operations**
All batch operations follow a three-screen flow:
1. **Manage Screen** - Select items, press operation key
2. **Summary Screen** - Review what will be changed, confirm or cancel
3. **Progress Screen** - Watch real-time progress with progress bar

Available operations in multi-select mode:
- `r` - Batch restore: Create symlinks for all selected configs
- `i` - Batch install: Install packages for all selected apps
- `d` - Batch delete: Remove configs and packages for all selected items

**Exit Behavior (Esc Priority)**
The `esc` key follows a priority system:
1. If in search mode → Exit search (keep selections)
2. If selections exist → Clear all selections
3. Otherwise → Return to previous screen

**Implementation Notes**
- Selection state tracked in `Model.selectedApps` (map[int]bool) and `Model.selectedSubEntries` (map[string]bool)
- `multiSelectActive` flag enables batch operation mode
- Summary screen shows operation details before execution
- Progress screen shows real-time feedback with progress bar
- All operations support dry-run mode via global `-n` flag
