# Packages

The `package` field on an [Application](applications.md) defines how to install the software associated with that application. tidydots supports multiple installation methods: system package managers, git repository clones, custom shell commands, installer scripts, and URL downloads.

## Package Schema (EntryPackage)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `managers` | map[string]ManagerValue | no | Package manager mappings |
| `custom` | map[string]string | no | OS-specific custom shell commands |
| `url` | map[string]URLInstallSpec | no | OS-specific URL download + install |

At least one of `managers`, `custom`, or `url` should be specified for the package to be installable.

## Installation Methods

tidydots tries installation methods in this order:

1. **Git packages** (if `managers.git` is defined)
2. **Installer packages** (if `managers.installer` is defined)
3. **Standard package managers** (first available manager from `managers`)
4. **Custom commands** (if `custom` has a command for the current OS)
5. **URL downloads** (if `url` has a spec for the current OS)

### Standard Package Managers

Map package manager names to package identifiers. tidydots selects the first available manager on the system.

```yaml
package:
  managers:
    pacman: "neovim"
    apt: "neovim"
    brew: "neovim"
    winget: "Neovim.Neovim"
    scoop: "neovim"
```

Each key is a package manager name and the value is the package identifier string for that manager.

### Git Packages

Clone or update a git repository as a package. The `managers.git` key takes a nested object instead of a string.

```yaml
package:
  managers:
    git:
      url: "https://github.com/tmux-plugins/tpm.git"
      branch: "master"
      targets:
        linux: "~/.tmux/plugins/tpm"
      sudo: false
```

**Git package fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | yes | Repository URL to clone |
| `branch` | string | no | Branch to clone (defaults to repo default branch) |
| `targets` | map[string]string | yes | OS-specific clone destination paths |
| `sudo` | bool | no | Run git commands with sudo (default: false) |

**Behavior:**

- If the target directory exists and contains a `.git/` subdirectory, tidydots runs `git pull` to update
- If the target directory does not exist, tidydots runs `git clone`
- Paths support `~` expansion

### Installer Packages

Run OS-specific shell commands to install software. The `managers.installer` key takes a nested object with a command map and optional binary check.

```yaml
package:
  managers:
    installer:
      command:
        linux: "curl -fsSL https://example.com/install.sh | sh"
        windows: "irm https://example.com/install.ps1 | iex"
      binary: "mytool"
```

**Installer package fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | map[string]string | yes | OS-specific shell commands to run |
| `binary` | string | no | Binary name to check if already installed (via PATH lookup) |

**Behavior:**

- On Linux, commands run via `sh -c`
- On Windows, commands run via `powershell -Command`
- If `binary` is specified, tidydots checks if it exists in PATH before running the install command

!!! warning "Security"
    Installer commands execute arbitrary shell commands from your configuration file. Only use configurations you trust.

### Custom Commands

Run an OS-specific shell command. Unlike installer packages, custom commands are defined outside the `managers` map.

```yaml
package:
  custom:
    linux: "cargo install ripgrep"
    windows: "cargo install ripgrep"
```

| Key | Value |
|-----|-------|
| OS name (`linux`, `windows`) | Shell command to execute |

**Behavior:**

- On Linux, runs via `sh -c`
- On Windows, runs via `powershell -Command`

!!! warning "Security"
    Custom commands execute arbitrary shell commands from your configuration file. Only use configurations you trust.

### URL Downloads

Download a file from a URL and run an install command against it.

```yaml
package:
  url:
    linux:
      url: "https://github.com/example/tool/releases/latest/download/tool-linux-amd64.tar.gz"
      command: "tar xzf {file} -C ~/.local/bin"
    windows:
      url: "https://github.com/example/tool/releases/latest/download/tool-windows.zip"
      command: "Expand-Archive -Path {file} -DestinationPath $HOME/.local/bin"
```

**URL install fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | yes | URL to download |
| `command` | string | yes | Shell command to run after download. Use `{file}` as placeholder for the downloaded file path |

**Behavior:**

- tidydots downloads the file to a temporary directory
- The `{file}` placeholder in `command` is replaced with the path to the downloaded file
- On Linux, download uses `curl -fsSL`; on Windows, uses `Invoke-WebRequest`
- The temporary directory is cleaned up after installation

!!! warning "Security"
    URL downloads execute content from external sources. Only use URLs you trust.

## Supported Package Managers

| Platform | Managers | Notes |
|----------|----------|-------|
| Arch Linux | `pacman`, `yay`, `paru` | `yay` and `paru` are AUR helpers |
| Debian / Ubuntu | `apt` | Uses `apt-get install -y` |
| Fedora / RHEL | `dnf` | Uses `dnf install -y` |
| macOS | `brew` | Homebrew |
| Windows | `winget`, `scoop`, `choco` | Windows Package Manager, Scoop, Chocolatey |

All standard managers are detected by checking if their binary is available in PATH.

## Manager Selection

tidydots selects which package manager to use through a priority system:

### 1. manager_priority (Highest Priority)

If `manager_priority` is set in `tidydots.yaml`, tidydots iterates the list and uses the first manager that is both listed and available on the system.

```yaml
manager_priority:
  - paru
  - yay
  - pacman
```

### 2. default_manager

If `manager_priority` is not set (or none of its entries are available), tidydots checks `default_manager`. If it is set and available, it is used.

```yaml
default_manager: "yay"
```

### 3. Auto-Selection (Fallback)

If neither setting applies, tidydots auto-selects based on the OS:

=== "Linux / macOS"

    Tried in order: `yay` > `paru` > `pacman` > `apt` > `dnf` > `brew`

=== "Windows"

    Tried in order: `winget` > `scoop` > `choco`

The first available manager wins.

!!! note
    Manager selection applies only to standard package managers. Git, installer, custom, and URL methods are used whenever their configuration matches the current OS, regardless of manager selection.

## Complete Examples

### Application with Multiple Manager Types

```yaml
applications:
  - name: "development-tools"
    description: "Core development environment"
    entries:
      - name: "dev-config"
        backup: "./dev"
        targets:
          linux: "~/.config/dev"
    package:
      managers:
        pacman: "base-devel"
        apt: "build-essential"
        brew: "gcc"
```

### Git Repository Package

```yaml
applications:
  - name: "tmux"
    description: "Terminal multiplexer"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "tmux-config"
        backup: "./tmux"
        targets:
          linux: "~/.config/tmux"
    package:
      managers:
        git:
          url: "https://github.com/tmux-plugins/tpm.git"
          branch: "master"
          targets:
            linux: "~/.tmux/plugins/tpm"
```

### Installer with Binary Check

```yaml
applications:
  - name: "starship"
    description: "Cross-shell prompt"
    entries:
      - name: "starship-config"
        backup: "./starship"
        targets:
          linux: "~/.config"
    package:
      managers:
        pacman: "starship"
        brew: "starship"
        installer:
          command:
            linux: "curl -sS https://starship.rs/install.sh | sh -s -- -y"
            windows: "winget install --id Starship.Starship"
          binary: "starship"
```

In this example, if `pacman` or `brew` is available, that manager is used. Otherwise, the installer command runs -- but only if `starship` is not already found in PATH.

### Custom Command Fallback

```yaml
applications:
  - name: "rust"
    description: "Rust programming language"
    entries: []
    package:
      managers:
        pacman: "rust"
        apt: "rustc"
        brew: "rust"
      custom:
        linux: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y"
```

### URL Download

```yaml
applications:
  - name: "lazygit"
    description: "Terminal UI for git"
    entries: []
    package:
      managers:
        pacman: "lazygit"
        brew: "lazygit"
      url:
        linux:
          url: "https://github.com/jesseduffield/lazygit/releases/latest/download/lazygit_0.40_Linux_x86_64.tar.gz"
          command: "tar xzf {file} -C ~/.local/bin lazygit"
```

### Combining All Methods

```yaml
applications:
  - name: "tool"
    description: "Example with all installation methods"
    entries: []
    package:
      managers:
        pacman: "tool"
        apt: "tool"
        installer:
          command:
            linux: "curl -fsSL https://example.com/install.sh | sh"
          binary: "tool"
      custom:
        windows: "choco install tool -y"
      url:
        linux:
          url: "https://example.com/tool-linux.tar.gz"
          command: "tar xzf {file} -C ~/.local/bin"
```

tidydots tries methods in order: installer first (if defined), then standard managers, then custom, then URL. The first successful method wins.
