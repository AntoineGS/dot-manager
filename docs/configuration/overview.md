# Configuration Overview

tidydots uses a **two-level configuration system**: a minimal app config that points to your dotfiles repository, and a repo config inside that repository that describes everything tidydots manages.

## Two-Level Configuration

### App Config

**Location:** `~/.config/tidydots/config.yaml`

This file is created by `tidydots init` and contains a single field:

```yaml
# tidydots app configuration
# This file only stores the path to your configurations repository

config_dir: ~/dotfiles
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `config_dir` | string | yes | Absolute or `~`-relative path to your dotfiles repository |

!!! note
    The `config_dir` path supports `~` expansion. tidydots verifies that the directory exists when loading the config. If the directory is missing, you will see an error prompting you to run `tidydots init` or create it manually.

### Repo Config

**Location:** `<config_dir>/tidydots.yaml`

This is the main configuration file that lives inside your dotfiles repository. It describes all applications, their config entries, and packages.

```yaml
version: 3

applications:
  - name: "nvim"
    description: "Neovim text editor"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        brew: "neovim"
```

## Root-Level Fields

The `tidydots.yaml` file supports the following root-level fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `version` | integer | no | `3` | Configuration format version. Must be `3` |
| `default_manager` | string | no | - | Preferred package manager when multiple are available |
| `manager_priority` | []string | no | - | Ordered list of package managers to try, highest priority first |
| `applications` | []Application | no | - | Array of application definitions |

### version

```yaml
version: 3
```

The version field is required. tidydots currently only supports version 3. If omitted, it defaults to 3, but explicitly setting it is recommended for clarity.

### default_manager

```yaml
default_manager: "yay"
```

Specifies which package manager to prefer when multiple are available on the system. This is overridden by `manager_priority` if both are set.

### manager_priority

```yaml
manager_priority:
  - paru
  - yay
  - pacman
```

An ordered list of package managers. tidydots tries each in order and uses the first one available on the system. This takes precedence over `default_manager`.

!!! tip
    If neither `default_manager` nor `manager_priority` is set, tidydots auto-selects a package manager based on your OS. See the [Packages](packages.md) reference for auto-selection details.

### applications

```yaml
applications:
  - name: "zsh"
    entries:
      - name: "zshrc"
        backup: "./zsh"
        targets:
          linux: "~/.config/zsh"
```

An array of [Application](applications.md) objects. Each application groups related config entries and an optional package definition under a single name.

## Complete Example

```yaml
version: 3
default_manager: "yay"
manager_priority:
  - paru
  - yay
  - pacman

applications:
  - name: "nvim"
    description: "Neovim text editor"
    when: '{{ or (eq .OS "linux") (eq .OS "windows") }}'
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        brew: "neovim"
        winget: "Neovim.Neovim"

  - name: "zsh"
    description: "Z shell configuration"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "zshrc"
        backup: "./zsh"
        targets:
          linux: "~/.config/zsh"
    package:
      managers:
        pacman: "zsh"
        apt: "zsh"
        brew: "zsh"
```

## Configuration Loading

When you run any tidydots command:

1. tidydots reads `~/.config/tidydots/config.yaml` to find your `config_dir`
2. It loads `<config_dir>/tidydots.yaml` as the repo config
3. Paths containing `~` are expanded to your home directory
4. Paths containing `{{ }}` template expressions are rendered (see [Templates](templates.md))
5. Applications are filtered by their `when` expressions against the current platform

!!! info "CLI Override"
    You can override the config directory with the `-d` / `--dir` flag on any command, bypassing the app config entirely.
