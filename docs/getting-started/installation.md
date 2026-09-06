# Installation

## Prerequisites

tidydots is written in Go. You need **Go 1.26 or later** installed on your system.

To check your Go version:

```bash
go version
```

If you do not have Go installed, follow the [official Go installation guide](https://go.dev/doc/install) for your platform.

## macOS (Homebrew)

Install tidydots on macOS using the Homebrew tap:

```bash
brew install AntoineGS/tidydots/tidydots
```

!!! note
    macOS is fully supported by tidydots. Internally, macOS uses the `linux` OS key for target paths and `when` expressions. When writing your `tidydots.yaml`, use `linux` for macOS paths:

    ```yaml
    targets:
      linux: "~/.config/nvim"   # Used on both Linux and macOS
      windows: "~/AppData/Local/nvim"
    ```

## Arch Linux (AUR)

For Arch Linux users, tidydots is available on the AUR:

```bash
# Using an AUR helper (e.g., yay or paru)
yay -S tidydots-git
paru -S tidydots-git
```

Or build manually from the AUR:

```bash
git clone https://aur.archlinux.org/tidydots-git.git
cd tidydots-git
makepkg -si
```

## Install with `go install`

The simplest way to install tidydots is with `go install`:

```bash
go install github.com/AntoineGS/tidydots/cmd/tidydots@latest
```

This downloads, compiles, and installs the `tidydots` binary into your `$GOPATH/bin` directory (usually `~/go/bin`).

!!! tip
    Make sure `$GOPATH/bin` is in your `PATH`. You can add this to your shell profile:

    ```bash
    export PATH="$PATH:$(go env GOPATH)/bin"
    ```

## Build from source

If you prefer to build from source, or want to contribute to the project:

```bash
git clone https://github.com/AntoineGS/tidydots.git
cd tidydots
go build ./cmd/tidydots
```

This produces a `tidydots` binary in the current directory. You can move it to a directory on your `PATH`:

```bash
# Linux / macOS
sudo mv tidydots /usr/local/bin/

# Or keep it local
mv tidydots ~/go/bin/
```

## Install from a local checkout

From a tidydots source checkout, `make install` installs the current local
`HEAD` with:

```bash
make install
```

The target runs `go install ./cmd/tidydots` and does **not** rewrite a setup
script, YAML file, or any other configuration. A local development checkout may
be behind, ahead of, or different from the repository's remote default branch;
that is expected. `make install` builds what is checked out locally and does not
fetch or update the checkout.

### Optional remote-default-branch updater

Remote revision tracking is an opt-in setup entry for a personal configuration;
it is not enabled by default and is not part of every configured Git package.
The updater resolves the repository's advertised remote default branch, rather
than assuming a branch name such as `main`, and compares that remote revision
with the installed Go binary's `vcs.revision` metadata.

An updater replacement and its matching YAML change can be staged for review
without changing the active configuration. Keep those migration artifacts
inactive until a supporting tidydots binary is installed and the two changes
are deliberately activated together; a symlinked script source is active
configuration, not an inert copy.

#### Commands after coordinated activation

After the supporting tidydots binary is installed and the script/YAML
migration has been reviewed and activated, the replacement supports these
commands:

```bash
~/.config/tidydots/setup-tidydots.sh --check
~/.config/tidydots/setup-tidydots.sh --apply
```

Before that coordinated activation, do not run the active-path `--apply`
command. If the active path still resolves to a legacy script, it does not
implement this status-mode contract; after activation, verify that the path
resolves to the reviewed replacement rather than treating a prepared copy as
active.

`--check` returns status `0` (**Set up**) when the installed binary matches, `1`
(**Needs setup**) when the binary is missing, `2` (**Outdated**) when it differs,
and `3` or higher (**Check failed**) when the state is indeterminate (for
example, missing Go VCS metadata, remote lookup failure, or timeout); an
indeterminate result never claims that an update is available.
`--apply` only updates a clean checkout whose origin, branch, and repository
root match the configured repository; it uses fast-forward-only Git updates and
never resets, stashes, or discards local commits.

Repository identity is intentionally strict: the checkout's `origin` URL and
`TIDYDOTS_REPOSITORY` must have the same spelling. The updater does not
canonicalize HTTPS, SSH, or scp-style Git URLs, even when they identify the
same hosted repository. Use the checkout's transport spelling in the
configured repository value. For example, the coordinated SSH-form commands
preserve an existing non-empty override while defaulting both operations to
the SSH spelling:

```bash
TIDYDOTS_REPOSITORY="${TIDYDOTS_REPOSITORY:-git@github.com:AntoineGS/tidydots.git}" \
  ~/.config/tidydots/setup-tidydots.sh --check
TIDYDOTS_REPOSITORY="${TIDYDOTS_REPOSITORY:-git@github.com:AntoineGS/tidydots.git}" \
  ~/.config/tidydots/setup-tidydots.sh --apply
```

The updater requires Bash 4.4 or newer, Git, Go, `make` for `--apply`, and the
coreutils `timeout` command (plus `env`). It performs noninteractive remote
metadata lookups with a five-second limit. These environment variables can override its
defaults when testing or using a different layout:

```bash
TIDYDOTS_REPOSITORY   # remote repository URL
TIDYDOTS_SOURCE_DIR   # local checkout
TIDYDOTS_BIN_DIR      # installation directory
TIDYDOTS_BINARY       # installed binary used for verification
```

The migration must be coordinated: first install a tidydots binary that
understands status-mode setup entries, then activate the matching setup script
and YAML change. Preview the configuration restore with `tidydots restore -n`
before applying it. A symlinked script source is active configuration, not a
safe inactive copy.

## Verify the installation

Run the help command to confirm tidydots is installed and accessible:

```bash
tidydots --help
```

You should see output similar to:

```
tidydots is a cross-platform tool for managing dotfiles and configurations.
It supports backup and restore operations using symlinks, with support for
both Windows and Linux systems.

Configuration is stored in two places:
  ~/.config/tidydots/config.yaml  - Points to your configurations repo
  <repo>/tidydots.yaml            - Defines paths to manage

Run 'tidydots init <path>' to set up the app configuration.
Run without arguments to start the interactive TUI.

Usage:
  tidydots [flags]
  tidydots [command]

Available Commands:
  backup        Backup configurations from target locations
  completion    Generate the autocompletion script for the specified shell
  help          Help about any command
  init          Initialize app configuration
  install       Install packages using configured package managers
  list          List configured paths
  list-packages List all configured packages
  preview       Live preview template rendering
  restore       Restore configurations by creating symlinks
  status        Show resolved configuration and package status

Flags:
      --actions      Start the interactive TUI with action filtering enabled
  -d, --dir string   Override repository directory, including TUI hostname choices
  -n, --dry-run      Show what would be done without making changes
  -h, --help         help for tidydots
  -o, --os string    Override OS detection (linux or windows)
  -v, --verbose      Enable verbose output
      --version      version for tidydots
```

## Supported platforms

tidydots works on:

- **Linux** -- All major distributions (Arch, Ubuntu, Fedora, etc.)
- **macOS** -- Via Homebrew; uses the `linux` OS key internally for target paths
- **Windows** -- With symlink/junction support

## Local app configuration

The app configuration at `~/.config/tidydots/config.yaml` points to the repository that
contains `tidydots.yaml`:

```yaml
config_dir: ~/.dotfiles
```

Configure hostname choices in the version-controlled `tidydots.yaml` repository config:

```yaml
version: 3
hostnames:
  - desktop
  - laptop
```

When selected in the TUI, the choices generate Go-template expressions such as
`{{ eq .Hostname "desktop" }}` for one host or
`{{ or (eq .Hostname "desktop") (eq .Hostname "laptop") }}` for multiple hosts.
For compatibility, the TUI uses `hostnames` from the local app config only when the repository
does not define them.

## Next steps

Once installed, head to the [Quick Start](quick-start.md) guide to set up your first dotfiles repository with tidydots.
