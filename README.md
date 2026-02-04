# dot-manager

A cross-platform dotfile management tool written in Go. Manage configuration files through symlinks, clone git repositories, and install packages across multiple package managers.

## Features

- **Symlink-based restoration** - Configs are symlinked from your dotfiles repo, so edits sync immediately
- **Git repository cloning** - Clone repositories to specific locations
- **Cross-platform** - Works on Linux and Windows with OS-specific path targets
- **Flexible filtering** - Filter entries by OS, distro, hostname, or user with regex support
- **Interactive TUI** - Bubble Tea-based terminal UI for guided operations
- **Package management** - Install packages across pacman, yay, paru, apt, dnf, brew, winget, scoop, choco
- **Smart adoption** - Automatically backs up existing configs before symlinking
- **Dry-run mode** - Preview all operations before making changes
- **Root/sudo support** - Separate entries for system-level files with `root: true`

## Installation

```bash
go install github.com/antoinegs/dot-manager/cmd/dot-manager@latest
```

Or build from source:

```bash
git clone https://github.com/antoinegs/dot-manager.git
cd dot-manager
go build ./cmd/dot-manager
```

## Quick Start

1. **Initialize** with your dotfiles repository:

   ```bash
   dot-manager init /path/to/your/dotfiles
   ```

2. **Create a configuration file** (`dot-manager.yaml`) in your dotfiles repo:

   ```yaml
   version: 3
   backup_root: "."

   applications:
     # Neovim application with config and package
     - name: "neovim"
       description: "Neovim text editor"

       configs:
         - name: "nvim-config"
           backup: "./nvim"
           targets:
             linux: "~/.config/nvim"
             windows: "~/AppData/Local/nvim"

       packages:
         - name: "neovim"
           managers:
             pacman: "neovim"
             apt: "neovim"

       filters:
         - include:
             os: "linux"

     # Zsh configuration
     - name: "zsh"
       description: "Z shell configuration"

       configs:
         - name: "zshrc"
           files: [".zshrc", ".zshenv"]
           backup: "./zsh"
           targets:
             linux: "~"

       packages:
         - name: "oh-my-zsh"
           managers:
             git:
               url: "https://github.com/ohmyzsh/ohmyzsh.git"
               targets:
                 linux: "~/.oh-my-zsh"
   ```

3. **Restore your configs**:

   ```bash
   dot-manager restore
   ```

## Configuration

### App Config

Located at `~/.config/dot-manager/config.yaml`, stores the path to your dotfiles repository:

```yaml
config_dir: "/path/to/your/dotfiles"
```

### Repository Config

Located in your dotfiles repo as `dot-manager.yaml`:

```yaml
version: 3
backup_root: "."

# Package manager settings
default_manager: "pacman"
manager_priority: ["yay", "paru", "pacman"]

applications:
  # Application with config
  - name: "app-name"
    description: "Optional description"

    configs:
      - name: "config-name"
        files: []              # Empty = entire folder
        backup: "./backup/path"
        targets:
          linux: "~/.config/app"
          windows: "~/AppData/Local/app"

    packages:
      - name: "tool"
        managers:
          pacman: "tool"
          apt: "tool-package"

      - name: "git-repo"
        managers:
          git:
            url: "https://github.com/user/repo.git"
            branch: "main"         # Optional
            targets:
              linux: "~/path/to/clone"
            sudo: false

    filters:
      - include:
          os: "linux"

  # System-level application (requires sudo)
  - name: "pacman-hooks"
    sudo: true

    configs:
      - name: "hooks"
        files: ["my-hook.hook"]
        backup: "./Linux/pacman"
        targets:
          linux: "/usr/share/libalpm/hooks"

    filters:
      - include:
          distro: "arch"
```

### Configuration Structure

**Applications** group related configs and packages together:
- Each application can have multiple configs
- Each application can have multiple packages
- Filters can be applied at the application level

### Application Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Application name |
| `description` | string | Optional description |
| `sudo` | bool | Requires root/sudo for all configs |
| `configs` | []Config | Configuration entries |
| `packages` | []Package | Package definitions |
| `filters` | []Filter | Conditional filters |

### Config Entry Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Config entry name |
| `files` | []string | Specific files (empty = entire folder) |
| `backup` | string | Path in dotfiles repo |
| `targets` | map | OS-specific target paths |
| `sudo` | bool | Requires root/sudo |

### Filtering

Applications can be filtered based on OS, distro, hostname, or user. Filters use regex matching.

```yaml
applications:
  # Only on Arch Linux
  - name: "pacman-config"
    configs:
      - name: "pacman-conf"
        backup: "./pacman"
        targets:
          linux: "~/.config/pacman"
    filters:
      - include:
          distro: "arch"

  # On any system except work laptop
  - name: "personal-config"
    configs:
      - name: "personal"
        backup: "./personal"
        targets:
          linux: "~/.config/personal"
    filters:
      - exclude:
          hostname: "work-laptop"

  # Multiple conditions (AND within a filter, OR between filters)
  - name: "dev-tools"
    configs:
      - name: "dev-config"
        backup: "./dev"
        targets:
          linux: "~/.config/dev"
    filters:
      # Either: (linux AND arch) OR (linux AND user is dev)
      - include:
          os: "linux"
          distro: "arch"
      - include:
          os: "linux"
          user: "dev"
```

Filter attributes:
- `os` - Operating system (linux, windows)
- `distro` - Linux distribution ID (arch, ubuntu, fedora, debian, etc.)
- `hostname` - Machine hostname
- `user` - Current username

### Package Configuration

```yaml
package:
  managers:           # Package manager -> package name
    pacman: "pkg"
    apt: "pkg"
    brew: "pkg"
  custom:             # OS -> shell command
    linux: "install.sh"
  url:                # OS -> URL download
    linux:
      url: "https://example.com/file"
      command: "sudo install {file} /usr/local/bin/"
```

## Commands

| Command | Description |
|---------|-------------|
| `dot-manager` | Launch interactive TUI |
| `dot-manager init <path>` | Initialize app configuration |
| `dot-manager restore` | Restore configs by creating symlinks |
| `dot-manager backup` | Backup configs from target locations |
| `dot-manager list` | List all configured entries |
| `dot-manager install [packages...]` | Install packages |
| `dot-manager list-packages` | Display configured packages |

### Global Flags

| Flag | Description |
|------|-------------|
| `-d, --dir` | Override configurations directory |
| `-o, --os` | Override OS detection (linux/windows) |
| `-n, --dry-run` | Preview without making changes |
| `-v, --verbose` | Enable verbose output |
| `-i, --interactive` | Use interactive TUI mode |

## Examples

```bash
# Preview restore without making changes
dot-manager restore -n

# Interactive restore
dot-manager restore -i

# Backup all configs
dot-manager backup

# Install all packages
dot-manager install

# Install specific packages
dot-manager install neovim ripgrep fzf

# List all entries
dot-manager list

# List available packages
dot-manager list-packages

# Override OS for cross-platform testing
dot-manager list -o windows
```

## Supported Package Managers

| Platform | Managers |
|----------|----------|
| Arch Linux | yay, paru, pacman |
| Debian/Ubuntu | apt |
| Fedora/RHEL | dnf |
| macOS | brew |
| Windows | winget, scoop, choco |

## How It Works

### Restore Process

For **config entries**:
1. If backup exists and target doesn't: create symlink
2. If backup doesn't exist but target does: **adopt** (move target to backup, then symlink)
3. If already symlinked: skip

For **git entries**:
1. If target doesn't exist: clone repository
2. If target exists and is a git repo: skip (already cloned)
3. If target exists but not a git repo: skip with warning

### Backup Process

Copies files from target locations to the backup directory. Skips files that are already symlinks to avoid duplicating restored configs.

## License

MIT
