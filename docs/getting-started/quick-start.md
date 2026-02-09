# Quick Start

This guide walks you through setting up tidydots from scratch. By the end, you will have a dotfiles repository with a working configuration that symlinks your files into place.

## 1. Create a dotfiles repository

If you already have a dotfiles repo, skip to [step 2](#2-initialize-tidydots).

Create a new directory for your dotfiles and initialize it as a Git repository:

```bash
mkdir ~/dotfiles
cd ~/dotfiles
git init
```

!!! tip
    A common convention is to keep your dotfiles repo at `~/dotfiles` or `~/.dotfiles`, but you can put it anywhere you like.

## 2. Initialize tidydots

Point tidydots at your dotfiles repository:

```bash
tidydots init ~/dotfiles
```

You should see:

```
App configuration saved to /home/youruser/.config/tidydots/config.yaml
Configurations directory: /home/youruser/dotfiles
```

This creates an app config file at `~/.config/tidydots/config.yaml` that remembers where your dotfiles live. You only need to run this once per machine.

!!! note
    If your dotfiles repo does not contain a `tidydots.yaml` file yet, you will see a warning. That is expected -- you will create it in the next step.

## 3. Create your configuration file

Create a `tidydots.yaml` in the root of your dotfiles repo. This file describes which configuration files to manage and where they belong on the system.

Here is a simple example that manages a Neovim configuration:

```yaml title="~/dotfiles/tidydots.yaml"
version: 3
backup_root: "."

applications:
  - name: "neovim"
    description: "Neovim text editor"

    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"

    package:
      managers:
        pacman: "neovim"
        apt: "neovim"
        brew: "neovim"
```

This tells tidydots:

- There is an application called "neovim"
- It has one config entry (`nvim-config`) whose source files live in `./nvim` inside your dotfiles repo
- On Linux, the config should appear at `~/.config/nvim`; on Windows, at `~/AppData/Local/nvim`
- The neovim package can be installed via pacman, apt, or brew

## 4. Add your config files to the repo

Copy (or move) your existing Neovim config into the backup location defined above:

```bash
# If you already have a Neovim config, copy it into your dotfiles repo
cp -r ~/.config/nvim ~/dotfiles/nvim
```

Your repo should now look like this:

```
~/dotfiles/
  tidydots.yaml
  nvim/
    init.lua
    lua/
      ...
```

!!! tip
    If you have an existing config on the machine but have not yet copied it into your repo, tidydots can **adopt** it for you automatically. When you run `restore`, if the target exists but the backup does not, tidydots moves the target into the backup location and then creates the symlink. See [Concepts](concepts.md#adopt) for more details.

## 5. Preview with dry-run

Before making any changes, use the `-n` (dry-run) flag to see what tidydots would do:

```bash
tidydots restore -n
```

You should see output like:

```
Detected OS: linux
Config directory: /home/youruser/dotfiles
=== DRY RUN MODE ===
time=... level=INFO msg="starting restore" os=linux version=3
time=... level=INFO msg="restoring application" app=neovim
time=... level=INFO msg="creating symlink" target=/home/youruser/.config/nvim source=/home/youruser/dotfiles/nvim
```

!!! warning
    Always preview with `-n` before running a restore for the first time. This lets you verify that paths are correct and nothing unexpected will happen.

## 6. Run the actual restore

Once you are satisfied with the dry-run output, run the real restore:

```bash
tidydots restore
```

```
Detected OS: linux
Config directory: /home/youruser/dotfiles
time=... level=INFO msg="starting restore" os=linux version=3
time=... level=INFO msg="restoring application" app=neovim
time=... level=INFO msg="creating symlink" target=/home/youruser/.config/nvim source=/home/youruser/dotfiles/nvim
```

## 7. Verify the symlinks

Confirm that the symlink was created:

```bash
ls -la ~/.config/nvim
```

```
lrwxrwxrwx 1 youruser youruser 34 Jan 15 10:30 /home/youruser/.config/nvim -> /home/youruser/dotfiles/nvim
```

Your Neovim configuration is now managed by tidydots. The actual files live in your dotfiles repo, and `~/.config/nvim` is a symlink pointing to them.

## 8. Commit your dotfiles

Now that everything is working, commit your configuration to Git:

```bash
cd ~/dotfiles
git add tidydots.yaml nvim/
git commit -m "Add neovim configuration"
```

## Adding more applications

You can add as many applications as you need. Here is a more complete example with multiple applications:

```yaml title="~/dotfiles/tidydots.yaml"
version: 3
backup_root: "."

applications:
  - name: "neovim"
    description: "Neovim text editor"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"

  - name: "zsh"
    description: "Zsh shell configuration"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "zshrc"
        backup: "./zsh"
        files: [".zshrc", ".zshenv"]
        targets:
          linux: "~"
    package:
      managers:
        pacman: "zsh"
        apt: "zsh"

  - name: "git"
    description: "Git version control"
    entries:
      - name: "git-config"
        backup: "./git"
        files: [".gitconfig"]
        targets:
          linux: "~"
          windows: "~"
```

!!! note
    The `when` field on the zsh application means it will only be included on Linux machines. Applications without a `when` field are always included. See [Concepts](concepts.md#when-expressions) for more details.

## What's next?

- Read [Concepts](concepts.md) to understand how tidydots works under the hood
- Explore the [Configuration](../configuration/overview.md) reference for all available options
- Try the interactive TUI by running `tidydots` with no arguments
- Learn about [package management](../guides/package-management.md) to install software alongside your configs
