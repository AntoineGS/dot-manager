# Multi-Machine Setups

One of tidydots' greatest strengths is managing a single dotfiles repository that adapts to different machines. Whether you have a desktop, a laptop, a work machine, and a server, you can share one repo and let tidydots handle the differences.

## How it works

tidydots provides three mechanisms for per-machine customization:

1. **`when` expressions** -- conditionally include or exclude entire applications
2. **Template conditionals** -- render different file content based on the current machine
3. **Path templating** -- use machine-specific paths for config targets

All three use Go template syntax with access to the same context variables:

| Variable | Description | Example values |
|----------|-------------|----------------|
| `.OS` | Operating system | `"linux"`, `"windows"` |
| `.Distro` | Linux distribution ID | `"arch"`, `"ubuntu"`, `"fedora"` |
| `.Hostname` | Machine hostname | `"my-desktop"`, `"work-laptop"` |
| `.User` | Current username | `"alice"`, `"root"` |
| `.Env` | Environment variables map | `{{ index .Env "HOME" }}` |

## Conditional applications with `when`

The `when` field on an application controls whether it is included on the current machine. If the expression evaluates to anything other than `"true"`, the application and all its entries are skipped.

### Filter by hostname

Run an application only on a specific machine:

```yaml
applications:
  - name: "desktop-tweaks"
    description: "Settings only for my desktop"
    when: '{{ eq .Hostname "my-desktop" }}'
    entries:
      - name: "desktop-config"
        backup: "./desktop"
        targets:
          linux: "~/.config/desktop-tweaks"
```

### Filter by distribution

Restrict to a specific Linux distribution:

```yaml
applications:
  - name: "pacman-config"
    description: "Pacman configuration"
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        backup: "./pacman"
        targets:
          linux: "/etc/pacman.conf"
    package:
      managers:
        pacman: "pacman"
```

### Filter by operating system

Include an application only on Linux or only on Windows:

```yaml
applications:
  - name: "linux-tools"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "tmux-config"
        backup: "./tmux"
        targets:
          linux: "~/.config/tmux"
```

### Combining conditions

Use `and`, `or`, and `not` to build complex expressions:

```yaml
applications:
  # Only on Ubuntu Linux
  - name: "ubuntu-settings"
    when: '{{ and (eq .OS "linux") (eq .Distro "ubuntu") }}'
    entries:
      - name: "ubuntu-config"
        backup: "./ubuntu"
        targets:
          linux: "~/.config/ubuntu-settings"

  # On any Linux except Arch
  - name: "non-arch-settings"
    when: '{{ and (eq .OS "linux") (ne .Distro "arch") }}'
    entries:
      - name: "generic-config"
        backup: "./generic-linux"
        targets:
          linux: "~/.config/generic"

  # Only on my personal machines
  - name: "personal"
    when: '{{ or (eq .Hostname "my-desktop") (eq .Hostname "my-laptop") }}'
    entries:
      - name: "personal-config"
        backup: "./personal"
        targets:
          linux: "~/.config/personal"
```

!!! tip
    All [sprout](https://github.com/go-sprout/sprout) template functions are available in `when` expressions, giving you access to string manipulation, regular expressions, and more.

## Template conditionals in file content

For cases where you want the *same* config file but with *different content* on different machines, use template files. Any file with a `.tmpl` extension in your backup directory is rendered as a Go template during restore.

### Basic example: hostname-specific settings

Create a file called `kitty.conf.tmpl` in your backup directory:

```
# Shared settings
font_family      JetBrainsMono Nerd Font
font_size        11
background_opacity 0.9

{{ if eq .Hostname "my-desktop" -}}
# Desktop: larger monitor, enable fancy shader
custom_shader    shaders/fancy.glsl
custom_shader_animation always
{{- end }}

{{ if eq .Hostname "my-laptop" -}}
# Laptop: smaller screen, boost font size
font_size        13
background_opacity 1.0
{{- end }}
```

When you run `tidydots restore`:

- On `my-desktop`, the shader lines are included and the laptop block is omitted
- On `my-laptop`, the font size override is included and the desktop block is omitted
- On any other machine, both blocks are omitted and you get the shared defaults

!!! info "How template rendering works"
    1. `kitty.conf.tmpl` is rendered to `kitty.conf.tmpl.rendered` (a sibling file in the backup directory)
    2. A symlink is created from the target (e.g., `~/.config/kitty/kitty.conf`) to the `.tmpl.rendered` file
    3. Non-template files in the same directory get normal symlinks

### Distro-specific content

```
# Package manager aliases
{{ if eq .Distro "arch" -}}
alias update="sudo pacman -Syu"
alias install="sudo pacman -S"
{{- else if eq .Distro "ubuntu" -}}
alias update="sudo apt update && sudo apt upgrade"
alias install="sudo apt install"
{{- else if eq .Distro "fedora" -}}
alias update="sudo dnf upgrade"
alias install="sudo dnf install"
{{- end }}
```

### Environment variables

Access environment variables through the `.Env` map:

```
{{ if index .Env "WAYLAND_DISPLAY" -}}
# Wayland-specific settings
GDK_BACKEND=wayland
{{- else -}}
# X11 settings
GDK_BACKEND=x11
{{- end }}
```

!!! warning "Gitignore rendered files"
    Add these patterns to your dotfiles repo's `.gitignore` to avoid committing generated files:

    ```
    *.tmpl.rendered
    *.tmpl.conflict
    .tidydots.db
    ```

## Path templating

Config paths themselves can include template expressions. This lets you use different target directories on different machines.

### Per-hostname config directories

```yaml
applications:
  - name: "nvim"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/{{ .Hostname }}/nvim"
```

On a machine with hostname `my-desktop`, the symlink target becomes `~/.config/my-desktop/nvim`. On `my-laptop`, it becomes `~/.config/my-laptop/nvim`.

### Per-user paths

```yaml
applications:
  - name: "shell-config"
    entries:
      - name: "profile"
        backup: "./shell"
        targets:
          linux: "/home/{{ .User }}/.profile"
```

!!! note
    Paths without `{{` delimiters are treated as regular paths and go through the standard path expansion (tilde expansion, etc.). Template path evaluation is only triggered when `{{` is detected.

## Practical examples

### Desktop vs. laptop setup

A common scenario: you want the same applications on both machines but with different display settings.

```yaml
version: 3

applications:
  # Shared on all machines
  - name: "neovim"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"

  # Terminal emulator with per-machine rendering
  - name: "kitty"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "kitty-config"
        backup: "./kitty"
        targets:
          linux: "~/.config/kitty"
    package:
      managers:
        pacman: "kitty"
        apt: "kitty"

  # Desktop-only: GPU-intensive tools
  - name: "gpu-tools"
    when: '{{ eq .Hostname "my-desktop" }}'
    entries:
      - name: "gpu-config"
        backup: "./gpu"
        targets:
          linux: "~/.config/gpu-tools"
    package:
      managers:
        pacman: "nvtop"
```

And in `./kitty/kitty.conf.tmpl`:

```
font_family JetBrainsMono Nerd Font
{{ if eq .Hostname "my-desktop" -}}
font_size 11
background_opacity 0.85
{{- else -}}
font_size 13
background_opacity 1.0
{{- end }}
```

### Cross-platform dotfiles (Linux + Windows)

```yaml
version: 3

applications:
  - name: "neovim"
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
        winget: "Neovim.Neovim"
        scoop: "neovim"

  - name: "git-config"
    entries:
      - name: "gitconfig"
        files: [".gitconfig"]
        backup: "./git"
        targets:
          linux: "~"
          windows: "~"

  # Linux-only
  - name: "zsh"
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

  # Windows-only
  - name: "powershell"
    when: '{{ eq .OS "windows" }}'
    entries:
      - name: "ps-profile"
        backup: "./powershell"
        targets:
          windows: "~/Documents/PowerShell"
```

### Work vs. personal machines

Use hostname-based `when` expressions to separate work and personal configurations:

```yaml
applications:
  # Shared everywhere
  - name: "neovim"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"

  # Personal machines only
  - name: "gaming"
    when: '{{ or (eq .Hostname "my-desktop") (eq .Hostname "my-laptop") }}'
    entries:
      - name: "steam-config"
        backup: "./steam"
        targets:
          linux: "~/.config/steam"

  # Work machine only
  - name: "work-tools"
    when: '{{ eq .Hostname "work-laptop" }}'
    entries:
      - name: "work-config"
        backup: "./work"
        targets:
          linux: "~/.config/work"
```

## Debugging expressions

Use dry-run mode to verify which applications are included on the current machine without making any changes:

```bash
tidydots list -n
```

This shows all applications and entries that match the current machine's context, letting you confirm your `when` expressions are working as expected.

You can also override the OS for testing:

```bash
# See what would be included on Windows
tidydots list -o windows
```

## Next steps

- [Templates](../configuration/templates.md) -- full template syntax and 3-way merge details
- [System Configs](system-configs.md) -- managing files that require sudo
- [Configuration Overview](../configuration/overview.md) -- complete configuration reference
