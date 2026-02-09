# Git Repositories

tidydots can clone and update git repositories as part of your dotfiles setup. This is useful for tools distributed as git repos (oh-my-zsh, vim plugin managers, custom scripts) that you want cloned to specific locations on each machine.

## How it works

Git is treated as a special package manager within the `package` field. When you run `tidydots install` or `tidydots restore`:

- If the target directory **does not exist**, tidydots runs `git clone` to create it
- If the target directory **exists and contains a `.git/` directory**, tidydots runs `git pull` to update it
- If the target directory **exists but is not a git repo**, tidydots skips it with a warning

## Configuration syntax

Git repositories are configured under `package.managers.git` at the application level:

```yaml
applications:
  - name: "oh-my-zsh"
    description: "Zsh framework"
    package:
      managers:
        git:
          url: "https://github.com/ohmyzsh/ohmyzsh.git"
          branch: "master"
          targets:
            linux: "~/.oh-my-zsh"
          sudo: false
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | The repository URL to clone |
| `branch` | string | No | Branch to check out (defaults to the repo's default branch) |
| `targets` | map | Yes | OS-specific clone destinations |
| `sudo` | bool | No | Run git commands with sudo (default: `false`) |

!!! note
    The `targets` map works the same as config entry targets -- you can specify different paths for `linux` and `windows`.

## Basic examples

### Oh-my-zsh

```yaml
applications:
  - name: "oh-my-zsh"
    description: "Zsh framework for managing configuration"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "zshrc"
        files: [".zshrc"]
        backup: "./zsh"
        targets:
          linux: "~"
    package:
      managers:
        git:
          url: "https://github.com/ohmyzsh/ohmyzsh.git"
          targets:
            linux: "~/.oh-my-zsh"
```

This clones oh-my-zsh to `~/.oh-my-zsh` and symlinks your `.zshrc` from your dotfiles repo.

### Zsh plugins

Clone individual zsh plugins alongside oh-my-zsh. Each plugin is a separate application:

```yaml
applications:
  - name: "zsh-autosuggestions"
    description: "Fish-like autosuggestions for zsh"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/zsh-users/zsh-autosuggestions.git"
          targets:
            linux: "~/.oh-my-zsh/custom/plugins/zsh-autosuggestions"

  - name: "zsh-syntax-highlighting"
    description: "Fish-like syntax highlighting for zsh"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/zsh-users/zsh-syntax-highlighting.git"
          targets:
            linux: "~/.oh-my-zsh/custom/plugins/zsh-syntax-highlighting"
```

### Vim/Neovim plugin manager

```yaml
applications:
  - name: "lazy-nvim"
    description: "Neovim plugin manager"
    package:
      managers:
        git:
          url: "https://github.com/folke/lazy.nvim.git"
          branch: "stable"
          targets:
            linux: "~/.local/share/nvim/lazy/lazy.nvim"
            windows: "~/AppData/Local/nvim-data/lazy/lazy.nvim"
```

### Tmux plugin manager

```yaml
applications:
  - name: "tpm"
    description: "Tmux Plugin Manager"
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
          targets:
            linux: "~/.tmux/plugins/tpm"
```

## Specifying a branch

By default, tidydots clones the repository's default branch. Use the `branch` field to check out a specific branch:

```yaml
applications:
  - name: "my-scripts"
    description: "Personal utility scripts"
    package:
      managers:
        git:
          url: "https://github.com/user/scripts.git"
          branch: "main"
          targets:
            linux: "~/.local/share/scripts"
```

!!! tip
    Pinning a branch is recommended for stability. Without it, the clone uses whatever the remote's HEAD points to, which could change if the upstream renames their default branch.

## Cross-platform targets

Specify different clone destinations for each operating system:

```yaml
applications:
  - name: "dotfiles-extras"
    description: "Extra configuration scripts"
    package:
      managers:
        git:
          url: "https://github.com/user/dotfiles-extras.git"
          targets:
            linux: "~/.local/share/dotfiles-extras"
            windows: "~/AppData/Local/dotfiles-extras"
```

## Sudo git clones

For repositories that need to be cloned to system-level directories, set `sudo: true`:

```yaml
applications:
  - name: "system-scripts"
    description: "System-wide utility scripts"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/org/system-scripts.git"
          branch: "main"
          targets:
            linux: "/opt/system-scripts"
          sudo: true
```

!!! warning "Security consideration"
    Only use `sudo: true` for repositories you trust. The git commands (clone and pull) run as root when sudo is enabled. Review the repository contents before granting elevated access.

## Combining git with standard managers

A single application can have both standard package managers and a git repo. tidydots installs the standard package via the system manager and clones the git repository separately:

```yaml
applications:
  - name: "neovim"
    description: "Neovim text editor with plugin manager"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        git:
          url: "https://github.com/folke/lazy.nvim.git"
          branch: "stable"
          targets:
            linux: "~/.local/share/nvim/lazy/lazy.nvim"
```

In this example, `tidydots install` installs neovim via pacman (or apt) and clones lazy.nvim to the correct directory.

## CLI usage

### Install all git repos

```bash
tidydots install
```

This processes all packages, including git repositories. Git repos are cloned or pulled alongside standard package installations.

### Dry-run

Preview what would be cloned or pulled:

```bash
tidydots install -n
```

```
[DRY-RUN] Would clone https://github.com/ohmyzsh/ohmyzsh.git to ~/.oh-my-zsh
[DRY-RUN] Would pull in ~/.oh-my-zsh (already cloned)
```

### Verbose output

See the git commands being executed:

```bash
tidydots install -v
```

## Practical example: complete shell environment

Here is a full configuration that sets up a zsh environment with oh-my-zsh, custom plugins, and a theme:

```yaml
version: 3
backup_root: "."

applications:
  - name: "zsh"
    description: "Z shell"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "zshrc"
        files: [".zshrc", ".zshenv"]
        backup: "./zsh"
        targets:
          linux: "~"
    package:
      managers:
        pacman: "zsh"
        apt: "zsh"

  - name: "oh-my-zsh"
    description: "Zsh framework"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/ohmyzsh/ohmyzsh.git"
          targets:
            linux: "~/.oh-my-zsh"

  - name: "zsh-autosuggestions"
    description: "Fish-like autosuggestions"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/zsh-users/zsh-autosuggestions.git"
          targets:
            linux: "~/.oh-my-zsh/custom/plugins/zsh-autosuggestions"

  - name: "zsh-syntax-highlighting"
    description: "Fish-like syntax highlighting"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/zsh-users/zsh-syntax-highlighting.git"
          targets:
            linux: "~/.oh-my-zsh/custom/plugins/zsh-syntax-highlighting"

  - name: "powerlevel10k"
    description: "Zsh theme"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        git:
          url: "https://github.com/romkatv/powerlevel10k.git"
          targets:
            linux: "~/.oh-my-zsh/custom/themes/powerlevel10k"
```

After running `tidydots install`, you get a fully configured zsh environment with oh-my-zsh, two plugins, and the powerlevel10k theme -- all managed through your dotfiles repo.

## Next steps

- [Package Management](package-management.md) -- standard package manager support
- [System Configs](system-configs.md) -- sudo for system-level files and repos
- [Multi-Machine Setups](multi-machine-setups.md) -- per-machine git repos with `when`
