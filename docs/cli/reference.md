# CLI Reference

Complete reference for all tidydots commands, flags, and usage patterns.

## Global flags

These flags are available on every command.

| Flag | Short | Description |
|------|-------|-------------|
| `--dir <path>` | `-d` | Override the configurations directory (ignores app config) |
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

Restore configurations by creating symlinks from target locations to backup sources in your dotfiles repo.

```
tidydots restore [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--interactive` | `-i` | Run in interactive TUI mode |
| `--no-merge` | | Disable merge mode; return an error if the target already exists |
| `--force` | | When combined with `--no-merge`, delete existing files instead of erroring |
| `--force-render` | | Force re-render of templates, skipping the 3-way merge |

### Behavior

For each config entry that matches the current OS and `when` conditions:

1. If the target does not exist and the backup does, a symlink is created.
2. If the target exists but the backup does not, the target is **adopted** -- moved into the backup location and then symlinked back.
3. Template files (`.tmpl` suffix) are rendered through the template engine. Rendered output is written to `.tmpl.rendered` and symlinked to the target path with the `.tmpl` suffix stripped.
4. On re-render, a 3-way merge preserves any manual edits made to the rendered file.

!!! warning
    The `--force` flag deletes existing target files. Always preview with `-n` first to verify what will be removed.

### Examples

```bash
# Preview what would happen
tidydots restore -n

# Restore all configs
tidydots restore

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
tidydots backup [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--interactive` | `-i` | Run in interactive TUI mode |

### Behavior

For each config entry that matches the current OS and `when` conditions, copies the files from the target location into the backup path. This is the inverse of `restore` -- it captures the current state of your live configs into the repo.

### Examples

```bash
# Preview what would be backed up
tidydots backup -n

# Backup all configs
tidydots backup

# Backup in interactive mode
tidydots backup -i
```

---

## tidydots list

Display all configured paths and their symlink targets for the current OS.

```
tidydots list [flags]
```

### Behavior

Lists every config entry that matches the current OS and `when` conditions, showing the backup path and the target path. This is useful for verifying your configuration and checking for broken symlinks.

### Examples

```bash
# List all configured paths
tidydots list

# List paths for a different OS
tidydots list -o windows

# List paths from a specific directory
tidydots list -d ~/dotfiles
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

1. Loads the configuration and filters packages by OS and `when` conditions.
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
# Preview template rendering
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
