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

The `.tmpl.rendered` cache and relative alias are used by symlink-mode entries.
For `method: copy`, a selected template is rendered directly to the real target
(`config.toml`); the repository cache and alias are not created. A first
deployment that replaces an existing target may create the exclusive target-side
recovery file `config.toml.tidydots.bak`; `--force-render` explicitly bypasses
that no-history backup.

When a symlink-mode template needs the suffix-free repository alias and a
regular file already occupies that alias, tidydots first preserves that file as
an exclusive, mode-`0600` `<alias>.tidydots.bak`. An occupied recovery path is
never replaced: restore stops and leaves the existing alias and live output
unchanged. `--force-render` does not bypass this preservation rule. Folder
entries also refuse a merge when a real target-side template alias is already
occupied, rather than removing it while converting the folder to template
links.

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
| `.IsWSL` | bool | Whether running inside Windows Subsystem for Linux | `true` (WSL1/WSL2), `false` (native) |
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
| **regex** | `regexMatch`, `regexFind`, `regexReplaceAll` |

For the full function reference, see the [sprout documentation](https://github.com/go-sprout/sprout).

### Sprout 1.1 argument order

tidydots uses Sprout 1.1's pipeline-oriented function signatures. Templates
using Sprig argument order, including templates Sprout 1.0 accepted with
warnings, must update these map and slice functions:

`get`, `set`, `unset`, `hasKey`, `pick`, `omit`, `append`, `prepend`, `slice`, and `without`.

Move the map or list to the end of a direct call, or use it as pipeline input:

```text
# Old
{{ append $items "value" }}

# Sprout 1.1
{{ $items | append "value" }}
```

The `regexFindAll`, `regexSplit`, `regexReplaceAll`, and
`regexReplaceAllLiteral` functions likewise take the input string last. Pipeline
forms are the clearest migration:

```text
{{ $value | regexFindAll "pattern" -1 }}
{{ $value | regexSplit "pattern" -1 }}
{{ $value | regexReplaceAll "pattern" "replacement" }}
{{ $value | regexReplaceAllLiteral "pattern" "replacement" }}
```

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

**WSL-aware conditional:**

```
{{ if .IsWSL }}
# WSL-specific settings (e.g., use Windows browser)
export BROWSER="wslview"
{{ else }}
export BROWSER="firefox"
{{ end }}
```

This is also useful in `when` expressions to exclude applications that don't work in WSL:

```yaml
applications:
  - name: "alacritty"
    when: '{{ and .HasDisplay (not .IsWSL) }}'

  - name: "wsl-utilities"
    when: '{{ .IsWSL }}'
```

`.IsWSL` is detected by checking `/proc/version` for the `microsoft` or `WSL` identifier, which works on both WSL1 and WSL2.

## How Template Restore Works

When symlink-mode `tidydots restore` encounters a `.tmpl` file in a backup directory:

1. **Read** the template source file (`config.toml.tmpl`)
2. **Render** it using the template engine with the current platform context
3. **Merge** the render output with any existing rendered file (see 3-Way Merge below)
4. **Write** the result to `config.toml.tmpl.rendered`
5. **Create a relative symlink** `config.toml` pointing to `config.toml.tmpl.rendered`
6. **Store** the pure render output in the SQLite state database (`.tidydots.db`)

If tidydots cannot safely read the current rendered output, or cannot preserve
required recovery data, it fails closed: it does not replace the rendered
output or its alias. Orphan rendered-output backups use an exclusive
`.tmpl.rendered.bak` path with mode `0600`; an occupied path must be resolved
or moved by you before restore can continue.

Non-template files in the same backup directory get normal symlinks as usual.

#### Copy-mode template deployment

With `method: copy` and an explicit `.tmpl` selection, restore follows the same
rendering and history policy but deploys the result as a real target file:

1. Read and hash the `.tmpl` source and read the current target.
2. Look up the normalized source path in `.tidydots.db`.
3. If the source hash is unchanged and the target exists, retain the target
   content without inventing a new base (while repairing a migration symlink or
   sudo ownership when necessary; existing regular-file modes are retained).
4. Otherwise render the source and merge `base` (the prior pure render),
   `theirs` (the current target), and `ours` (the new render).
5. Deploy the merged result. On conflict, deploy the pure render and write the
   full recovery content to the `.tmpl.conflict` artifact before changing the
   target.
6. Save the pure render and source hash only after deployment and artifact work
   succeeds.

The template source remains the source of truth for generated lines, while
unchanged user edits in the target are preserved by the three-way merge. An
ordinary copy entry still overwrites target drift rather than merging it. The
`--force-render` flag bypasses the merge and overwrites the target with the fresh
render. Without an initialized state database, copy templates use no-history
semantics, save an exclusive `<target>.tidydots.bak` before replacing an
existing target, and do not create a second cache or state mechanism; the
explicit `--force-render` path bypasses that backup. Matching content alone is
not a no-op when the required target type or sudo ownership needs repair.
Existing regular modes are retained, native-created files use source
permissions, sudo-enabled Linux new or symlink-replacement files use `0600`,
and conflict/recovery artifacts use `0600`.

### Explicit file selections

Entries with a `files` list render only the selected `.tmpl` files. Name the
**source** in `files`, including its `.tmpl` suffix. A suffix-free name is an
ordinary literal selection; it does not implicitly discover a similarly named
template. Unlisted templates are left untouched.

For example, this entry deploys a single rendered Git configuration file:

```yaml
entries:
  - name: "gitconfig"
    backup: "./git"
    files:
      - ".gitconfig.tmpl"
    targets:
      linux: "~"
```

After restore, the layout is:

```text
git/.gitconfig.tmpl                  template source
git/.gitconfig.tmpl.rendered         generated content
git/.gitconfig -> .gitconfig.tmpl.rendered
~/.gitconfig -> <repo>/git/.gitconfig
```

The final `.tmpl` suffix is stripped once when choosing the deployed name, so
`config.tmpl.tmpl` deploys as the literal target name `config.tmpl` without
recursively rendering the output. For nested sources, the parent directory is
preserved: `nested/config.tmpl.tmpl` deploys as `nested/config.tmpl`. The
relative alias in the backup directory keeps the source template and the
generated content distinct while the target uses the expected suffix-free name.

Template status and rendered-file diff discovery also use only the listed
templates. For symlink entries, folder entries continue to discover templates
recursively, and a missing rendered output is reported as needing a re-render
rather than healthy. Copy entries inspect their live suffix-free targets instead
of a rendered repository file.

Entries using `method: copy` deliberately render selected `.tmpl` files into
suffix-free real targets. This is a deliberate compatibility change from the
former literal-copy behavior. During backup, selected `.tmpl` sources are
skipped in both symlink and copy modes so an installed file cannot overwrite the
template source in the repository.

#### Restore safety for selected templates

Before a selected-template restore mutates the entry, tidydots validates all
selected template sources and template-derived paths together. A selected
source must be a regular, renderable file within the entry; selected template
and literal paths may not collide; generated output aliases may not conflict
with sources or each other; concrete targets may not overlap the repository,
state database, or generated artifacts; and source/target aliases and unsafe
symlink parents are rejected. A target root above a nested repository is valid
when its concrete deployed files are outside the repository. Literal selections
still go through their normal restore checks as they are deployed, so a missing
literal source or disallowed target can fail later.

Run the narrowest restore as a dry run first to inspect the intended work:

```bash
tidydots restore <app> gitconfig -n
```

#### Backup behavior

For entries with explicit selections, `tidydots backup` deliberately skips
selected `.tmpl` paths in both symlink and copy modes. In symlink mode, the
deployed suffix-free target is a link through the backup alias; in copy mode, it
is a generated real file. Neither may replace the template source in the repo.
Ordinary non-template copy files retain their normal literal backup behavior.

## 3-Way Merge

The 3-way merge system preserves manual edits you make to rendered files. For
symlink-mode templates it uses these inputs:

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

If none of the fast paths apply, tidydots compares each side's edits against
the previous pure render. Independent insertions, deletions, and changes are
normally retained together. Changes that overlap the same base region produce
a conflict, as do ambiguous end-of-file boundary changes that could otherwise
join content incorrectly. For unusually divergent files, tidydots may choose a
conservative conflict instead of attempting an unsafe automatic merge.

This improves preservation of unrelated edits, but a three-way merge cannot
guarantee a clean result for every pair of changes. Review conflict artifacts
before incorporating edits into the template source.

### Conflict Markers

When a conflict is detected, the merged output contains markers. In symlink
mode, tidydots writes those markers to `.tmpl.conflict` and keeps the valid
pure render in `.tmpl.rendered`:

```
<<<<<<< user-edits
font_size = 16
=======
font_size = 12
>>>>>>> template
```

For symlink mode, tidydots writes the full merged content, including conflict
markers, to `.tmpl.conflict`. It overwrites `.tmpl.rendered` with the fresh
template output so the configuration consumed by applications remains valid.
For copy mode, the same recovery content is saved beside the source while the
suffix-free target receives the pure render; copy mode has no `.tmpl.rendered`
cache. Any manual edits that could not be merged remain available in the
conflict file. Both conflict artifacts use restrictive `0600` permissions. In
symlink mode, the artifact is exclusive: an existing `.tmpl.conflict` blocks a
new conflicting restore until you resolve or move it. In copy mode, a new
conflict refreshes the existing conflict artifact while the pure render is
deployed.

!!! tip "Resolving Conflicts"
    Inspect `.tmpl.conflict`, then port the desired user edits into the `.tmpl`
    source and run restore again. A later successful merge removes the stale
    conflict file automatically.

### Skip Optimization

If the template source has not changed (detected via SHA-256 hash comparison against the database), and the rendered file already exists on disk, symlink mode skips re-rendering entirely and just ensures the relative symlink is correct. Copy mode uses the same source-byte hash fast path when its suffix-free target exists, preserving target edits without creating a repository cache. The hash does not include hostname, OS, user, or environment context; use `--force-render` to refresh output when context changes.

### Render History and Upgrades

Symlink render history is isolated by the template source's path relative to the
repository root, plus OS and hostname. For example, `git/config.tmpl` and
`ghostty/config.tmpl` have separate baselines even though each entry selects
`config.tmpl`. Symlink deployments of the same source share one `.tmpl.rendered`
output and therefore share its history, regardless of their entry or target.
Source keys have an explicit `./` marker (for example, `./git/config.tmpl`) to
distinguish them from older entry-relative keys.

Copy history additionally includes the **resolved suffix-free target file**.
Its key is an unambiguous quoted pair, for example
`copy:"./shared/config.tmpl":"/home/user/target-a/config"`. The target is the
expanded, cleaned absolute deployment path with forward slashes; target
symlinks are not followed when constructing the key. Different backup-root
spellings or entry names do not split history for the same source/target pair.
Different copy targets do not share history with each other or with the source's
symlink output. Restoring one deployment cannot mark another deployment's stale
output as current. Restore, preflight, status, and diff use these same identities.

Older databases may contain interleaved histories for identically named
templates. On upgrade, tidydots checks the legacy history for the **current
source's SHA-256 hash on this OS and hostname**, not just the latest record.
It reuses a baseline only when all matching records agree on the pure rendered
content. A normal restore copies that verified baseline into the scoped history,
including when the source is unchanged, while preserving the existing rendered
file or copy target and its local edits. The old records are retained; unrelated
history is never merged or moved. Copies look up the original entry-relative
legacy key, not another deployment's copy key. Status, diff, and dry runs do not
migrate state.

Source-only scoped records (`./shared/config.tmpl`) from an earlier candidate
cannot establish which copy target was updated, even when the source hash
matches. They are not used as copy baselines. If no verified original legacy
baseline exists, their presence blocks a normal restore of an existing copy
target rather than silently treating it as a first-render orphan. This also
applies when switching a source-only symlink deployment to copy mode. If only
source-only scoped history exists and the live target is **byte-for-byte equal
to a fresh render**, restore can safely initialize a new copy baseline without
reusing any historical record. This permits unedited symlink-to-copy migration.
Differing outputs remain blocked. This exception does not relax the original
entry-relative legacy hash/consistency checks. Missing targets can be rendered
normally; an explicit `--force-render` can initialize a new copy baseline after
local edits have been preserved separately.

If legacy history exists but cannot supply a trustworthy baseline, status reports
the template as outdated and diff does not invent a baseline from another
application. Normal restore refuses to overwrite an existing output and reports
that it was left unchanged. This can happen when the source changed before its
first restore with the fixed version, or when matching historical renders differ.
Preserve any local edits in the template source or a separate recovery file
before deliberately using `--force-render`. Missing outputs can be rendered
normally. Templates without any legacy history retain normal first-render
behavior, including the existing orphan-backup policy.

## Force Render

The `--force-render` flag bypasses the 3-way merge and overwrites the rendered
file, or the suffix-free copy target, with the new template output, discarding
any user edits. It also skips the first-render target backup for copy templates.
It never bypasses repository-alias preservation. Symlink-mode Force Render
leaves existing conflict artifacts untouched. Copy-mode Force Render removes
stale conflict artifacts, as a conflict-free copy render does. Preserve recovery
content you still need before forcing a restore.

In the interactive TUI, this behavior is labeled **Force Restore** and is
available with `R`. The normal `r` Restore action preserves rendered-template
edits through the 3-way merge. Force Restore uses the same target scope as
Restore, always asks for confirmation, and discards manual edits to rendered
template files.

```bash
tidydots restore --force-render
```

!!! warning
    Using `--force-render` permanently discards any manual edits to `.tmpl.rendered` files or copy-mode targets. There is no undo.

## Live Preview

The `tidydots preview` command lets you iterate on templates with instant feedback. It watches `.tmpl` files for changes and re-renders them on every save, so you can see the output update in real time.

### Usage

```bash
# Watch a single template
tidydots preview ./alacritty/alacritty.toml.tmpl

# Watch all templates in a directory
tidydots preview ./alacritty
```

### Recommended Workflow

1. Open a terminal and start the preview watcher:

    ```bash
    tidydots preview ./alacritty/alacritty.toml.tmpl
    ```

2. In your editor, open the `.tmpl` source and the `.tmpl.rendered` output side by side.
3. Edit and save the `.tmpl` file. The rendered output updates automatically.
4. If you introduce a syntax error, the watcher prints the error but preserves the last good render -- your `.tmpl.rendered` file is never left in a broken state.
5. Press `Ctrl+C` when you are done.

### How It Differs from Restore

| | `tidydots preview` | `tidydots restore` |
|---|---|---|
| **Purpose** | Author and debug templates | Deploy configs |
| **Watches for changes** | Yes (continuous) | No (one-shot) |
| **Creates symlinks** | No | Yes |
| **Updates state DB** | No | Yes |
| **3-way merge** | No | Yes |

`preview` is a lightweight authoring tool -- it renders templates without touching symlinks or the SQLite state database. Once you are happy with the template, run `tidydots restore` to deploy it.

### Reverse Editing (Neovim Plugin)

When using the [tidydots.nvim](https://github.com/AntoineGS/tidydots.nvim) plugin, the rendered preview buffer supports **reverse editing** -- you can edit the rendered output directly and have changes propagate back to the template source.

#### Configuration

Reverse editing is enabled by default. To disable it:

```lua
require("tidydots").setup({
  reverse_edit = false,
})
```

#### Editable vs Read-Only Lines

Not all lines in the rendered buffer are editable:

| Line Type | Editable | Example |
|-----------|----------|---------|
| **Text** | Yes | `font_size = 12` (plain text, no template syntax) |
| **Expression** | No | Lines originating from `{{ .Hostname }}` or `{{ upper .User }}` |
| **Directive** | N/A | Lines from `{{ if }}`, `{{ end }}`, etc. (not visible in rendered output) |

Read-only lines are highlighted with the `TidydotsReadOnlyLine` highlight group (linked to `Comment` by default). Attempting to edit a read-only line reverts the change immediately.

#### Text Edits

When you modify a text line in the rendered buffer, the change is written directly to the corresponding line in the template source buffer. The mapping uses the source map that tidydots emits during rendering to identify which template line produced each rendered line.

#### Structural Edits

Adding or deleting lines in the rendered buffer triggers a round-trip through the CLI:

1. The plugin detects the structural change (line count differs from last known state).
2. It sends a `rendered_edit` message to the running `tidydots preview` process via stdin.
3. The CLI applies the edit to the template source using the reverse source map.
4. The CLI responds with a `template_update` containing the new template content and a cursor hint.
5. The plugin updates the template buffer and cursor position.

Only text lines can be deleted through the rendered buffer -- expression and directive lines are protected.

#### Cursor Sync

When you move the cursor in the rendered buffer, the template buffer cursor follows to the corresponding source line. This works in both directions: the existing forward sync (template → rendered) and the reverse sync (rendered → template) keep both buffers aligned.

## Viewing Template Diffs

If you manually edit a `.tmpl.rendered` file, the TUI shows the entry with a **Modified** status (blue). For entries with `files`, it considers only the explicitly selected `.tmpl` sources. You can view a diff of your edits and update the template source directly:

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
| `template_path` | Repository-relative source key with a `./` prefix and forward slashes; copy keys also include the resolved target |
| `pure_render` | The unmerged template output (used as `base` in future merges) |
| `template_hash` | SHA-256 hash of the template source (for skip optimization) |
| `rendered_at` | Timestamp of the render |
| `platform_os` | OS at render time |
| `platform_host` | Hostname at render time |

The database uses WAL mode for safe concurrent access and maintains a history of renders per template.

History is scoped by repository-relative source path, OS, and hostname, plus the
resolved target for copy deployments. Templates such as `Both/Git/config.tmpl`
and `Linux/ghostty/config.tmpl` have independent render histories even though
their filenames match. Status checks and 3-way merges use the same identities.

!!! warning "Upgrading from entry-relative render history"
    Older versions recorded paths relative to each entry's backup directory,
    which could mix up templates with the same name. A legacy baseline is reused
    only when records matching the current source hash, OS, and hostname agree
    on the pure render. Otherwise, existing output is left unchanged and normal
    restore stops so local edits can be preserved before using `--force-render`.

    When there is no scoped or legacy history, a normal restore backs up an
    existing `.tmpl.rendered` file to `.tmpl.rendered.bak` before a fresh render is
    written. Manual edits remain in that backup but are not automatically
    merged into the fresh output. Review those edits and incorporate anything
    you need into the template source. Preview the affected entry with
    `tidydots restore <app> <entry> -n` before restoring it, and do not use
    `--force-render` if you need the backup.

    Restore stops if the backup already exists or cannot be created. Preserve
    or reconcile an existing `.bak` outside that path before retrying; tidydots
    will not overwrite it. Keep these machine-specific backups out of Git.

## Recommended .gitignore

Add these patterns to the `.gitignore` in your dotfiles repository:

```gitignore
# tidydots generated files
*.tmpl.rendered
*.tmpl.rendered.bak
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

A malformed template, or a template or environment expansion that produces an
empty path, is an error rather than a literal or fallback deployment path. Use
an explicit `.` when you intentionally mean the repository root for `backup`.

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
