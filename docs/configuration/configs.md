# Configs (SubEntry)

A **SubEntry** (config entry) represents a single configuration that tidydots manages, by default through symlinks. Config entries live inside an [Application's](applications.md) `entries` array and define where files are backed up in your dotfiles repo and where they should be deployed on the target system.

## Schema Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Entry identifier, unique within its application |
| `when` | string | no | Go template expression that conditionally includes this entry |
| `backup` | string | yes | Path in the dotfiles repo where config files are stored |
| `targets` | map[string]string | yes | OS-specific target paths where files are deployed |
| `files` | []string | no | Specific files to manage. Empty = entire folder |
| `method` | string | no | Deployment method: `symlink` (default) or `copy`. See [Deployment Method](#deployment-method) |
| `sudo` | bool | no | Use elevated privileges for deployment operations |

## How It Works

When you run `tidydots restore`, for each config entry tidydots:

1. Reads the `backup` path (relative to the config directory)
2. Looks up the `targets` map for the current OS
3. Creates a symlink from the target path pointing to the backup path (or writes a real file copy, if `method: copy` is set — see [Deployment Method](#deployment-method))
4. If `files` is specified, only those specific files are symlinked (or copied)

The result is that your system reads configuration from the target path, but the actual files live in your dotfiles repository.

## Conditional Entries

`when` is optional on every config entry, including the default symlink method and
`method: copy`. It uses the same Go-template expressions and context as an
[application `when`](applications.md#when-expressions). An entry is included only
when **both** its application's `when` and its own `when` evaluate to `true`.

```yaml
entries:
  - name: omarchbook .desktop files
    when: '{{ eq .Hostname "omarchbook" }}'
    backup: ./Linux/os/applications/omarchbook
    targets:
      linux: ~/.local/share/applications/omarchbook
```

This keeps the desktop files out of every machine except `omarchbook`, while other
entries in the same application can still apply elsewhere. A false result or a
template error excludes the entry before tidydots reads its `targets` or `backup`
or performs any config operation. Excluded entries are also omitted from `list` and
the TUI manage view. If you explicitly target one on the CLI, for example
`tidydots restore os "omarchbook .desktop files"`, tidydots returns a conditions
mismatch error instead of restoring it.

## Fields in Detail

### backup

The `backup` field specifies where the configuration files are stored in your dotfiles repository. This path is relative to the directory containing `tidydots.yaml`.

```yaml
backup: "./nvim"           # Relative to config directory
backup: "./shell/zsh"      # Nested directory
```

!!! note
    The `backup` field is what makes an entry a "config entry." If `backup` is present, tidydots treats the entry as a symlink-managed configuration.

### targets

The `targets` field is a map from OS identifier to the target path on that OS. tidydots looks up the current OS and uses the corresponding path.

At least one target must be declared. A config entry with `backup` set but no targets will be rejected during validation.

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

### method

The `method` field selects how tidydots deploys this entry's files to the target path:

- `symlink` (default, or when `method` is omitted) — creates a symlink at the target pointing back into the dotfiles repo.
- `copy` — writes a real, independent file at the target instead. The repo file remains the source of truth.

```yaml
method: copy
```

See [Deployment Method](#deployment-method) below for the full behavior, migration notes, and v1 limitations.

### sudo

When `sudo: true` is set, tidydots uses elevated privileges for all symlink operations on this entry. This is required for targets outside your home directory, such as system configuration files.

```yaml
sudo: true
```

!!! warning
    Only set `sudo: true` when the target path genuinely requires elevated privileges (e.g., `/etc/` paths). Using sudo unnecessarily may create files owned by root in unexpected locations.

## Deployment Method

By default, config entries are deployed as symlinks: the target path becomes a symlink pointing back into your dotfiles repo, and the repo file is what you actually edit. Setting `method: copy` on an entry switches to writing a real, independent file at the target instead.

```yaml
entries:
  - name: "blacklist"
    method: copy
    when: '{{ eq .Hostname "omarchbook" }}'
    files: ["blacklist-raydium.conf"]
    backup: "./Linux/modprobe"
    targets:
      linux: "/etc/modprobe.d"
    sudo: true
```

### Symlink vs. Copy

| Method | Target becomes | How updates propagate |
|--------|-----------------|------------------------|
| `symlink` (default) | A symlink into the dotfiles repo | Immediate — the target always reflects the repo file |
| `copy` | A real, independent file | Only on the next `tidydots restore` |

### Refresh and Idempotency

With `method: copy`, every `tidydots restore` compares the target file's content against the corresponding repo (backup) file:

- If the contents differ, the target is overwritten with the current repo content.
- If the contents already match, tidydots makes no changes (a no-op).

Because the target is a real file rather than a live link, editing the file in the dotfiles repo and re-running `tidydots restore` is how changes reach a copy-mode target.

!!! warning
    If the target already exists as a real file and its contents differ from the repo's, `tidydots restore` **overwrites it with the repo content**. Unlike `symlink` mode, copy mode does **not** merge the existing target's content into your backup first — whatever was at the target is lost. The `--no-merge` and `--force` flags that control this behavior for symlink entries do not apply to `copy` entries; copy mode always overwrites on drift, unconditionally. Back up any pre-existing target file yourself before pointing a new `method: copy` entry at it.

### Migrating from Symlink to Copy

If the target currently exists as a symlink (for example, the entry was previously deployed with `method: symlink`, or adopted), switching the entry to `method: copy` and re-running `tidydots restore` removes the existing symlink and replaces it with a real file copied from the repo. This makes symlink-to-copy migration safe without any manual cleanup.

### Limitations

- **Files only** — `method: copy` requires an explicit, non-empty `files:` list. Whole-folder copying (`files: []`) is not supported and is rejected during config validation.
- **Literal `.tmpl` files** — Copy entries do not render templates. A selected `config.toml.tmpl` is copied literally to `config.toml.tmpl`; use the default symlink method when that source should render. See [Templates](templates.md#explicit-file-selections).

### When to Use It

Use `method: copy` for files that must be readable very early in boot, before `$HOME` (or an encrypted subvolume containing your dotfiles repo) is mounted — for example `/etc/modprobe.d` or `/etc/udev/rules.d`. At that point in boot, a symlink into the dotfiles repo would be a dangling link, since its target isn't available yet; a real copied file has no such dependency and is readable immediately.

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

### Single Template File

Select a template by its source filename, including the `.tmpl` suffix:

```yaml
entries:
  - name: "gitconfig"
    backup: "./git"
    files:
      - ".gitconfig.tmpl"
    targets:
      linux: "~"
```

This renders and deploys `~/.gitconfig`; it does not deploy a file named
`~/.gitconfig.tmpl`. See [Explicit file selections](templates.md#explicit-file-selections)
for the generated-file layout and safety rules.

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

For an explicit `files:` list, name the source with its `.tmpl` suffix (for
example, `.gitconfig.tmpl`). Only those selected sources render; a suffix-free
entry such as `.gitconfig` remains an ordinary file selection and does not
implicitly discover the template. `method: copy` always copies `.tmpl` files
literally.

See [Templates](templates.md) for the full template system documentation.
