# Interactive TUI

tidydots includes an interactive terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and styled with [Lipgloss](https://github.com/charmbracelet/lipgloss). The TUI provides a visual way to browse, edit, and manage your dotfiles configuration without memorizing CLI commands.

## Launching the TUI

There are two ways to start the interactive interface:

```bash
# Launch directly (no arguments)
tidydots

# Or use the -i flag with any command
tidydots restore -i
tidydots backup -i
tidydots install -i
```

Running `tidydots` with no arguments opens the full TUI experience. Using `-i` with a specific command opens the TUI focused on that operation.

## Main screen

The main screen displays a table view of all your applications and their entries. Each row shows:

- **Application name** and description
- **Entry names** (configs and packages) nested under their application
- **Status indicators** showing the current state of each entry

### Status indicators

| Status | Meaning |
|--------|---------|
| Ready | Backup/source exists, target does not -- ready to restore |
| Linked | Symlink is correct, or a copy target is present and in sync |
| Adopt | Target exists but backup does not -- symlink mode can adopt the existing file |
| Missing | Neither backup nor target exist |
| Outdated | A listed `.tmpl` source or its render history requires an update; symlink entries inspect rendered output and copy entries inspect the suffix-free target |
| Modified | A rendered template file or suffix-free copy target differs from its pure render; folder entries inspect discovered templates recursively |
| Unavailable | tidydots could not safely inspect a selected template source or live target; review access and run the TUI in an environment that can read the file |
| Loading... | State not yet resolved -- shown briefly for [setup entries](../configuration/setup.md) while their check command runs |
| Set up | Setup entry: the check command passed -- nothing to do |
| Needs setup | Setup entry: the check command failed -- restore will run the setup command |

!!! info
    Setup entries can't be resolved by inspecting the filesystem the way config entries can -- their state comes from actually running the entry's `check` command. tidydots runs that check in a background goroutine rather than on the UI thread, so a setup entry's row may briefly show **Loading...** before settling on **Set up** or **Needs setup**.

For config entries with an explicit `files` list, “listed” means only the
selected `.tmpl` source names participate in template status. An entry with an
empty `files` list is a folder entry; its templates are discovered recursively.

## Navigation

tidydots uses vim-style keybindings alongside arrow keys for navigation.

### Core keybindings

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Collapse application row |
| `→` / `l` / `enter` | Expand application row (show sub-entries) |
| `e` | Edit the selected application, config entry, or setup entry in the TUI |
| `esc` | Go back or cancel (see [priority](#clearing-selections)) |
| `tab` / `space` | Toggle selection |
| `/` | Search and filter |
| `f` | Toggle filter (show/hide apps excluded by `when` expressions) |
| `x` | Toggle the action filter (show only applications or entries needing work) |
| `ctrl+r` | Refresh all package, config, template, and setup statuses |
| `r` | Restore the selected application or entry, deploying symlinks/copies and preserving rendered-template edits through the normal merge |
| `R` | Force Restore the selected application or entry, always requiring confirmation and discarding rendered-template edits |
| `ctrl+u` | Move up by half the visible table height |
| `ctrl+d` | Move down by half the visible table height |
| `gg` | Move to the first row |
| `G` | Move to the last row |
| `s` / `ctrl+s` | Save changes |
| `i` | Context-sensitive: install package (on app row) or view diff (on modified entry) |
| `m` | Show results from the last operation |
| `p` | Edit package dependencies (in package form) |
| `d` / `delete` / `backspace` | Delete selected item |
| `q` | Quit |

### Adding items

| Key | Action |
|-----|--------|
| `A` | Add a new application |
| `a` | Choose and add a new sub-entry to the current application |

Pressing `a` opens an entry-type chooser before the edit form:

- **Folder symlink** links an entire backup directory to its target.
- **File symlink** links selected files from a backup directory.
- **File copy** deploys selected files as real copies instead of symlinks.
- **Setup command** runs an OS-specific command when its read-only check fails.

Use `up`/`k` and `down`/`j` to choose a type, `enter` to continue to the form,
or `esc` to cancel. The form's type controls remain editable before saving.

### Sorting

Press a sort key to sort by that column. Press the same key again to reverse the direction.

| Key | Sort by |
|-----|---------|
| `n` | Name |
| `t` | Status |
| `p` | Path |

### Search and filter

Press `/` to enter search mode. Type to filter applications and entries by name, description, target paths, or backup paths. The list updates in real time as you type. Press `enter` to confirm or `esc` to exit search mode (your selections are preserved).

Press `f` to toggle the filter. When enabled (the default), applications excluded by
their application-level `when` expression are hidden. When disabled, those applications
are shown. Entries excluded by their own `when` are removed before table filtering and
remain absent in either mode.

Press `x` to toggle the action filter. It keeps applications with an uninstalled package and entries whose state needs attention (`Missing`, `Ready`, `Adopt`, `Needs setup`, `Outdated`, `Modified`, or `Unavailable`). The filter composes with search and the `f` platform filter. If enabling it would hide selected items, tidydots asks for confirmation; answer `y` to enable it or `n` to leave the current view and selections unchanged. Confirming does not clear selections.

The TUI can start with this filter already enabled by running `tidydots --actions`. This keeps the normal interactive behavior while showing actionable work as soon as status checks settle.

Package status is automatically rechecked after installs. Press `ctrl+r` on the clean main list to manually refresh all package, config, template, and setup statuses. The refresh preserves filters, selections, expansion, and cursor position; existing loading indicators show progress while statuses are rechecked.

The paging and jump motions operate only on the clean main list, not while search or a confirmation dialog is active. `gg` is a two-key sequence: press `g` twice. A single `g` waits for the second key and any other key cancels that pending sequence. Empty and one-row tables remain clamped safely.

### Mouse support

| Input | Action |
|-------|--------|
| Left click | Move cursor to clicked row |
| Right click | Move cursor and toggle selection |
| Scroll wheel | Scroll up/down by 3 rows |

## Two-phase editing

All text fields in the TUI use a two-phase editing approach. This prevents accidental edits while navigating.

### Phase 1: Navigation mode

When you navigate to a text field, it is **focused** but **not editable**. You can see the field is highlighted, but typing does not modify its value. Use `↑/k` and `↓/j` to move between fields.

### Phase 2: Edit mode

Press `enter` or `e` to enter edit mode. The field becomes active and shows a text cursor. Type to modify the value.

- Press `enter` to **save** your changes and return to navigation mode
- Press `esc` to **cancel** your changes and return to navigation mode

!!! info
    This two-phase pattern applies to all editable fields: name, description, targets, backup path, repository URL, branch, file list items, and `when` expressions.

## Multi-selection

The TUI supports selecting multiple items for batch operations.

### Selecting items

| Key | Action |
|-----|--------|
| `tab` / `space` | Toggle selection on the current item |

- **Selecting an application** automatically selects all its visible sub-entries (configs and packages)
- **Deselecting an application** deselects all its sub-entries
- **Sub-entries** can be individually toggled even when their parent application is not selected

### Selection banner

When items are selected, a banner appears at the top of the screen showing the count:

```
2 app(s), 5 item(s) selected
```

Selections persist across search filtering, screen navigation, and edit operations.

### Clearing selections

Press `esc` to clear all selections. The `esc` key follows a priority system:

1. If in search mode, `esc` exits search first (selections are kept)
2. If selections exist, `esc` clears all selections
3. Otherwise, `esc` returns to the previous screen

## Batch operations

With items selected, you can perform operations on all of them at once. Each batch operation follows a three-screen flow.

### Available operations

| Key | Operation | Description |
|-----|-----------|-------------|
| `r` | Restore | Restore selected config entries using their configured method, and run selected [setup entries](../configuration/setup.md), preserving rendered-template edits through the normal merge |
| `R` | Force Restore | Use the same selected application or entry scope as Restore, always show a confirmation, and discard manual edits to rendered-template files |
| `i` | Install | Install packages for all selected applications |
| `d` | Delete | Remove configs and packages for all selected items |

!!! info "Setup entries during restore"
    Restore runs [setup entries](../configuration/setup.md) the same way `tidydots restore` does on the command line: the entry's `check` command runs first, the setup command runs only if that check fails, and the check then runs again to confirm the change actually landed. An entry marked `sudo: true` may prompt for a password, so tidydots hands the terminal over to the command while it runs -- exactly as it does for package installation -- and returns to the TUI once it finishes. This applies whether you restore a single row, a whole application, or a batch selection.

### Three-screen flow

Every batch operation proceeds through three screens:

**1. Select screen (main screen)**

Browse and select the items you want to operate on. Use `tab` or `space` to toggle selections.

**2. Summary screen**

After pressing an operation key (`r`, `R`, `i`, or `d`), a summary screen appears showing exactly what will be changed. Restore (`r`) preserves manual edits to rendered templates through the normal merge. Force Restore (`R`) always asks for confirmation before proceeding and discards those edits. Review the list of operations, then:

- Press `y` or `enter` to confirm and proceed
- Press `n` or `esc` to cancel and return to the main screen

**3. Progress screen**

Once confirmed, a progress screen shows real-time feedback as each operation executes. A progress bar tracks completion.

**4. Results popup**

When the operation finishes, a popup overlay appears showing all results (success or failure for each item). Press `enter` or `esc` to dismiss. Press `m` from the main screen to re-open the results at any time. If results exceed the popup height, scroll with `↑/k` and `↓/j`.

!!! tip
    The global `-n` (dry-run) flag works with batch operations too. When dry-run is enabled, the progress screen shows what *would* happen without making actual changes.

### Example workflow

1. Launch tidydots: `tidydots`
2. Navigate to the applications you want to restore
3. Press `tab` on each application to select it
4. Press `r` to start batch restore, or `R` for Force Restore when rendered-template edits should be discarded
5. Review the summary of deployments to be made
6. Press `enter` to confirm
7. Watch the progress bar as deployments are applied

## Editing applications and entries

### Edit an application

Navigate to an application row and press `e` to open the edit screen. You can modify:

- **Name** -- the application identifier
- **Description** -- optional description text
- **When** -- conditional expression for machine filtering

If hostname choices are configured at the top level of `tidydots.yaml`, focusing **When** and pressing `enter` or `e` opens a chooser. Use `space` or `tab` to select one or more hosts. Press `enter` or `e` to apply the selection without saving the form, or `s` / `ctrl+s` to apply it and save immediately. The chooser also provides **Type expression** for manual Go-template input. For example, selecting `desktop` generates:

```yaml
when: '{{ eq .Hostname "desktop" }}'
```

Selecting `desktop` and `laptop` generates:

```yaml
when: '{{ or (eq .Hostname "desktop") (eq .Hostname "laptop") }}'
```

### Editing package dependencies

When editing an application's packages section, you can manage dependencies for any standard package manager:

1. Navigate to the packages section of the application form
2. Move to a native package manager entry (e.g., `winget`, `apt`)
3. Press `p` to open the dependency editor for that manager
4. Use the list editor to add, edit, or delete dependencies:
   - `↑/k`, `↓/j` to navigate
   - `enter` or `e` to edit an item or add a new one
   - `d` or `delete` to remove a dependency
   - `esc` to exit the dependency editor
5. Dependencies are shown as a count indicator on the manager row: `winget: sxyazi.yazi (3 deps)`

### Edit a config entry

Navigate to a config entry and press `e` to edit. Editable fields include:

- **Name** -- entry identifier
- **Backup** -- path in your dotfiles repo
- **Targets** -- OS-specific target paths (linux, windows)
- **Files** -- specific file list (empty means entire folder)
- **Sudo** -- toggle for elevated privileges
- **Copy files** -- toggle for [`method: copy`](../configuration/configs.md#deployment-method), which deploys real files instead of symlinks
- **When** -- optional Go-template condition for this individual entry

The **Copy files** toggle only appears when an explicit file list is set, because copy mode is files-only. Switching an entry back to whole-folder mode therefore clears it.

Focusing the sub-entry **When** field and pressing `enter` or `e` opens the same
hostname chooser used for applications when hostnames are configured in `tidydots.yaml`.
Use `space` or `tab` to select hosts and confirm to generate a condition, or
choose **Type expression** for manual Go-template input. The generated condition is
saved on that entry, so it combines with the parent application's condition.

### Edit a setup entry

 Navigate to an entry and press `e`, then toggle **Setup entry**. The form exposes paired `Check` and `Run` fields for Linux and Windows, **Sudo**, and **When**. The **When** editor and hostname chooser work the same way as for config entries. Each OS must have both commands or neither, and at least one OS must be configured. Save writes the `check`, `run`, and optional `when` string to `tidydots.yaml`. A setup entry cannot also have backup or target fields. You can run it from the TUI with `r` and delete it with `d`.

Check commands must remain read-only and fast because they run during every TUI state refresh; see [Setup Entries](../configuration/setup.md).

### Edit custom and URL packages

In an application's **Packages** section, move below the standard managers to **Custom** or **URL download** and press `enter` to add or open that section. Use `↑/k` and `↓/j` to move through its fields, `e` or `enter` to edit a text field, and `enter` to confirm the text. `esc` cancels the current field edit; `s` or `ctrl+s` saves the application form. Press `d`, `delete`, or `backspace` on the section row to remove the complete custom or URL section and its values.

Custom packages require at least one Linux or Windows command. URL packages require both a URL and an install command for each OS where either value is supplied; use `{file}` in the command for the downloaded file path. Command fields (including installer and setup commands) are unbounded text areas and preserve pasted multiline content and tabs. Invalid partial sections are rejected when saving rather than written to the repository.

### File picker

When editing the files list of a config entry, you have two ways to add files:

**Browse Files mode**

1. Navigate to the files field and press `enter`
2. Select "Browse Files" from the menu
3. An interactive file browser opens, starting in the target directory
4. Navigate with `↑/↓` or `k/j`, toggle file selection with `space` or `tab`
5. Selected files are highlighted with a purple background
6. Press `enter` to confirm your selections
7. Files are automatically converted to relative paths

**Type Path mode**

1. Navigate to the files field and press `enter`
2. Select "Type Path" from the menu
3. Type the relative file path directly
4. Press `enter` to confirm

!!! note
    The file picker only accepts files within the target directory hierarchy. Selected files are stored as relative paths.

### List field navigation

When editing a list field (like files), the field has its own internal cursor:

- `↑/k` and `↓/j` navigate within the list
- At the top or bottom of the list, navigation moves to adjacent form fields
- `enter` or `e` on a list item enters edit mode for that item
- `enter` or `e` on the "Add" button starts adding a new item
- `d`, `delete`, or `backspace` removes the selected item

## Saving changes

Press `s` or `ctrl+s` to save your changes to the `tidydots.yaml` configuration file. The TUI writes back to the same file it loaded from.

!!! warning
    Save writes to your `tidydots.yaml` immediately. If you want to preview changes first, use dry-run mode (`tidydots -n`) to confirm behavior before saving.

## Template diff & edit

When a config entry uses templates (`.tmpl` files) and you have manually edited the generated output, the entry shows a **Modified** status in blue. For `method: copy`, this check reads the live suffix-free target file; it does not use a `.tmpl.rendered` cache. A changed source or missing render history shows **Outdated**. If the source or target cannot be read safely, the entry shows **Unavailable** in red and is included by the action filter.

For an entry with a `files` list, status and diff discovery inspect only listed `.tmpl` source names; an unlisted template does not affect the row. List the `.tmpl` source name (not its suffix-free target name) to render and deploy a single template. Template status checks never request `sudo` interactively. Run the TUI in an appropriately authorized environment when protected content is unavailable.

### Viewing diffs

1. Navigate to a sub-entry row showing **Modified** status
2. Press `i` to launch the diff viewer
3. If the entry contains multiple modified template files, a picker appears -- select the file you want to inspect
4. Your editor opens with two panes:
    - **Left pane**: A unified diff showing your edits (read-only); copy-mode diffs read the real deployed target
    - **Right pane**: The `.tmpl` source file (editable)
5. Edit the template to backport your changes, then save and quit your editor
6. The TUI resumes and refreshes the entry status

### Editor detection

tidydots automatically detects the best way to launch the editor:

| Mode | Condition | Behavior |
|------|-----------|----------|
| **Neovim** (default) | `nvim` is on `$PATH` | Opens both files in vertical splits with the diff pane read-only |
| **Tmux** | Running inside tmux with `$EDITOR` set | Opens template in a tmux split pane, diff in the current pane |
| **Fallback** | Neither nvim nor tmux available | Opens just the template in `$EDITOR` (or `vim`/`vi`/`nano`) |

!!! tip
    The diff compares the **pure render** (what the template produced) against the **current file on disk** (with your edits). For symlink entries that current file is `.tmpl.rendered`; for copy entries it is the live suffix-free target. In both cases the editor opens the `.tmpl` source, not the deployed file, so you can backport changes safely. `--force-render` applies to copy templates as well and overwrites the deployed target on restore.

## Help text

Context-sensitive help is displayed at the bottom of each screen. The help text updates based on your current state:

- In navigation mode: shows navigation and action keys
- In edit mode: shows save and cancel keys
- In selection mode: shows available batch operations

## Practical examples

### Restore specific configs interactively

```bash
tidydots restore -i
```

1. Browse the list of applications and config entries
2. Select the ones you want to restore with `tab`
3. Press `r` to restore
4. Review the summary and confirm

### Install packages interactively

```bash
tidydots install -i
```

1. Browse applications that have packages configured
2. Select which applications to install packages for
3. Press `i` to install
4. Review the summary showing which packages and managers will be used
5. Confirm to proceed

### Add a new application via TUI

1. Launch `tidydots`
2. Press `A` to add a new application
3. Fill in the name, description, and `when` expression
4. Press `a` to add config entries with backup paths and targets
5. Press `s` to save the configuration

## Next steps

- [Multi-Machine Setups](multi-machine-setups.md) -- conditional configs with `when` expressions
- [Package Management](package-management.md) -- package installation details
- [System Configs](system-configs.md) -- managing files requiring sudo
