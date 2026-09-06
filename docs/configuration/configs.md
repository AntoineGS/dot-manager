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

With the default symlink method, the target reads from the backup in your dotfiles
repository. Copy mode instead deploys independent files at the target and updates
them only during a later restore.

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
    The `backup` field is what makes an entry a "config entry." The entry's `method` then selects symlink or copy deployment.

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
- `copy` — writes a real, independent file at the target instead. Ordinary files remain source-controlled by the repo and overwrite target drift; selected `.tmpl` files are rendered and merged using render history (see [Templates](templates.md#copy-mode-template-deployment)).

```yaml
method: copy
```

See [Deployment Method](#deployment-method) below for the full behavior, migration notes, and v1 limitations.

### sudo

When `sudo: true` is set, tidydots uses elevated privileges for deployment operations
on this entry when the selected operation supports them. This is required for
targets outside your home directory, such as system configuration files.

```yaml
sudo: true
```

!!! warning
    Only set `sudo: true` when the target path genuinely requires elevated privileges (e.g., `/etc/` paths). Using sudo unnecessarily may create files owned by root in unexpected locations.

For copy-template entries, elevated writes are supported only on a Linux runtime.
Windows uses native filesystem operations, and a sudo-enabled copy-template entry
on another runtime host fails explicitly. Status and diff inspection never request
sudo reads.

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

With `method: copy`, every `tidydots restore` compares the target file with the selected source:

- Ordinary files: if the contents differ, the target is overwritten with the current repo content.
- Template files selected with their `.tmpl` suffix: the target is compared with the previous pure render stored in `.tidydots.db`. User edits are retained where the template did not change, while template changes are applied where they do not conflict. The source hash covers source bytes only; changes to template context such as hostname or environment do not trigger this fast path, so use `--force-render` when those values change.
- If the contents already match and the target's required type and ownership also match, tidydots makes no changes (a no-op). Matching content alone may still require type or ownership repair; existing regular-file modes are retained.

For ordinary copies, editing the file in the dotfiles repo and re-running `tidydots restore` is how changes reach the target. For copy-mode templates, the `.tmpl` source is still the source of truth for generated content, while the target is the current merge input. A successful restore stores the new pure render, not the target's user edits.

!!! warning
    A first copy-template deployment that would replace an existing target saves an exclusive recovery copy beside it as `<target>.tidydots.bak`, unless `--force-render` explicitly bypasses the no-history backup. If that path is already occupied, restore refuses to overwrite either file. Legacy targets named with the `.tmpl` suffix are not removed automatically; rename or remove them explicitly after reviewing the migration.
    Status treats a recovery path that is not a regular file, including a symlink, as unavailable. The legacy `.tmpl` target path remains permitted and is not removed implicitly.

For copy-template idempotency, matching content alone is not sufficient for a
no-op: the target must also be the expected regular-file type and have the
required ownership. Existing regular target modes are retained. New
native files use the source template's permission bits; new files and symlink
replacements for `sudo: true` on Linux use restrictive `0600` permissions.
Recovery backups and conflict artifacts also use `0600`.

### Copy-mode template example

```yaml
entries:
  - name: snapper-config
    method: copy
    sudo: true
    backup: ./snapper
    files: [root.tmpl]
    targets:
      linux: /etc/snapper/configs
```

The selected `root.tmpl` renders to `/etc/snapper/configs/root`. It does not create
the symlink-mode `root` alias or `root.tmpl.rendered` cache in the repository.
On later restores, edits in the target are merged with the new render using the
SQLite history. A conflict deploys the pure template output and keeps the full
recovery content in `root.tmpl.conflict`; a later conflict-free restore removes
that stale artifact. `--force-render` skips the merge and overwrites the target
with the fresh render.

For example, the tested hostname-specific source:

```text
{{ if eq .Hostname "omarchbook" }}no/1{{ else }}yes/7{{ end }}
```

renders `no/1` on `omarchbook` and `yes/7` on `desktop`.

### Migrating from Symlink to Copy

If the target currently exists as a symlink (for example, the entry was previously deployed with `method: symlink`, or adopted), switching the entry to `method: copy` and re-running `tidydots restore` removes the supported migration symlink and replaces it with a real file copied or rendered from the repo. The replacement uses the source mode for native writes and `0600` for sudo-enabled Linux writes. This makes symlink-to-copy migration safe without any manual cleanup.

### Limitations

- **Files only** — `method: copy` requires an explicit, non-empty `files:` list. Whole-folder copying (`files: []`) is not supported and is rejected during config validation.
- **Template selections are rendered** — A selected `config.toml.tmpl` deploys as the real file `config.toml` and uses the copy-template merge history. This replaces the previous literal-copy behavior deliberately; ordinary non-template copy entries retain their overwrite-on-drift behavior. See [Templates](templates.md#copy-mode-template-deployment).

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

Config entries can contain `.tmpl` template files in their backup directory. During
symlink-mode restore, these files are rendered using the template engine, and the
output is written as `.tmpl.rendered` sibling files. The symlink then points to
the rendered output. In copy mode, selected templates render directly to the
suffix-free target; no repository alias or `.tmpl.rendered` cache is created.

For example, if your backup directory contains `alacritty.toml.tmpl`:

1. tidydots renders the template to `alacritty.toml.tmpl.rendered`
2. A relative symlink `alacritty.toml` is created pointing to `alacritty.toml.tmpl.rendered`
3. The folder-level symlink from the target path points to the backup directory as usual

For an explicit `files:` list, name the source with its `.tmpl` suffix (for
example, `.gitconfig.tmpl`). Only those selected sources render; a suffix-free
entry such as `.gitconfig` remains an ordinary file selection and does not
implicitly discover the template. This selection rule is the same for copy
entries, whose selected templates now render directly into real target files.

During backup, selected `.tmpl` sources are skipped in both deployment methods;
the live generated target must never replace the template source in the repo.

See [Templates](templates.md) for the full template system documentation.
