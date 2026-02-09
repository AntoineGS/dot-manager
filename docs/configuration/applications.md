# Applications

An **Application** is the top-level grouping unit in tidydots. Each application bundles related config entries and an optional package definition under a single name with optional conditional inclusion.

## Schema Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique application identifier |
| `description` | string | no | Human-readable description |
| `when` | string | no | Go template expression for conditional inclusion |
| `entries` | []SubEntry | no | Configuration entries (omit for package-only apps) |
| `package` | EntryPackage | no | App-level package definition for installation |

### Minimal Example

```yaml
applications:
  - name: "nvim"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
```

### Full Example

```yaml
applications:
  - name: "nvim"
    description: "Neovim text editor"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"
      - name: "nvim-spell"
        backup: "./nvim-spell"
        files:
          - "en.utf-8.add"
        targets:
          linux: "~/.local/share/nvim/site/spell"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        brew: "neovim"
```

## When Expressions

The `when` field controls whether an application is included based on the current platform. It uses Go `text/template` syntax and must evaluate to exactly the string `"true"` for the application to be included.

### Template Context

Templates have access to the following context variables:

| Variable | Type | Description | Example values |
|----------|------|-------------|----------------|
| `.OS` | string | Operating system | `"linux"`, `"windows"` |
| `.Distro` | string | Linux distribution ID | `"arch"`, `"ubuntu"`, `"fedora"` |
| `.Hostname` | string | Machine hostname | `"desktop"`, `"laptop"` |
| `.User` | string | Current username | `"alice"` |
| `.Env` | map[string]string | Environment variables | Access via `index .Env "HOME"` |

### Expression Examples

**Match a specific OS:**

```yaml
when: '{{ eq .OS "linux" }}'
```

**Match multiple conditions (AND):**

```yaml
when: '{{ and (eq .OS "linux") (eq .Distro "arch") }}'
```

**Match either condition (OR):**

```yaml
when: '{{ or (eq .OS "linux") (eq .OS "windows") }}'
```

**Match a specific hostname:**

```yaml
when: '{{ eq .Hostname "work-laptop" }}'
```

**Negate a condition:**

```yaml
when: '{{ ne .OS "windows" }}'
```

**Check an environment variable:**

```yaml
when: '{{ eq (index .Env "DISPLAY") ":0" }}'
```

**Complex expression (Arch Linux on a specific machine):**

```yaml
when: '{{ and (eq .Distro "arch") (eq .Hostname "desktop") }}'
```

### Evaluation Rules

- **Empty or missing `when`**: The application is always included.
- **Result must be exactly `"true"`**: Any other string (including `"false"`, `"1"`, or empty) means the application is excluded.
- **Template errors**: If the template fails to render (e.g., syntax error), the application is excluded.
- **Whitespace**: Leading and trailing whitespace in the result is trimmed before comparison.

!!! warning
    The `when` expression is evaluated as a Go template. Make sure to quote the entire value in YAML to avoid parsing issues, especially when using `{{ }}` delimiters.

## Entries

The `entries` field is an array of [SubEntry](configs.md) objects. Each entry defines a config symlink managed by tidydots. An application must have at least one entry.

```yaml
entries:
  - name: "main-config"
    backup: "./nvim"
    targets:
      linux: "~/.config/nvim"

  - name: "snippets"
    backup: "./nvim-snippets"
    files:
      - "go.json"
      - "python.json"
    targets:
      linux: "~/.config/nvim/snippets"
```

See the [Configs](configs.md) reference for the full SubEntry schema.

## Package

The `package` field defines how to install the application. It is an [EntryPackage](packages.md) object with support for system package managers, git repositories, custom commands, and URL downloads.

```yaml
package:
  managers:
    pacman: "neovim"
    apt: "neovim"
    brew: "neovim"
```

!!! note
    The field name is `package` (singular), not `packages`. It is defined at the application level, not on individual entries.

See the [Packages](packages.md) reference for all package types and installation methods.

## Sudo Behavior

!!! info
    The `sudo` flag is available on individual [SubEntry](configs.md) config entries, not on the Application itself. If you need elevated privileges for all entries in an application, set `sudo: true` on each entry individually.

When `sudo: true` is set on a config entry, tidydots uses elevated privileges for symlink operations on that entry's target path. This is needed for system-level configurations like files under `/etc/`.

```yaml
applications:
  - name: "system-config"
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        sudo: true
        backup: "./system/pacman"
        targets:
          linux: "/etc/pacman.conf"
      - name: "hosts"
        sudo: true
        backup: "./system/hosts"
        targets:
          linux: "/etc/hosts"
```

## Tips

!!! tip "Organizing Applications"
    Group related config files under a single application. For example, put your shell's `.zshrc`, `.zprofile`, and `.zshenv` as separate entries under one `zsh` application rather than creating three separate applications.

!!! tip "Naming Conventions"
    Use lowercase, descriptive names for applications (e.g., `nvim`, `zsh`, `git`). Entry names within an application can be more specific (e.g., `nvim-config`, `nvim-snippets`).
