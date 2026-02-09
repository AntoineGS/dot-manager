# Configs (SubEntry)

A **SubEntry** (config entry) represents a single configuration that tidydots manages through symlinks. Config entries live inside an [Application's](applications.md) `entries` array and define where files are backed up in your dotfiles repo and where they should be symlinked on the target system.

## Schema Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Entry identifier, unique within its application |
| `backup` | string | yes | Path in the dotfiles repo where config files are stored |
| `targets` | map[string]string | yes | OS-specific target paths where symlinks are created |
| `files` | []string | no | Specific files to manage. Empty = entire folder |
| `sudo` | bool | no | Use elevated privileges for symlink operations |

## How It Works

When you run `tidydots restore`, for each config entry tidydots:

1. Reads the `backup` path (relative to `backup_root` in `tidydots.yaml`)
2. Looks up the `targets` map for the current OS
3. Creates a symlink from the target path pointing to the backup path
4. If `files` is specified, only those specific files are symlinked

The result is that your system reads configuration from the target path, but the actual files live in your dotfiles repository.

## Fields in Detail

### backup

The `backup` field specifies where the configuration files are stored in your dotfiles repository. This path is relative to `backup_root` (which defaults to the directory containing `tidydots.yaml`).

```yaml
backup: "./nvim"           # Relative to backup_root
backup: "./shell/zsh"      # Nested directory
```

!!! note
    The `backup` field is what makes an entry a "config entry." If `backup` is present, tidydots treats the entry as a symlink-managed configuration.

### targets

The `targets` field is a map from OS identifier to the target path on that OS. tidydots looks up the current OS and uses the corresponding path.

```yaml
targets:
  linux: "~/.config/nvim"
  windows: "~/AppData/Local/nvim"
```

Supported OS keys:

| Key | Platform |
|-----|----------|
| `linux` | Linux (all distributions) |
| `windows` | Windows |

Paths support `~` expansion to the user's home directory.

**Path templating** is also supported. Any path containing `{{ }}` delimiters is rendered as a Go template before expansion:

```yaml
targets:
  linux: "~/.config/{{ .Hostname }}/nvim"
```

See [Templates](templates.md) for available template variables and functions.

### files

The `files` field is an optional list of specific filenames to manage. When specified, only those files are symlinked individually. When omitted or empty, the entire folder is symlinked.

```yaml
# Symlink specific files only
files:
  - ".zshrc"
  - ".zprofile"
```

```yaml
# Symlink the entire folder (files omitted)
files: []
```

!!! tip
    Use `files` when you want to manage individual dotfiles from a backup directory that may contain other files you do not want symlinked. Leave `files` empty when you want the entire directory structure managed as a unit.

### sudo

When `sudo: true` is set, tidydots uses elevated privileges for all symlink operations on this entry. This is required for targets outside your home directory, such as system configuration files.

```yaml
sudo: true
```

!!! warning
    Only set `sudo: true` when the target path genuinely requires elevated privileges (e.g., `/etc/` paths). Using sudo unnecessarily may create files owned by root in unexpected locations.

## Examples

### Single File

Manage a single configuration file:

```yaml
entries:
  - name: "gitconfig"
    backup: "./git"
    files:
      - ".gitconfig"
    targets:
      linux: "~"
      windows: "~"
```

This creates a symlink `~/.gitconfig` pointing to `<dotfiles>/git/.gitconfig`.

### Entire Folder

Manage an entire configuration directory:

```yaml
entries:
  - name: "nvim-config"
    backup: "./nvim"
    targets:
      linux: "~/.config/nvim"
      windows: "~/AppData/Local/nvim"
```

This creates a symlink `~/.config/nvim` pointing to `<dotfiles>/nvim/`.

### Multiple Files from One Backup

Pick specific files from a backup directory:

```yaml
entries:
  - name: "zsh-dotfiles"
    backup: "./zsh"
    files:
      - ".zshrc"
      - ".zprofile"
      - ".zshenv"
    targets:
      linux: "~"
```

This creates three individual symlinks (`~/.zshrc`, `~/.zprofile`, `~/.zshenv`), each pointing to the corresponding file in `<dotfiles>/zsh/`.

### Cross-Platform Entry

Define different target paths per OS:

```yaml
entries:
  - name: "terminal-config"
    backup: "./alacritty"
    targets:
      linux: "~/.config/alacritty"
      windows: "~/AppData/Roaming/alacritty"
```

On Linux the symlink is at `~/.config/alacritty`; on Windows it is at `~/AppData/Roaming/alacritty`. Both point to the same `<dotfiles>/alacritty/` backup directory.

### Path Templates

Use template expressions in target paths for host-specific layouts:

```yaml
entries:
  - name: "nvim-config"
    backup: "./nvim"
    targets:
      linux: "~/.config/{{ .Hostname }}/nvim"
```

On a machine with hostname `desktop`, this resolves to `~/.config/desktop/nvim`. See [Templates](templates.md) for the full set of available variables.

### System-Level Config with Sudo

Manage files that require root privileges:

```yaml
entries:
  - name: "pacman-conf"
    sudo: true
    backup: "./system/pacman"
    files:
      - "pacman.conf"
    targets:
      linux: "/etc"

  - name: "hosts"
    sudo: true
    backup: "./system"
    files:
      - "hosts"
    targets:
      linux: "/etc"
```

### Multiple Entries in One Application

Group related configs under one application:

```yaml
applications:
  - name: "zsh"
    description: "Z shell configuration"
    when: '{{ ne .OS "windows" }}'
    entries:
      - name: "zsh-dotfiles"
        backup: "./zsh"
        files:
          - ".zshrc"
          - ".zprofile"
        targets:
          linux: "~"

      - name: "zsh-custom"
        backup: "./zsh/custom"
        targets:
          linux: "~/.config/zsh/custom"
    package:
      managers:
        pacman: "zsh"
        apt: "zsh"
        brew: "zsh"
```

## Template Files in Config Entries

Config entries can contain `.tmpl` template files in their backup directory. During restore, these files are rendered using the template engine, and the output is written as `.tmpl.rendered` sibling files. The symlink then points to the rendered output.

For example, if your backup directory contains `alacritty.toml.tmpl`:

1. tidydots renders the template to `alacritty.toml.tmpl.rendered`
2. A relative symlink `alacritty.toml` is created pointing to `alacritty.toml.tmpl.rendered`
3. The folder-level symlink from the target path points to the backup directory as usual

See [Templates](templates.md) for the full template system documentation.
