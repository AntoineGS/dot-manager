# Installation

## Prerequisites

tidydots is written in Go. You need **Go 1.21 or later** installed on your system.

To check your Go version:

```bash
go version
```

If you do not have Go installed, follow the [official Go installation guide](https://go.dev/doc/install) for your platform.

## Install with `go install`

The simplest way to install tidydots is with `go install`:

```bash
go install github.com/antoinegs/tidydots/cmd/tidydots@latest
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
git clone https://github.com/antoinegs/tidydots.git
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
  tidydots [command]

Available Commands:
  backup        Backup configurations from target locations
  completion    Generate the autocompletion script for the specified shell
  help          Help about any command
  init          Initialize app configuration
  install       Install packages using configured package managers
  list          List all configured paths
  list-packages List all configured packages
  restore       Restore configurations by creating symlinks

Flags:
  -d, --dir string   Override configurations directory (ignores app config)
  -n, --dry-run      Show what would be done without making changes
  -h, --help         help for tidydots
  -o, --os string    Override OS detection (linux or windows)
  -v, --verbose      Enable verbose output
```

## Supported platforms

tidydots works on:

- **Linux** -- All major distributions (Arch, Ubuntu, Fedora, etc.)
- **Windows** -- With symlink/junction support

## Next steps

Once installed, head to the [Quick Start](quick-start.md) guide to set up your first dotfiles repository with tidydots.
