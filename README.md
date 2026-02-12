# tidydots

A cross-platform dotfile management tool written in Go. Manage configuration
files through symlinks, render templates for machine-specific configs, install
packages, and clone git repositories --- all from a single YAML file.

> For comprehensive documentation, visit **[tidydots.io](https://tidydots.io)**.

<img width="825" height="489" alt="image" src="https://github.com/user-attachments/assets/1de6041f-ca59-4138-871f-2ef0a48a1630" />

## Features

- **Symlink-based config management** --- edits sync instantly, no copying
- **Cross-platform** --- Linux and Windows with OS-specific target paths
- **Template rendering** --- Go templates for machine-specific configuration
- **Multi-package-manager support** --- pacman, yay, paru, apt, dnf, brew,
  winget, scoop, choco
- **Interactive TUI** --- Bubble Tea terminal interface for visual management
- **Git repository management** --- clone and update repos as packages
- **Smart adopt workflow** --- migrates existing configs automatically
- **Dry-run mode** --- preview every operation before it runs

## Installation

```bash
go install github.com/antoinegs/tidydots/cmd/tidydots@latest
```

Or build from source:

```bash
git clone https://github.com/antoinegs/tidydots.git
cd tidydots
go build ./cmd/tidydots
```

## Quick Start

1. **Initialize** with your dotfiles repository:

   ```bash
   tidydots init /path/to/your/dotfiles
   ```

2. **Create** a `tidydots.yaml` in your dotfiles repo:

   ```yaml
   version: 3

   applications:
     - name: "nvim"
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
   ```

3. **Restore** your configs:

   ```bash
   tidydots restore
   ```

   or let the TUI guide you through the process:

   ```bash
   tidydots
   ```

See the [Quick Start guide](https://tidydots.io/getting-started/quick-start/)
for a full walkthrough, or explore the
[configuration reference](https://tidydots.io/configuration/overview/) for all
available options.

## Documentation

The full documentation is available at **[tidydots.io](https://tidydots.io)**
and covers:

- [Getting Started](https://tidydots.io/getting-started/quick-start/) ---
  installation, quick start, and core concepts
- [Configuration](https://tidydots.io/configuration/overview/) --- applications,
  configs, packages, and templates
- [Guides](https://tidydots.io/guides/multi-machine-setups/) --- multi-machine
  setups, package management, system configs
- [CLI Reference](https://tidydots.io/cli/reference/) --- all commands and flags
- [Troubleshooting](https://tidydots.io/troubleshooting/) --- common issues and
  solutions

## License

MIT
