# CLI Reference

Complete reference for all tidydots commands, flags, and usage patterns.

## Global flags

These flags are available on every command.

| Flag | Short | Description |
|------|-------|-------------|
| `--dir <path>` | `-d` | Override the repository directory, including its TUI hostname choices |
| `--os <os>` | `-o` | Override OS detection (`linux` or `windows`) |
| `--dry-run` | `-n` | Show what would be done without making changes |
| `--verbose` | `-v` | Enable verbose output |

!!! tip
    Combine `-n` and `-v` for the most detailed preview of any operation:

    ```bash
    tidydots restore -n -v
    ```

---

## tidydots

Run tidydots with no subcommand to launch the interactive TUI.

```
tidydots [flags]
```

The TUI provides a visual interface for browsing applications, restoring configs, installing packages, and editing your `tidydots.yaml`. It requires a terminal -- if standard input is not a TTY, tidydots prints an error and exits.

### Flags

| Flag | Description |
|------|-------------|
| `--actions` | Start the TUI with the action filter enabled |

!!! note
    The TUI reads your configuration on startup. Make sure you have run `tidydots init` first, or pass `--dir` to point at your dotfiles repo.

---

## tidydots init

Initialize the app configuration by setting the path to your dotfiles repository.

```
tidydots init <path> [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | Path to your dotfiles repository |

### Behavior

1. Resolves the path to an absolute directory (expands `~`).
2. Verifies the directory exists.
3. Writes the path to `~/.config/tidydots/config.yaml`.
4. Warns if `tidydots.yaml` is not found inside the directory.

You only need to run this once per machine. After initialization, all other commands will read the saved path automatically.

### Examples

```bash
# Initialize with an absolute path
tidydots init ~/dotfiles

# Initialize with a relative path
tidydots init ./my-configs

# Output
App configuration saved to /home/youruser/.config/tidydots/config.yaml
Configurations directory: /home/youruser/dotfiles
```

---

## tidydots restore

Restore configurations by deploying files from backup sources in your dotfiles repo.

```
tidydots restore [app [entry]] [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `app` | No | Exact application name to restore. If omitted, all matching applications are restored. |
| `entry` | No | Exact config or setup entry name within `app`. Requires `app`. |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--interactive` | `-i` | Run in interactive TUI mode |
| `--no-merge` | | Disable merge mode; return an error if the target already exists |
| `--force` | | When combined with `--no-merge`, delete existing files instead of erroring |
| `--force-render` | | Force re-render of templates, skipping the 3-way merge |

### Behavior

Targeting an application restores all included config and setup entries in that
application. Targeting an entry restores only that entry. Applications and entries
are included only when their respective `when` expressions match; for an entry,
both its parent application and entry conditions must match. A false condition or
template error skips the entry before tidydots resolves its targets or backup, runs
a setup `check`/`run`, or lists it. Explicitly targeting an excluded application or
entry returns a conditions mismatch error. Targets cannot be combined with
`--interactive`.

For each config entry that matches the current OS and `when` conditions:

1. In symlink mode, if the target does not exist and the backup does, a symlink is created.
2. In symlink mode, if the target exists but the backup does not, the target is **adopted** -- moved into the backup location and then symlinked back. Copy mode never adopts an existing target; back up missing sources first.
3. In symlink mode, template files (`.tmpl` suffix) are rendered through the template engine. Rendered output is written to `.tmpl.rendered` and symlinked to the target path with the `.tmpl` suffix stripped. In `method: copy` entries, selected templates are rendered directly to real suffix-free target files and use the target as the current merge input.
4. On re-render, a 3-way merge preserves any manual edits made to the rendered file or copy-mode target. Ordinary copy files remain literal and overwrite target drift.

!!! warning
    The `--force` flag deletes existing target files. Always preview with `-n` first to verify what will be removed.

### Examples

```bash
# Preview what would happen
tidydots restore -n

# Restore all configs
tidydots restore

# Restore every entry for one application
tidydots restore nvim

# Restore one entry within an application
tidydots restore nvim config

# Restore in interactive mode
tidydots restore -i

# Restore with strict mode (error if targets already exist)
tidydots restore --no-merge

# Restore with strict mode, replacing existing files
tidydots restore --no-merge --force

# Force re-render all templates (discard manual edits to rendered files)
tidydots restore --force-render

# Restore with OS override
tidydots restore -o windows
```

---

## tidydots backup

Copy configuration files from target locations back into the backup directory in your dotfiles repo.

```
tidydots backup [app [entry]] [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `app` | No | Exact application name to back up. If omitted, all matching applications are backed up. |
| `entry` | No | Exact config entry name within `app`. Requires `app`; setup entries cannot be backed up. |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--interactive` | `-i` | Run in interactive TUI mode |

### Behavior

For each config entry that matches the current OS and `when` conditions, copies the files from the target location into the backup path. This is the inverse of `restore` -- it captures the current state of your live configs into the repo. For file-list entries, selected `.tmpl` sources are skipped in both symlink and copy modes so a generated target cannot overwrite its template source; ordinary non-template copy files retain literal backup behavior.

Targeting an application backs up all included config entries in that application and skips setup entries. Directly targeting a setup entry returns an error. Unknown applications and entries return errors, as do applications or entries excluded by current `when` conditions. Targets cannot be combined with `--interactive`.

### Examples

```bash
# Preview what would be backed up
tidydots backup -n

# Backup all configs
tidydots backup

# Back up every config entry for one application
tidydots backup nvim

# Back up one config entry
tidydots backup nvim config

# Backup in interactive mode
tidydots backup -i
```

---

## tidydots list

Display all configured paths and their symlink targets for the current OS.

```
tidydots list [app [entry]] [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `app` | No | Exact application name to list. If omitted, all matching applications are listed. |
| `entry` | No | Exact config entry name within `app`. Requires `app`; setup entries cannot be listed. |

### Behavior

Lists every config entry that matches the current OS and `when` conditions, showing the backup path and the target path. This is useful for verifying your configuration, checking for broken symlinks, and reviewing copy-mode destinations.

Targeting an application lists its included config entries, skips its setup entries, and retains the application's package summary. Targeting an entry lists only that config entry and suppresses the package summary. Directly targeting a setup entry returns an error. Unknown applications and entries return errors, as do applications or entries excluded by current `when` conditions.

### Examples

```bash
# List all configured paths
tidydots list

# List one application
tidydots list nvim

# List one config entry
tidydots list nvim config

# List paths for a different OS
tidydots list -o windows

# List paths from a specific directory
tidydots list -d ~/dotfiles
```

---

## tidydots status

Resolve and report the current package and configuration-entry status without changing the system.

```
tidydots status [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--actions` | Show only applications and entries that need action |
| `--json` | Output stable structured JSON |

Status waits for every package, config, template, and setup check used by the TUI before deciding whether an item is actionable. For selected templates in `method: copy` entries, it compares the source hash and render history with the live suffix-free target; target edits are `Modified`, source or history changes are `Outdated`, and missing targets are `Ready`. Status reads are native-only and never request `sudo` interactively, even when the entry has `sudo: true`; an unreadable source or target is reported as actionable `Unavailable`. An actionable result is still a successful command and exits `0`; configuration or status-computation failures exit nonzero. The command honors the global `--dir`, `--os`, and `--verbose` flags.

With `--actions`, the `applications` array is reduced using the same predicate as the TUI `x` filter. `counts` always includes totals and actionable counts for all applications and entries that apply to the selected platform.

### JSON schema

```json
{
  "actionable": true,
  "actions_only": true,
  "counts": {
    "applications": 1,
    "entries": 1,
    "packages": 0,
    "actionable_applications": 1,
    "actionable_entries": 1,
    "actionable_packages": 0
  },
  "applications": [
    {
      "name": "nvim",
      "description": "Neovim editor",
      "status": "Unknown",
      "actionable": true,
      "package": null,
      "entries": [
        {
          "name": "config",
          "kind": "config",
          "state": "Ready",
          "actionable": true,
          "target": "~/.config/nvim",
          "backup": "./nvim",
          "method": "symlink"
        }
      ]
    }
  ]
}
```

`state` uses the same labels as the TUI. Actionable entry states are `Missing`, `Ready`, `Adopt`, `Needs setup`, `Outdated`, `Modified`, and `Unavailable`; an uninstalled package is actionable when its installation method is available. `Unavailable` means status could not safely read or validate the selected source or live target, rather than meaning the target is absent. `package` is an object when the application defines a package and contains its selected `name`, `method`, nullable `installed` value, and `actionable` flag.

### Examples

```bash
# Show all resolved statuses
tidydots status

# Get actionable items for a status bar or integration
tidydots status --actions --json

# Resolve status for another repository and OS
tidydots status --dir ~/dotfiles --os windows --json
```

---

## tidydots install

Install packages using the configured package managers.

```
tidydots install [package-names...] [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `package-names` | No | Specific package names to install. If omitted, all matching packages are installed. |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--interactive` | `-i` | Run in interactive TUI mode |

### Behavior

1. Loads the configuration and filters packages by OS and their application's `when` condition. Packages remain application-level and do not have entry-level conditions.
2. Detects available package managers on the system.
3. Selects the best manager for each package based on `default_manager` and `manager_priority` settings.
4. Installs each package, reporting success or failure.

If specific package names are provided as arguments, only those packages are installed. Otherwise, all matching packages are installed.

### Examples

```bash
# Preview all package installations
tidydots install -n

# Install all packages
tidydots install

# Install specific packages
tidydots install neovim zsh

# Install in interactive mode
tidydots install -i

# Install with verbose output
tidydots install -v
```

---

## tidydots list-packages

Display all configured packages with their availability and installation method.

```
tidydots list-packages [flags]
```

### Behavior

Lists every package that matches the current OS and `when` conditions. For each package, shows:

- An availability indicator (`✓` if installable, `✗` if not)
- The package name
- The installation method (which package manager will be used, or `unavailable`)
- The package description, if configured

### Examples

```bash
# List all packages
tidydots list-packages

# Check package availability for a different OS
tidydots list-packages -o windows
```

Sample output:

```
Available package managers: [pacman yay]

✓ neovim (pacman)
    Neovim text editor
✓ zsh (pacman)
✓ nvim-plugins (git)
✗ powershell (unavailable)
```

---

## tidydots preview

Watch template files for changes and render them in real time.

```
tidydots preview <path>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `path` | Yes | A single `.tmpl` file or a directory containing `.tmpl` files |

### Behavior

1. Discovers all `.tmpl` files at the given path (recursively if a directory).
2. Performs an initial render of every discovered template.
3. Watches for file saves using filesystem notifications.
4. On each save, re-renders the changed template and writes the output to the sibling `.tmpl.rendered` file.
5. On a syntax error, prints the error and keeps the last good `.tmpl.rendered` intact.
6. Runs until interrupted with `Ctrl+C`.

The command uses the current platform context (OS, Hostname, User, etc.) for rendering, and inherits the global `--dir`, `--os`, and `--verbose` flags.

### Examples

```bash
# Preview a single template file
tidydots preview ./alacritty/alacritty.toml.tmpl

# Preview all templates in a directory
tidydots preview ./alacritty

# Preview with OS override
tidydots preview ./zsh -o windows

# Preview with verbose logging
tidydots preview ./alacritty -v
```

Sample terminal output:

```
Watching 3 template(s)...
  config.zshrc.tmpl
  config.gitconfig.tmpl
  config.alacritty.tmpl

✓ config.zshrc.tmpl rendered (14:32:05)
✗ config.zshrc.tmpl error: template: config.zshrc.tmpl:12: unexpected "}" (14:32:08)
✓ config.zshrc.tmpl rendered (14:32:11)
```

!!! tip
    Open the `.tmpl` source and its `.tmpl.rendered` output side by side in your editor for a live preview workflow. Every time you save the template, the rendered file updates automatically.

### NDJSON Protocol

When watching a single template, `tidydots preview` communicates with editor plugins via NDJSON (newline-delimited JSON) on stdin/stdout.

**Stdout** (CLI → editor): After each render, the CLI emits a source map response:

```json
{"source_map":{"1":"1","2":"3"},"reverse_map":{"1":"1","3":"2"},"line_types":{"1":"text","2":"directive","3":"expression"},"file":"config.toml.tmpl"}
```

| Field | Description |
|-------|-------------|
| `source_map` | Template line → rendered line (forward mapping) |
| `reverse_map` | Rendered line → template line (reverse mapping) |
| `line_types` | Template line → type (`text`, `expression`, or `directive`) |
| `file` | Path of the template file |

**Stdin** (editor → CLI): The editor can send content updates or structural edits:

```json
{"content":"updated template content here"}
{"rendered_edit":{"inserts":[{"after_rendered_line":3,"text":"new line"}],"deletes":[5]}}
```

**Stdout** (CLI → editor): After a structural edit, the CLI responds with a template update:

```json
{"template_update":{"content":"updated template source","cursor_line":4}}
```

---

## tidydots completion

Generate shell autocompletion scripts for tidydots.

```
tidydots completion <shell> [flags]
```

### Supported shells

| Shell | Command |
|-------|---------|
| bash | `tidydots completion bash` |
| zsh | `tidydots completion zsh` |
| fish | `tidydots completion fish` |
| powershell | `tidydots completion powershell` |

### Examples

```bash
# Add to your ~/.bashrc
source <(tidydots completion bash)

# Add to your ~/.zshrc
source <(tidydots completion zsh)

# Add to your fish config
tidydots completion fish | source

# Add to your PowerShell profile
tidydots completion powershell | Out-String | Invoke-Expression
```

!!! tip
    Run `tidydots completion <shell> --help` for detailed instructions on setting up autocompletion for your specific shell.

---

## Examples

### First-time setup on a new machine

```bash
# 1. Clone your dotfiles repo
git clone https://github.com/youruser/dotfiles.git ~/dotfiles

# 2. Initialize tidydots
tidydots init ~/dotfiles

# 3. Preview what will happen
tidydots restore -n

# 4. Restore all configs
tidydots restore

# 5. Install all packages
tidydots install
```

### Day-to-day usage

```bash
# Launch the interactive TUI to browse and manage everything
tidydots

# After editing configs on disk, back them up into the repo
tidydots backup

# Check what is currently configured
tidydots list
tidydots list-packages
```

### Working with templates

```bash
# Live preview a template while editing
tidydots preview ./alacritty/alacritty.toml.tmpl

# Live preview all templates in a directory
tidydots preview ./alacritty

# Preview template rendering (one-shot, no watch)
tidydots restore -n -v

# Force re-render templates after changing a .tmpl file
tidydots restore --force-render

# Re-render with dry-run to verify
tidydots restore --force-render -n
```

### Using overrides

```bash
# Point at a different dotfiles directory (without changing app config)
tidydots restore -d ~/work-dotfiles

# Preview what would happen on Windows from a Linux machine
tidydots list -o windows

# Combine directory override with dry-run
tidydots install -d ~/dotfiles -n -v
```
