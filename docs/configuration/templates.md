# Templates

tidydots includes a template engine based on Go's `text/template` that lets you generate machine-specific configurations from a single source file. Templates are processed during `tidydots restore` and support a 3-way merge system that preserves your manual edits across re-renders.

## File Naming Convention

Template files use the `.tmpl` suffix. During restore, tidydots generates sibling files:

| File | Description |
|------|-------------|
| `config.toml.tmpl` | Template source (you write this, committed to git) |
| `config.toml.tmpl.rendered` | Rendered output (generated, gitignored) |
| `config.toml.tmpl.conflict` | Conflict markers from merge (generated, gitignored) |
| `config.toml` | Relative symlink pointing to `config.toml.tmpl.rendered` |

The symlink target on your system (e.g., `~/.config/alacritty/alacritty.toml`) points into your backup directory, where `alacritty.toml` is itself a relative symlink to `alacritty.toml.tmpl.rendered`.

## Template Context Variables

Templates have access to the following context struct:

| Variable | Type | Description | Example |
|----------|------|-------------|---------|
| `.OS` | string | Operating system | `"linux"`, `"windows"` |
| `.Distro` | string | Linux distribution ID | `"arch"`, `"ubuntu"`, `"fedora"` |
| `.Hostname` | string | Machine hostname | `"desktop"`, `"work-laptop"` |
| `.User` | string | Current username | `"alice"` |
| `.HasDisplay` | bool | Whether a display server is available | `true` (X11/Wayland/Windows), `false` (headless) |
| `.Env` | map[string]string | All environment variables | See below |

### Accessing Environment Variables

Use the `index` function to read environment variables:

```
{{ index .Env "HOME" }}
{{ index .Env "EDITOR" }}
{{ index .Env "XDG_CONFIG_HOME" }}
```

The `.Env` map contains all process environment variables plus any platform-specific overrides.

## Template Functions

tidydots uses [sprout](https://github.com/go-sprout/sprout) to provide a rich set of template functions. The following registries are available:

| Registry | Examples |
|----------|----------|
| **std** | `default`, `empty`, `ternary`, `fail` |
| **strings** | `trim`, `upper`, `lower`, `replace`, `contains`, `hasPrefix`, `hasSuffix` |
| **numeric** | `add`, `sub`, `mul`, `div`, `mod`, `max`, `min` |
| **conversion** | `toString`, `toInt`, `toFloat64`, `toBool` |
| **maps** | `dict`, `get`, `set`, `hasKey`, `keys`, `values` |
| **slices** | `list`, `first`, `last`, `append`, `has`, `uniq` |
| **regexp** | `regexMatch`, `regexFind`, `regexReplaceAll` |

For the full function reference, see the [sprout documentation](https://github.com/go-sprout/sprout).

### Template Examples

**Conditional block based on OS:**

```
{{ if eq .OS "linux" }}
font_size = 12
{{ else }}
font_size = 14
{{ end }}
```

**Host-specific values:**

```
{{ if eq .Hostname "desktop" }}
monitor_count = 3
dpi = 96
{{ else if eq .Hostname "laptop" }}
monitor_count = 1
dpi = 144
{{ else }}
monitor_count = 1
dpi = 96
{{ end }}
```

**Using sprout functions:**

```
# User: {{ upper .User }}
# Config generated for {{ .Hostname | title }}
```

**Default values:**

```
editor = "{{ default "vim" (index .Env "EDITOR") }}"
```

**GUI vs headless conditional:**

```
{{ if .HasDisplay }}
# GUI applications
exec alacritty
{{ else }}
# Terminal-only setup
export TERM=xterm-256color
{{ end }}
```

This is also useful in `when` expressions to conditionally include entire applications:

```yaml
applications:
  - name: "alacritty"
    when: '{{ .HasDisplay }}'

  - name: "tmux-heavy-config"
    when: '{{ not .HasDisplay }}'
```

On Linux, `.HasDisplay` is `true` when `DISPLAY` (X11) or `WAYLAND_DISPLAY` (Wayland) is set. On Windows, it is always `true`.

## How Template Restore Works

When `tidydots restore` encounters a `.tmpl` file in a backup directory:

1. **Read** the template source file (`config.toml.tmpl`)
2. **Render** it using the template engine with the current platform context
3. **Merge** the render output with any existing rendered file (see 3-Way Merge below)
4. **Write** the result to `config.toml.tmpl.rendered`
5. **Create a relative symlink** `config.toml` pointing to `config.toml.tmpl.rendered`
6. **Store** the pure render output in the SQLite state database (`.tidydots.db`)

Non-template files in the same backup directory get normal symlinks as usual.

## 3-Way Merge

The 3-way merge system preserves manual edits you make to rendered files. It uses three inputs:

| Input | Source | Description |
|-------|--------|-------------|
| **base** | `.tidydots.db` (SQLite) | The previous pure render output stored in the database |
| **theirs** | `.tmpl.rendered` on disk | The current rendered file, which may contain your manual edits |
| **ours** | New template render | The freshly rendered template output |

### Merge Logic

The merge follows these fast paths first:

- **base == theirs**: No user edits were made. Use the new render (`ours`).
- **base == ours**: Template did not change. Keep user edits (`theirs`).
- **theirs == ours**: Both arrive at the same result. Use the new render (`ours`).

If none of the fast paths apply, the merge proceeds line-by-line:

- **Only template changed** (base line == their line, our line differs): Use the new template line.
- **Only user changed** (base line == our line, their line differs): Keep the user edit.
- **Both changed the same way** (their line == our line): Use either (they are identical).
- **Both changed differently**: This is a **conflict**.

### Conflict Markers

When a conflict is detected, the merged output contains markers:

```
<<<<<<< user-edits
font_size = 16
=======
font_size = 12
>>>>>>> template
```

A separate `.tmpl.conflict` file is also written with the full merged content including conflict markers. The `.tmpl.rendered` file itself receives the merged content (including any conflict markers), so you can resolve conflicts by editing the rendered file directly.

!!! tip "Resolving Conflicts"
    Edit the `.tmpl.rendered` file to resolve conflicts, then remove the conflict markers. Your edits will be preserved on the next render through the 3-way merge. Alternatively, if you want to discard your edits entirely, use `--force-render`.

### Skip Optimization

If the template source has not changed (detected via SHA-256 hash comparison against the database), and the rendered file already exists on disk, tidydots skips re-rendering entirely and just ensures the relative symlink is correct.

## Force Render

The `--force-render` flag bypasses the 3-way merge and overwrites the rendered file with the new template output, discarding any user edits.

```bash
tidydots restore --force-render
```

!!! warning
    Using `--force-render` permanently discards any manual edits to `.tmpl.rendered` files. There is no undo.

## Viewing Template Diffs

If you manually edit a `.tmpl.rendered` file, the TUI shows the entry with a **Modified** status (blue). You can view a diff of your edits and update the template source directly:

1. Open the TUI: `tidydots`
2. Navigate to the modified entry and press `i`
3. Your editor opens with the diff (read-only) alongside the `.tmpl` source file
4. Update the template to incorporate your edits, save, and quit

This workflow makes it easy to experiment with rendered config files and then backport successful changes into the template source. See [Interactive TUI - Template diff & edit](../guides/interactive-tui.md#template-diff--edit) for full details.

!!! info "Modified vs Outdated"
    **Modified** means the rendered file on disk differs from the pure render stored in the database -- you edited the output. **Outdated** means the template source (`.tmpl`) has changed since the last render -- the template needs re-rendering.

## SQLite State Database

tidydots stores template render history in a SQLite database at `.tidydots.db` in the root of your dotfiles repository (next to `tidydots.yaml`).

The database stores:

| Field | Description |
|-------|-------------|
| `template_path` | Relative path of the `.tmpl` file |
| `pure_render` | The unmerged template output (used as `base` in future merges) |
| `template_hash` | SHA-256 hash of the template source (for skip optimization) |
| `rendered_at` | Timestamp of the render |
| `platform_os` | OS at render time |
| `platform_host` | Hostname at render time |

The database uses WAL mode for safe concurrent access and maintains a history of renders per template.

## Recommended .gitignore

Add these patterns to the `.gitignore` in your dotfiles repository:

```gitignore
# tidydots generated files
*.tmpl.rendered
*.tmpl.conflict
.tidydots.db
```

These files are machine-specific and should not be committed to your dotfiles repository.

## Path Templating

Template expressions are also supported in `targets` and `backup` path fields in your `tidydots.yaml`. Any path containing `{{ }}` delimiters is rendered through the template engine before `~` and environment variable expansion.

```yaml
entries:
  - name: "nvim-config"
    backup: "./nvim"
    targets:
      linux: "~/.config/{{ .Hostname }}/nvim"
```

Paths without `{{ }}` delimiters fall through to standard path expansion, maintaining full backward compatibility.

### Path Template Examples

**Host-specific target directory:**

```yaml
targets:
  linux: "~/.config/{{ .Hostname }}/alacritty"
```

**User-specific path:**

```yaml
targets:
  linux: "/home/{{ .User }}/.config/nvim"
```

**Distro-specific path:**

```yaml
backup: "./{{ .Distro }}/systemd"
```

## Real-World Example: Host-Specific Terminal Config

Here is a complete example showing how to use templates for a terminal emulator configuration that varies by machine.

**tidydots.yaml:**

```yaml
version: 3

applications:
  - name: "alacritty"
    description: "GPU-accelerated terminal"
    when: '{{ ne .OS "windows" }}'
    entries:
      - name: "alacritty-config"
        backup: "./alacritty"
        targets:
          linux: "~/.config/alacritty"
    package:
      managers:
        pacman: "alacritty"
        apt: "alacritty"
        brew: "alacritty"
```

**./alacritty/alacritty.toml.tmpl:**

```toml
[window]
{{ if eq .Hostname "desktop" }}
# Desktop: large monitor, no decorations
dimensions = { columns = 160, lines = 50 }
decorations = "None"
{{ else if eq .Hostname "laptop" }}
# Laptop: smaller screen, keep decorations
dimensions = { columns = 120, lines = 35 }
decorations = "Full"
{{ else }}
# Default
dimensions = { columns = 120, lines = 40 }
decorations = "Full"
{{ end }}

[font]
{{ if eq .Hostname "laptop" }}
size = 14.0
{{ else }}
size = 12.0
{{ end }}
normal = { family = "JetBrains Mono", style = "Regular" }

[env]
TERM = "xterm-256color"
```

After running `tidydots restore` on the `desktop` machine, the backup directory contains:

```
alacritty/
  alacritty.toml.tmpl          # Template source (committed)
  alacritty.toml.tmpl.rendered # Rendered output (gitignored)
  alacritty.toml               # Symlink -> alacritty.toml.tmpl.rendered
```

And `~/.config/alacritty` is a symlink to the backup directory, so your terminal reads the rendered configuration seamlessly.
