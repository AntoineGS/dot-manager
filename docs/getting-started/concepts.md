# Concepts

This page explains the core ideas behind tidydots. Understanding these concepts will help you get the most out of the tool.

## What are dotfiles?

On Unix-like systems, many programs store their configuration in files and directories that begin with a dot (`.`), such as `.bashrc`, `.gitconfig`, or `.config/nvim`. These are commonly called **dotfiles**.

Over time, you invest significant effort customizing your tools. Dotfile management helps you:

- **Back up** your configuration so you never lose it
- **Version control** changes with Git so you can track and revert
- **Share** the same setup across multiple machines
- **Rebuild** a fresh system quickly after a reinstall

## Symlinks vs. copying

There are two common approaches to managing dotfiles:

**Copying** files between a repository and their target locations works, but it creates two separate copies. Edits to one copy do not appear in the other, so you constantly need to remember to sync changes.

**Symlinking** is the approach tidydots uses. Instead of copying, tidydots creates symbolic links (symlinks) from the target location to the files in your dotfiles repository. When a program reads `~/.config/nvim`, the operating system transparently follows the symlink to the actual files inside your repo.

This means:

- There is only **one copy** of each file -- the one in your repo
- Any edits you make (whether through the program or directly) are immediately reflected in the repo
- Running `git diff` in your repo shows exactly what changed
- No manual syncing is ever needed

```
~/.config/nvim  --->  ~/dotfiles/nvim/
    (symlink)            (real files, tracked by git)
```

## Key terminology

### Backup directory

The **backup directory** (or backup path) is the location inside your dotfiles repository where the actual configuration files are stored. It is the source of truth.

In your `tidydots.yaml`, each config entry specifies a `backup` path relative to the repository root:

```yaml
entries:
  - name: "nvim-config"
    backup: "./nvim"        # Files live at ~/dotfiles/nvim/
    targets:
      linux: "~/.config/nvim"
```

### Target

The **target** is the path where the configuration is expected on the system. This is where programs look for their config files -- for example, `~/.config/nvim` for Neovim or `~/.bashrc` for Bash.

tidydots creates a symlink at the target that points back to the backup directory. Targets are specified per operating system, so you can handle Linux and Windows paths in the same configuration:

```yaml
targets:
  linux: "~/.config/nvim"
  windows: "~/AppData/Local/nvim"
```

### Adopt

**Adopting** is what happens when a target file or directory already exists on the system, but the corresponding backup location in your repo is empty.

When you run `tidydots restore` in this situation, tidydots:

1. **Moves** the existing target into the backup location in your repo
2. **Creates a symlink** at the target pointing to the new backup location

This is particularly useful when you are setting up tidydots on a machine that already has configuration files in place. You do not need to manually copy files into your repo first -- tidydots handles the migration for you.

!!! tip
    Adopting only happens when the backup does not exist. If both the backup and target exist, tidydots performs a merge instead, combining the contents and handling conflicts.

### Applications

An **application** is a logical grouping in your `tidydots.yaml`. Each application has a name, an optional description, and contains one or more entries (config items). An application can also have an associated package for installation.

```yaml
applications:
  - name: "neovim"
    description: "Neovim text editor"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
```

Applications help you organize related configuration and packages together. For example, a "neovim" application might contain the Neovim config entry and the neovim package, while a "zsh" application groups the shell config with the zsh package.

### Entries

**Entries** are the individual config items within an application. An entry becomes a **config entry** when it has a `backup` field -- this is what tells tidydots it manages files via symlinks.

Each entry specifies:

- **`name`** -- A descriptive identifier
- **`backup`** -- Path to the source files in your repo (what makes it a config entry)
- **`targets`** -- OS-specific paths where the symlink should be created
- **`files`** (optional) -- Specific files to symlink individually, rather than the entire folder

When `files` is empty or omitted, the entire backup folder is symlinked as a unit. When `files` is specified, only those individual files are symlinked, allowing the target directory to contain other non-managed files:

```yaml
entries:
  # Symlinks the entire folder
  - name: "nvim-config"
    backup: "./nvim"
    targets:
      linux: "~/.config/nvim"

  # Symlinks only specific files
  - name: "zshrc"
    backup: "./zsh"
    files: [".zshrc", ".zshenv"]
    targets:
      linux: "~"
```

### When expressions

**When expressions** let you conditionally include or exclude applications based on the current machine's properties. They use Go template syntax and are evaluated at runtime.

```yaml
applications:
  - name: "zsh"
    when: '{{ eq .OS "linux" }}'
    # ...

  - name: "arch-packages"
    when: '{{ eq .Distro "arch" }}'
    # ...

  - name: "work-tools"
    when: '{{ eq .Hostname "work-laptop" }}'
    # ...
```

Available template variables:

| Variable    | Description                                   | Example values              |
|-------------|-----------------------------------------------|-----------------------------|
| `.OS`       | Operating system                              | `"linux"`, `"windows"`      |
| `.Distro`   | Linux distribution ID                         | `"arch"`, `"ubuntu"`, `"fedora"` |
| `.Hostname` | Machine hostname                              | `"work-laptop"`, `"server"` |
| `.User`     | Current username                              | `"alice"`, `"root"`         |
| `.Env`      | Map of environment variables                  | `{{ index .Env "HOME" }}`   |

If no `when` field is specified, the application is always included. If the expression evaluates to anything other than `"true"`, the application is skipped.

!!! note
    When expressions use Go's `text/template` syntax. You can combine conditions with `and`, `or`, and `not` -- for example: `{{ and (eq .OS "linux") (eq .Distro "arch") }}`.

## The workflow

The typical tidydots workflow has three steps:

### 1. Initialize

Run `tidydots init <path>` once per machine to tell tidydots where your dotfiles repository lives. This creates an app config at `~/.config/tidydots/config.yaml`.

### 2. Configure

Edit `tidydots.yaml` in your dotfiles repo to describe the applications, config entries, and packages you want to manage. You can do this by hand or use the interactive TUI (run `tidydots` with no arguments).

### 3. Restore

Run `tidydots restore` to create symlinks from target locations to your backup files. Use `tidydots restore -n` first to preview what will happen.

For packages, run `tidydots install` to install all configured packages using the appropriate package manager for your system.

```
            tidydots.yaml               tidydots restore
  [Define] ----------------> [Review] ------------------> [Symlinks created]
  your config                 dry-run                      configs in place
                              tidydots restore -n
```

This cycle repeats whenever you add a new application, change a target path, or set up a new machine. The restore operation is idempotent -- running it multiple times is safe, and it will skip entries that are already correctly symlinked.

## Next steps

- Follow the [Quick Start](quick-start.md) to put these concepts into practice
- Read the [Configuration overview](../configuration/overview.md) for the full reference on `tidydots.yaml`
- Learn about [templates](../configuration/templates.md) for dynamic file content
- Explore [multi-machine setups](../guides/multi-machine-setups.md) to use `when` expressions across different hosts
