# Troubleshooting

Common issues and how to resolve them.

---

## Configuration not found / "config_dir not set"

**Symptom:** tidydots exits with an error about a missing configuration or `config_dir` not being set.

**Cause:** The app configuration at `~/.config/tidydots/config.yaml` does not exist or does not contain a valid `config_dir` value. This happens when you have not yet run `tidydots init` on this machine.

**Solution:**

Run `tidydots init` with the path to your dotfiles repository:

```bash
tidydots init ~/dotfiles
```

Alternatively, bypass the app config entirely by passing the `--dir` flag on every command:

```bash
tidydots restore -d ~/dotfiles
```

!!! tip
    You only need to run `tidydots init` once per machine. After that, all commands read the saved path automatically.

---

## Broken symlinks

**Symptom:** A program cannot find its configuration, or `ls -la` shows a symlink pointing to a path that no longer exists.

**Cause:** The backup source in your dotfiles repo was moved, renamed, or deleted after the symlink was created.

**Solution:**

1. Run `tidydots list` to see all configured paths and verify they are correct:

    ```bash
    tidydots list
    ```

2. If the backup path in `tidydots.yaml` is wrong, update it to match the actual location of the files in your repo.

3. Re-run `tidydots restore` to recreate the symlinks:

    ```bash
    tidydots restore
    ```

!!! note
    If the target already exists as a broken symlink, `restore` will replace it. If the target exists as a regular file or directory, you may need `--no-merge --force` to overwrite it.

---

## Template merge conflicts

**Symptom:** A `.tmpl.rendered` file contains conflict markers like:

```
<<<<<<< user-edits
your manual changes here
=======
new template output here
>>>>>>> template
```

**Cause:** You manually edited a `.tmpl.rendered` file, and then re-rendered the template. The 3-way merge detected that both sides changed the same lines and could not automatically resolve the difference.

**Solution:**

**Option 1: Resolve manually.** Open the `.tmpl.rendered` file, pick the correct version for each conflicting section, and remove the conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`).

**Option 2: Discard your edits.** If you want the pure template output without any manual changes, re-run restore with `--force-render`:

```bash
tidydots restore --force-render
```

!!! warning
    `--force-render` overwrites all rendered files with fresh template output. Any manual edits to `.tmpl.rendered` files will be lost.

**Option 3: Check the conflict file.** When a merge conflict occurs, tidydots also writes a `.tmpl.conflict` file alongside the rendered file. You can inspect it for additional context.

!!! tip
    To avoid merge conflicts in the future, prefer making changes in the `.tmpl` source file rather than editing the `.tmpl.rendered` output directly.

---

## Permission errors with sudo entries

**Symptom:** tidydots fails with "permission denied" when trying to create symlinks or copy files to system-level paths like `/etc/`.

**Cause:** The config entry targets a path that requires root privileges, and tidydots is running as a normal user. Entries with `sudo: true` in your `tidydots.yaml` need elevated privileges.

**Solution:**

Run tidydots with `sudo`:

```bash
sudo tidydots restore
```

If only some entries require sudo, you can also split your workflow:

1. Run `tidydots restore` as a normal user first (non-sudo entries will succeed, sudo entries will fail).
2. Run `sudo tidydots restore` to handle the remaining sudo entries.

!!! note
    When running with `sudo`, the home directory (`~`) may resolve to `/root` instead of your user home. Use `--dir` to explicitly specify the dotfiles path if needed:

    ```bash
    sudo tidydots restore -d /home/youruser/dotfiles
    ```

---

## Template rendering failures

**Symptom:** tidydots reports a template parsing or execution error during `restore`.

**Cause:** There is a syntax error in one of your `.tmpl` files. Common mistakes include:

- Unclosed template delimiters (`{{` without a matching `}}`)
- Undefined template variables or functions
- Mismatched quotes inside template expressions

**Solution:**

1. Preview the operation with dry-run and verbose output to identify the failing file:

    ```bash
    tidydots restore -n -v
    ```

2. Open the `.tmpl` file mentioned in the error and fix the syntax. Template files use Go `text/template` syntax. For example:

    ```
    # Correct
    {{ eq .OS "linux" }}
    {{ .Hostname }}
    {{ index .Env "HOME" }}

    # Wrong -- missing closing delimiter
    {{ eq .OS "linux"
    ```

3. Available context variables are:

    | Variable | Description | Example |
    |----------|-------------|---------|
    | `.OS` | Operating system | `"linux"`, `"windows"` |
    | `.Distro` | Linux distribution | `"arch"`, `"ubuntu"` |
    | `.Hostname` | Machine hostname | `"my-laptop"` |
    | `.User` | Current username | `"youruser"` |
    | `.Env` | Environment variables map | `{{ index .Env "HOME" }}` |

4. All [sprout](https://github.com/go-sprout/sprout) template functions are available (string manipulation, math, collections, and more).

!!! tip
    Template expressions are also supported in `targets` and `backup` paths. If a path contains `{{` and fails to render, the same debugging approach applies.

---

## Package installation failures

**Symptom:** `tidydots install` reports one or more packages as failed.

**Cause:** Several things can go wrong:

- The required package manager is not installed on the system.
- The package name is wrong for the selected manager.
- The manager needs elevated privileges (e.g., `apt` requires `sudo`).
- Network connectivity issues for managers that download packages.

**Solution:**

1. Check which package managers are available and which manager tidydots will use for each package:

    ```bash
    tidydots list-packages
    ```

    Packages marked with `✗` cannot be installed because no configured manager is available.

2. If the wrong manager is being selected, configure `default_manager` or `manager_priority` in your `tidydots.yaml`:

    ```yaml
    default_manager: "pacman"
    manager_priority:
      - "pacman"
      - "yay"
      - "paru"
    ```

3. Preview the installation to see exactly what commands will run:

    ```bash
    tidydots install -n -v
    ```

4. For managers that require sudo (like `apt` or `pacman`), run with elevated privileges:

    ```bash
    sudo tidydots install
    ```

---

## Git clone failures

**Symptom:** A git package entry fails during `tidydots install` with a clone or pull error.

**Cause:** Common reasons include:

- The repository URL is incorrect or the repository does not exist.
- Authentication is required (private repository) and credentials are not configured.
- The specified branch does not exist.
- Network connectivity issues.
- The target directory already exists but is not a git repository.

**Solution:**

1. Verify the URL manually:

    ```bash
    git ls-remote https://github.com/user/repo.git
    ```

2. If the repository is private, ensure your git credentials (SSH key or token) are configured.

3. If a specific branch is configured, verify it exists:

    ```bash
    git ls-remote --heads https://github.com/user/repo.git branch-name
    ```

4. If the target directory exists but is corrupted, remove it and let tidydots clone fresh:

    ```bash
    rm -rf ~/.local/share/some-plugin
    tidydots install
    ```

5. Check your `tidydots.yaml` git configuration:

    ```yaml
    applications:
      - name: "my-plugin"
        entries: []
        package:
          managers:
            git:
              url: "https://github.com/user/repo.git"
              branch: "main"
              targets:
                linux: "~/.local/share/my-plugin"
    ```

---

## Interactive mode requires a terminal

**Symptom:** Running `tidydots` (with no subcommand) or using the `-i` flag produces:

```
interactive mode requires a terminal; use subcommands (restore, backup, list) for non-interactive use
```

**Cause:** tidydots is not connected to a terminal. This happens when running from a script, a cron job, or piping output.

**Solution:** Use the explicit subcommands instead of the TUI:

```bash
tidydots restore
tidydots backup
tidydots install
```

These commands work without a terminal and can be used in scripts and automation.

---

## Debugging tips

When something is not working as expected, these flags and commands help narrow down the problem.

### Preview with dry-run

The `-n` flag shows what tidydots would do without making any changes. Use it before every destructive operation:

```bash
tidydots restore -n
tidydots install -n
tidydots backup -n
```

### Enable verbose output

The `-v` flag provides detailed information about each step:

```bash
tidydots restore -v
tidydots install -v
```

Combine with dry-run for the safest, most detailed preview:

```bash
tidydots restore -n -v
```

### Inspect your configuration

Use `list` and `list-packages` to verify that tidydots is reading your config correctly:

```bash
# Show all config entries and their target paths
tidydots list

# Show all packages and their install methods
tidydots list-packages
```

### Check the template state database

tidydots stores template render history in a SQLite database at `.tidydots.db` in your dotfiles repo root. If template merges are behaving unexpectedly, you can inspect or delete this file:

```bash
# View the database (requires sqlite3)
sqlite3 ~/dotfiles/.tidydots.db ".tables"
sqlite3 ~/dotfiles/.tidydots.db "SELECT * FROM renders;"

# Reset template state (forces fresh renders on next restore)
rm ~/dotfiles/.tidydots.db
```

!!! warning
    Deleting `.tidydots.db` removes all stored render history. The next `tidydots restore` will treat all templates as if they are being rendered for the first time, which means any manual edits to `.tmpl.rendered` files may be overwritten.

### Override OS detection

If you want to see what tidydots would do on a different OS without switching machines:

```bash
tidydots list -o windows
tidydots restore -o linux -n
```

### Override the config directory

If you have multiple dotfiles repos or want to test a different configuration:

```bash
tidydots restore -d ~/work-dotfiles -n
tidydots list -d /tmp/test-dotfiles
```
