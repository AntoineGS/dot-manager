# Package Management

tidydots can install packages alongside your configuration files. Define which packages each application needs, and tidydots handles installing them with the right package manager for the current system.

## Supported package managers

| Platform | Managers |
|----------|----------|
| Arch Linux | pacman, yay, paru |
| Debian/Ubuntu | apt |
| Fedora/RHEL | dnf |
| macOS | brew |
| Windows | winget, scoop, choco |

tidydots automatically detects which package managers are available on the current system. You only need to define the package names -- tidydots picks the right manager.

## Configuring packages

Packages are defined at the application level using the `package` field (singular). Each application can have one package definition with mappings for multiple managers.

### Basic package definition

```yaml
applications:
  - name: "neovim"
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
        winget: "Neovim.Neovim"
        scoop: "neovim"
```

When you run `tidydots install`, tidydots chooses an available manager that has
a package name configured for that application, using configured priority,
then the default manager, then platform availability order.

### Package-only applications

An application does not need config entries. You can define applications that only install packages:

```yaml
applications:
  - name: "ripgrep"
    description: "Fast recursive search"
    package:
      managers:
        pacman: "ripgrep"
        apt: "ripgrep"
        brew: "ripgrep"
        winget: "BurntSushi.ripgrep.MSVC"

  - name: "fzf"
    description: "Fuzzy finder"
    package:
      managers:
        pacman: "fzf"
        apt: "fzf"
        brew: "fzf"
```

### Different package names per manager

Some packages have different names across distributions. The `managers` map lets you specify the correct name for each:

```yaml
applications:
  - name: "fd"
    description: "Simple, fast alternative to find"
    package:
      managers:
        pacman: "fd"
        apt: "fd-find"
        brew: "fd"
        winget: "sharkdp.fd"
```

## Manager priority system

When multiple package managers are available on the same system (common on Arch Linux with AUR helpers), tidydots needs to know which one to prefer. You control this with two settings in your `tidydots.yaml`.

### `manager_priority`

An ordered list of preferred managers. tidydots tries each one in order and
uses the first that is both available and configured with a package name for
the application:

```yaml
version: 3
manager_priority: ["paru", "yay", "pacman"]

applications:
  - name: "neovim"
    package:
      managers:
        pacman: "neovim"
```

With this configuration, tidydots uses the first available priority manager
that is configured for the application. For an application that only declares
`pacman: "neovim"`, it uses `pacman` even if `paru` is installed.

!!! tip
    This is particularly useful on Arch Linux where you might want to prefer an AUR helper (yay/paru) over pacman, since AUR helpers can install both official and AUR packages.

### `default_manager`

A simpler alternative -- specify a single preferred manager:

```yaml
version: 3
default_manager: "pacman"

applications:
  - name: "neovim"
    package:
      managers:
        pacman: "neovim"
```

### How selection works

tidydots selects a package manager in this order:

1. Use the first `manager_priority` entry that is both configured for the application and available
2. Otherwise use `default_manager` when it is configured for the application and available
3. Otherwise use the first available configured manager in platform availability order

Git and `installer` package definitions take precedence over ordinary manager
selection. `custom` and `url` are fallbacks. A manager entry containing only
`deps` is dependency-only and cannot become the main installation method.

### Dependencies and validation

tidydots selects and validates the main method before running dependencies.
It then installs dependencies from eligible standard manager entries before the
selected main method. If any dependency fails, tidydots does not run the main
install. This validation means an invalid or blank git target, invalid git URL,
or missing installer command for the current OS cannot leave a partial
dependency installation. Dry-run performs the same validation and shows the
planned dependency commands followed by the selected main command, without
running install commands. A failure in the selected main method does not fall
through to a later method.

## Custom installers

For software that is not available through standard package managers, you can define custom install commands per OS:

```yaml
applications:
  - name: "custom-tool"
    description: "Tool installed via custom script"
    package:
      custom:
        linux: "curl -fsSL https://example.com/install.sh | bash"
        windows: "irm https://example.com/install.ps1 | iex"
```

## URL-based installation

You can also install software by downloading from a URL:

```yaml
applications:
  - name: "binary-tool"
    description: "Tool downloaded from URL"
    package:
      url:
        linux:
          url: "https://github.com/user/tool/releases/latest/download/tool-linux-amd64"
          command: "sudo install {file} /usr/local/bin/tool"
```

The `{file}` placeholder is replaced with the path to the downloaded file.

## CLI commands

### Install all packages

Install every package defined in your `tidydots.yaml`:

```bash
tidydots install
```

tidydots skips packages whose manager is not available on the current system and respects `when` expressions to skip applications that do not match the current machine.

### Install specific packages

Install only named packages:

```bash
tidydots install neovim ripgrep fzf
```

The names must match the `name` field of the application in your `tidydots.yaml`.

### Preview with dry-run

See what would be installed without actually running any install commands:

```bash
tidydots install -n
```

```
[DRY-RUN] Would install neovim via pacman
[DRY-RUN] Would install ripgrep via pacman
[DRY-RUN] Would install fzf via pacman
```

!!! tip
    Always run with `-n` first when testing a new configuration to verify that the right packages and managers are selected.

### List configured packages

View all packages and their configured managers:

```bash
tidydots list-packages
```

This shows each package, the managers it supports, and which manager would be used on the current system.

### Interactive mode

Launch the interactive TUI for package installation:

```bash
tidydots install -i
```

In interactive mode, you can browse applications, select which packages to install, and review the operations before they run. See the [Interactive TUI](interactive-tui.md) guide for details.

## Practical examples

### Full development environment

```yaml
version: 3
manager_priority: ["paru", "yay", "pacman"]

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
        brew: "neovim"

  - name: "ripgrep"
    description: "Fast search tool"
    package:
      managers:
        pacman: "ripgrep"
        apt: "ripgrep"
        brew: "ripgrep"

  - name: "fzf"
    description: "Fuzzy finder"
    package:
      managers:
        pacman: "fzf"
        apt: "fzf"
        brew: "fzf"

  - name: "lazygit"
    description: "Terminal UI for git"
    package:
      managers:
        pacman: "lazygit"
        brew: "lazygit"

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
        pacman: "tmux"
        apt: "tmux"
        brew: "tmux"
```

### Distro-specific packages

Use `when` to define packages that only apply to certain distributions:

```yaml
applications:
  # Arch-specific AUR packages
  - name: "visual-studio-code"
    when: '{{ eq .Distro "arch" }}'
    package:
      managers:
        paru: "visual-studio-code-bin"

  # Ubuntu-specific PPA packages
  - name: "neovim-unstable"
    when: '{{ eq .Distro "ubuntu" }}'
    package:
      managers:
        apt: "neovim"
```

## Verbose output

For troubleshooting, use verbose mode to see detailed information about manager detection and package installation:

```bash
tidydots install -v
```

This shows which managers were detected, which manager was selected for each package, and the exact commands being run.

## Next steps

- [Git Repositories](git-repositories.md) -- using git as a package manager to clone repos
- [Interactive TUI](interactive-tui.md) -- batch install with the terminal UI
- [Multi-Machine Setups](multi-machine-setups.md) -- per-machine package selection with `when`
