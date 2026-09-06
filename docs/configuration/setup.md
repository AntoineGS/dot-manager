# Setup Entries

A **setup entry** performs a system change that config files alone cannot: enabling a
systemd unit, registering a shell completion, installing a git hook. It pairs a `check`
command (does this already hold?) with a `run` command (make it hold).

Setup entries run during `tidydots restore`, in the order they appear under `entries`.

## Example

The `vicinae` launcher ships a systemd user unit. Installing the package and symlinking
the config is not enough — the unit must also be enabled:

```yaml
  - package:
      managers:
        yay: vicinae-bin
    name: vicinae
    entries:
      - targets:
          linux: ~/.config/vicinae
        name: config
        backup: ./Linux/vicinae
        files: [settings.json]

      - name: enable-service
        when: '{{ eq .Hostname "omarchbook" }}'
        check:
          linux: systemctl --user is-enabled --quiet vicinae.service
        run:
          linux: systemctl --user enable --now vicinae.service
```

On restore, tidydots runs the check. If the unit is already enabled the check exits 0 and
nothing happens. If it is not, the run command executes, and the check runs a second time
to confirm the change actually took effect.

The optional entry-level `when` expression uses the same syntax as an
[application condition](applications.md#when-expressions). Both the application and
entry conditions must evaluate to `true` for this setup entry to apply. A false
result or template error skips the entry **before** tidydots evaluates the OS maps,
runs `check` (including a dry-run check), or runs `run`; it is also hidden from the
TUI manage view. Explicitly targeting an excluded setup entry returns a conditions
mismatch error.

## How it works

```
1. no `run` command for this OS  -> skip
2. `check` reports applied        -> skip, report "Set up"
3. `check` reports an error       -> report the error; never run setup
4. --dry-run                     -> report what would run; never runs it
5. execute `run`                 -> a non-zero exit is an error
6. re-run `check`                -> a non-applied result is an error
```

An indeterminate or failed status check blocks the run even during `--dry-run`.
This prevents a remote or otherwise unavailable check from being mistaken for a
missing setup. Step 6 catches a script that exits 0 without doing its job. A
post-check diagnostic is surfaced as an error, while a legacy `exit-code` check
continues to treat every nonzero exit as "still failing."

For a status check, a nonblank diagnostic on **stderr** from exit 1 or 2 is kept
as a sanitized, bounded diagnostic. These exits remain non-errors for the
initial check and still authorize `run`; if the post-check returns 1 or 2, its
diagnostic is appended to the existing "command succeeded but check still
fails" message. With no diagnostic, that legacy generic message is unchanged.
Status diagnostics do not use stdout. For Check failed results, tidydots keeps
the existing stderr-first behavior, then uses the execution error or a fixed
exit-code fallback.

## Editing in the TUI

The TUI application form can create and edit setup entries. On the sub-entry form, enable
**Setup entry** to replace config fields with these command pairs:

- `Check (linux)` and `Run (linux)`
- `Check (windows)` and `Run (windows)`
- `Check mode` — `exit-code` (default) or `status`
- `Sudo`
- `When` — optional condition for this individual setup entry

Each configured OS must have both a check and a run command, and at least one OS must be
configured. The form refuses to save a partial pair or an entry with no configured OS.
Setup entries cannot have `backup` or `targets`; choose the config-entry mode to edit those
fields instead. Saving writes the resulting `check` and `run` maps plus the optional
`check_mode` and `when` values to `tidydots.yaml`.

For the **When** field, `enter` or `e` opens the hostname chooser when saved hostname
choices are available in `tidydots.yaml`. Select hosts with `space` or `tab`, then
confirm to generate an expression; choose **Type expression** to enter a Go-template
condition manually.

From the main TUI list, `r` runs the selected setup entry (or selected application's setup
entries), while `x` includes entries whose check currently fails in the action filter. Setup
checks also run during TUI state detection, so keep them read-only and fast.

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Required. Identifies the step in output and in the TUI. |
| `when` | string | Optional. Go-template condition for this setup entry. |
| `check` | map: OS → command | Required. Exit 0 means "already set up". |
| `run` | map: OS → command | Required. Runs only when `check` fails. |
| `check_mode` | string | Optional. `exit-code` (default) or opt-in `status`; setup entries only. |
| `sudo` | bool | Optional. Runs `run` with elevated privileges. `check` never uses sudo. |

A sub-entry is either a **config entry** (it has a `backup`) or a **setup entry** (it has
a `run`). It cannot be both, and a setup entry cannot declare `targets`.

Every OS listed under `run` must also be listed under `check`, and vice versa. This is
enforced at load time.

### Check mode

By default, or when `check_mode` is omitted, checks use `exit-code` mode: exit code 0
means the setup is already applied, and any nonzero exit code means setup is needed. This
preserves the standard setup-entry behavior.

Set `check_mode: status` to opt into status mode. The check command's exit code then has
the following meaning:

| Exit code | Meaning |
| --- | --- |
| `0` | Set up |
| `1` | Needs setup |
| `2` | Outdated |
| `3` or higher | Check failed (indeterminate or error) |

Negative process exit codes and failures to launch the check are errors. The `run` command
always keeps ordinary success/failure semantics: exit code 0 means success and any nonzero
exit code means failure; `check_mode` does not assign status meanings to `run`.

A **Check failed** result is attention-worthy, but it is not permission to run `run`: the
check must safely resolve to **Needs setup** or **Outdated** first. Status reporting never
runs setup commands.

## The OS map is the platform gate

An absent OS key means the step does not apply there. The entry above has no `windows:`
key, so it is skipped entirely on Windows. No `when:` clause is needed for this — though
the application's own `when:` still gates the whole group. Use an entry-level `when:`
when the step is restricted by context other than its OS map, such as hostname.

## Commands

Commands run through `sh -c` on Unix and `powershell -Command` on Windows, with the
configurations repo root as the working directory. That means multi-line commands and
repo-relative script paths both work:

```yaml
      - name: install-hooks
        check:
          linux: test -x /etc/pacman.d/hooks/pkg-backup-aur.hook
        run:
          linux: sh ./Linux/pacman/install-hooks.sh
        sudo: true
```

## The check contract

**Check commands must be side-effect free and fast.**

They are executed:

- on every `tidydots restore`,
- during `--dry-run` (this is how dry-run can truthfully report whether the setup *would*
  run),
- and on every TUI state-detection pass.

`systemctl --user is-enabled --quiet vicinae.service` satisfies both requirements. A check
that mutates state, prompts, or takes seconds to return will make tidydots feel broken.

tidydots cannot enforce this — it is a contract you accept when you write a setup entry.

## No state is stored

There is no database and no marker file. The check *is* the state. If you later disable
the vicinae unit by hand, the next restore notices and re-enables it.

This is deliberate: a "we already ran this once" record would go stale the moment its
effect was undone, and the step would never repair itself.

## Security

Setup commands are arbitrary shell commands from your configuration file. Only use
configurations you trust.
