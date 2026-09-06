# AGENTS.md

This file provides guidance to coding agents working with code in this repository.

## Agent skills

### Issue tracker

Issues and specs are tracked in this repository's GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Triage uses the canonical `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix` labels. See `docs/agents/triage-labels.md`.

### Domain docs

Domain documentation uses a single-context layout. See `docs/agents/domain.md`.

## Build & Test Commands

```bash
# Build
go build ./cmd/tidydots

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

## Documentation

User-facing documentation lives in `docs/` as a MkDocs site (Material for MkDocs). The site is deployed to [tidydots.io](https://tidydots.io) via GitHub Actions on push to main.

**IMPORTANT:** After ANY code change that affects user-facing behavior (new features, changed flags, config format changes, bug fixes that change behavior, CLI output changes, TUI changes), you MUST update the corresponding documentation in `docs/`. This includes:

- **New/changed CLI flags or commands** → update `docs/cli/reference.md`
- **Config format changes** → update `docs/configuration/` pages (applications, configs, packages, templates)
- **New features or changed behavior** → update relevant guide in `docs/guides/`
- **Template system changes** → update `docs/configuration/templates.md`
- **TUI changes** → update `docs/guides/interactive-tui.md`
- **Installation changes** → update `docs/getting-started/installation.md`

Documentation structure:
```
docs/
├── index.md                          # Landing page
├── getting-started/                  # Installation, quick start, concepts
├── configuration/                    # Config reference (overview, apps, configs, packages, templates)
├── guides/                           # Task-oriented guides (multi-machine, packages, git, sudo, TUI)
├── cli/reference.md                  # CLI command reference
└── troubleshooting.md                # Common issues and solutions
```

**Configuration format reminder:** The correct v3 field names are `entries` (not `configs`/`packages`), `package` singular at app level (not `packages`), and `when` (not `filters`). Always use these in documentation examples.

### tidydots-config agent skill

Some environments also carry a `tidydots-config` agent skill (an opencode/Claude
skill that helps author and troubleshoot `tidydots.yaml`). It is not vendored in
this repo; when present it lives at `~/.config/opencode/skills/tidydots-config/`
(`SKILL.md` plus `references/schema.md` and `references/troubleshooting.md`).

**IMPORTANT:** Any change that alters the config schema, adds/renames a config
field, changes package-manager or install-method behavior, changes the
template/`when` system, or changes CLI safety behavior can make this skill stale.
When you make such a change AND the skill is present at that path, update it in
the same session (usually `references/schema.md` or `references/troubleshooting.md`),
keeping it in sync with `docs/`. If the skill is not present, skip this — it is an
optional, environment-local artifact, not a build dependency.

## Testing

See [TESTING.md](TESTING.md) for comprehensive testing documentation including:
- Unit, integration, and snapshot testing
- TUI golden file workflow
- Test patterns and best practices
- Troubleshooting guide

## Architecture

**tidydots** is a cross-platform dotfile management tool written in Go. It manages configuration files through symlinks, clones git repositories, and handles package installation across multiple package managers.

### Core Components

- **cmd/tidydots/main.go** - Cobra CLI entry point defining all commands (init, restore, backup, list, install, list-packages)
- **internal/config/** - Two-level YAML configuration: app config (`~/.config/tidydots/config.yaml`) and repo config (`tidydots.yaml`)
- **internal/config/entry.go** - Entry type for config (symlinks) management
- **internal/config/when.go** - Template-based `when` expression evaluation for conditional inclusion
- **internal/manager/** - Core operations (backup, restore, adopt, list) with platform-aware path selection
- **internal/template/** - Template engine with sprout functions, 3-way merge algorithm
- **internal/state/** - SQLite state store for template render history
- **internal/platform/** - OS/distro detection (Linux/Windows), hostname/user detection
- **internal/tui/** - Bubble Tea-based interactive terminal UI
  - **tuishared/** - Shared constants, styles, keybindings, hints
  - **forms/** - ApplicationForm, SubEntryForm structs; form validation, building, rendering
  - **table/** - PathState type and Row struct for the manage-screen table
  - **operations/** - Operation/ResultItem types and batch operation messages
  - **detection/** - DetectConfigState and package detection functions
  - **components/** - Reusable UI components (list field, text field)
- **internal/packages/** - Multi-package-manager support (pacman, yay, paru, apt, dnf, brew, winget, scoop, choco, git)

### Filesystem and Exec Abstractions

- **internal/fsys/** - `FS` interface abstracting filesystem operations. `OsFS` is the real implementation; `MemFS` is an in-memory fake for testing.
- **internal/cmdexec/** - `Runner` interface abstracting command execution. `OsRunner` is the real implementation; `StubRunner` is a test fake that records calls and returns queued results.

Manager has optional `WithFS(fsys.FS)` and `WithRunner(cmdexec.Runner)` builder methods for dependency injection in tests.

### Key Patterns

- **Unified entries**: Single `entries` array with `sudo: true` flag for entries requiring elevated privileges
- **Entry types**: Config entries (have `backup`) manage symlinks. `SubEntry.Method` selects the deployment mode: `symlink` (default, or empty) or `copy`. A selected `.tmpl` source in a symlink entry renders and deploys through a suffix-free backup alias (target → alias → `.tmpl.rendered`); only listed templates participate, while copy entries copy `.tmpl` files literally. Copy mode performs content-based drift detection (re-copies only when the repo file's contents differ from the target, no-op when in sync), replaces any pre-existing symlink at the target (safe migration from symlink mode), and requires a non-empty `files:` list (see `internal/config/entry.go`, `internal/manager/template_files.go`, `internal/manager/copy.go`)
- **When-based selection**: Applications conditionally included via Go template `when` expressions (e.g., `{{ eq .OS "linux" }}`)
- **Symlink-based restoration**: Configs are symlinked from the dotfiles repo rather than copied by default; set `method: copy` on an entry to deploy a real file copy instead (see [configs.md](docs/configuration/configs.md#deployment-method))
- **Dry-run mode**: All operations support `-n` flag for safe preview
- **Table-driven tests**: Tests use `t.TempDir()` for filesystem isolation
- **File size targets**: Production files should be <400 LOC (hard max 800). If a file exceeds 400 LOC, consider whether it has multiple responsibilities that should be split.

### Configuration Format (tidydots.yaml)

**Version 3 (Current)**

```yaml
version: 3

# Application-level settings
applications:
  - name: "nvim"
    description: "Neovim text editor"
    when: '{{ eq .OS "linux" }}'

    entries:
      # Config entry (symlink management)
      - name: "nvim-config"
        files: []  # Empty = entire folder
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"

    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        brew: "neovim"
        git:
          url: "https://github.com/user/plugins.git"
          branch: "main"
          targets:
            linux: "~/.local/share/nvim/site/pack/plugins/start/myplugins"
          sudo: false

  # System-level application with sudo
  - name: "system-config"
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "hosts"
        sudo: true
        backup: "./system/hosts"
        targets:
          linux: "/etc/hosts"
```

**Setup entries:** A sub-entry with `check` and `run` (both OS→command maps) is a *setup
entry* rather than a config entry — it runs a command instead of deploying files. On
restore, `check` runs; if it exits non-zero, `run` executes and `check` runs again to
confirm the effect landed. Stateless and self-healing: there is no database, so a step
undone by hand is repaired on the next restore. Check commands must be read-only and
fast — they run on every restore, during `--dry-run`, and on every TUI refresh. See
`docs/configuration/setup.md`.

### Git as a Package Manager

Git repositories can be installed as packages by adding git to the managers map:

```yaml
applications:
  - name: "dotfiles"
    package:
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
- `--force-render` - Force re-render of templates, skipping 3-way merge (restore only)

### Template System (`internal/template/`, `internal/state/`)

tidydots supports Go `text/template` processing for both file contents and config paths, with platform-aware context data.

**Template Context Variables**
Templates have access to a `TemplateContext` struct:
- `.OS` - Operating system (`"linux"` or `"windows"`)
- `.Distro` - Linux distribution ID (e.g., `"arch"`, `"ubuntu"`)
- `.Hostname` - Machine hostname
- `.User` - Current username
- `.HasDisplay` - Whether a display server is available (X11/Wayland/Windows)
- `.IsWSL` - Whether running inside Windows Subsystem for Linux (WSL1/WSL2)
- `.Env` - Map of environment variables (e.g., `{{ index .Env "HOME" }}`)

**Template Functions**: All [sprout](https://github.com/go-sprout/sprout) functions are available (string manipulation, math, collections, etc.)

**File Naming Convention**
- `.tmpl` suffix identifies template files (e.g., `.zshrc.tmpl`)
- `.tmpl.rendered` - Rendered output (generated, gitignored)
- `.tmpl.conflict` - Conflict markers from merge (generated, gitignored)

**How it Works**
1. During restore, `.tmpl` files in backup directories are rendered using the template engine
2. Output is written as a sibling `.tmpl.rendered` file in the backup directory
3. A symlink is created from the target (with `.tmpl` stripped) to the `.tmpl.rendered` file
4. Non-template files in the same directory get normal symlinks

**3-Way Merge with SQLite State**
- Pure render output is stored in `.tidydots.db` (SQLite, in backup root)
- On re-render, a 3-way merge preserves user edits to the rendered file:
  - `base` = previous pure render from DB
  - `theirs` = current `.tmpl.rendered` on disk (may have user edits)
  - `ours` = newly rendered template output
- Conflicts generate `<<<<<<< user-edits` / `=======` / `>>>>>>> template` markers
- `--force-render` flag bypasses merge and always overwrites

**Gitignore Patterns** (recommended in dotfiles repo):
```
*.tmpl.rendered
*.tmpl.conflict
.tidydots.db
```

**Path Templating**
Config paths (targets, backup) also support template expressions:
```yaml
targets:
  linux: "~/.config/{{ .Hostname }}/nvim"
```
Paths without `{{` delimiters fall through to standard `ExpandPath` (backward compatible).

**Key Files**
- `internal/template/context.go` - TemplateContext struct and platform factory
- `internal/template/engine.go` - Template engine with sprout functions
- `internal/template/merge.go` - 3-way merge algorithm
- `internal/state/store.go` - SQLite state store for render history
- `internal/manager/template_restore.go` - Template-specific restore logic

### TUI Patterns (internal/tui/)

The TUI uses Bubble Tea with consistent interaction patterns across all components:

**Two-Phase Editing for Text Fields**
All text inputs use a two-phase approach:
1. **Navigation mode**: Field is focused but not editable. Use `↑/k` and `↓/j` to navigate between fields.
2. **Edit mode**: Press `enter` or `e` to enter edit mode. The text input becomes active and accepts typing. Press `enter` to save or `esc` to cancel.

This applies to: name, description, targets, backup path, repo URL, branch, file list items, and when expressions.

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

**List Field Pattern** (files)
List fields have their own cursor (`filesCursor`) separate from `focusIndex`:
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
