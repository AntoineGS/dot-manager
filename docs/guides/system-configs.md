# System Configs

Some configuration files live outside your home directory and require root/sudo access to manage. tidydots handles this through the `sudo` flag, letting you manage system-level files like `/etc/hosts`, pacman hooks, and systemd units alongside your regular dotfiles.

## The `sudo` flag

The `sudo` flag tells tidydots to run symlink and file operations with elevated privileges. It can be set at two levels:

### Application-level sudo

Set `sudo: true` on the application to apply elevated privileges to **all entries** within it:

```yaml
applications:
  - name: "system-config"
    description: "System-level configuration files"
    sudo: true
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "hosts"
        backup: "./system/hosts"
        targets:
          linux: "/etc/hosts"

      - name: "resolv-conf"
        files: ["resolv.conf"]
        backup: "./system/network"
        targets:
          linux: "/etc"
```

Both entries inherit `sudo: true` from the application.

### Entry-level sudo

Set `sudo: true` on individual entries when only some entries in an application need elevated privileges:

```yaml
applications:
  - name: "dns-config"
    description: "DNS and network configuration"
    entries:
      - name: "user-dns-settings"
        backup: "./dns/user"
        targets:
          linux: "~/.config/dns"

      - name: "system-resolv"
        backup: "./dns/system"
        sudo: true
        targets:
          linux: "/etc/resolv.conf"
```

In this example, `user-dns-settings` is managed with normal permissions while `system-resolv` uses sudo.

!!! tip
    Prefer entry-level `sudo: true` over application-level when possible. This gives you finer control and makes it clear exactly which files require elevated access.

## Common examples

### /etc/hosts

Manage a custom hosts file for ad blocking or local development domains:

```yaml
applications:
  - name: "hosts-file"
    description: "Custom hosts file"
    sudo: true
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "hosts"
        files: ["hosts"]
        backup: "./system"
        targets:
          linux: "/etc"
```

Your `./system/hosts` file in the dotfiles repo is symlinked to `/etc/hosts` with sudo.

### Pacman hooks

On Arch Linux, custom pacman hooks live in `/usr/share/libalpm/hooks/`:

```yaml
applications:
  - name: "pacman-hooks"
    description: "Custom pacman hooks"
    sudo: true
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "hooks"
        files: ["cleanup.hook", "update-grub.hook"]
        backup: "./pacman/hooks"
        targets:
          linux: "/usr/share/libalpm/hooks"
```

### Pacman configuration

```yaml
applications:
  - name: "pacman-config"
    description: "Pacman configuration"
    sudo: true
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        files: ["pacman.conf"]
        backup: "./pacman"
        targets:
          linux: "/etc"

      - name: "makepkg-conf"
        files: ["makepkg.conf"]
        backup: "./pacman"
        targets:
          linux: "/etc"
```

### Systemd units

Manage custom systemd service files:

```yaml
applications:
  - name: "systemd-services"
    description: "Custom systemd service units"
    sudo: true
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "backup-timer"
        files: ["backup.service", "backup.timer"]
        backup: "./systemd"
        targets:
          linux: "/etc/systemd/system"
```

!!! note
    After restoring systemd units, you still need to enable and start them manually:

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable --now backup.timer
    ```

### Systemd user units (no sudo needed)

User-level systemd units do not require sudo:

```yaml
applications:
  - name: "systemd-user"
    description: "User-level systemd services"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "user-services"
        files: ["syncthing.service"]
        backup: "./systemd-user"
        targets:
          linux: "~/.config/systemd/user"
```

### SSH server configuration

```yaml
applications:
  - name: "sshd-config"
    description: "SSH server configuration"
    sudo: true
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "sshd"
        files: ["sshd_config"]
        backup: "./ssh"
        targets:
          linux: "/etc/ssh"
```

## Filtering by distribution

System configuration files are often distro-specific. Use `when` expressions to apply them only where they belong:

```yaml
applications:
  # Arch-specific system configs
  - name: "arch-system"
    description: "Arch Linux system configuration"
    sudo: true
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        files: ["pacman.conf"]
        backup: "./arch/pacman"
        targets:
          linux: "/etc"

      - name: "mkinitcpio"
        files: ["mkinitcpio.conf"]
        backup: "./arch"
        targets:
          linux: "/etc"

  # Ubuntu-specific system configs
  - name: "ubuntu-system"
    description: "Ubuntu system configuration"
    sudo: true
    when: '{{ eq .Distro "ubuntu" }}'
    entries:
      - name: "apt-sources"
        backup: "./ubuntu/apt"
        targets:
          linux: "/etc/apt/sources.list.d"

  # Fedora-specific system configs
  - name: "fedora-system"
    description: "Fedora system configuration"
    sudo: true
    when: '{{ eq .Distro "fedora" }}'
    entries:
      - name: "dnf-conf"
        files: ["dnf.conf"]
        backup: "./fedora"
        targets:
          linux: "/etc/dnf"
```

## Combining with hostname filtering

For machine-specific system configs, combine `when` with hostname checks:

```yaml
applications:
  - name: "server-config"
    description: "Server-specific system configuration"
    sudo: true
    when: '{{ and (eq .OS "linux") (eq .Hostname "my-server") }}'
    entries:
      - name: "nginx-conf"
        backup: "./server/nginx"
        targets:
          linux: "/etc/nginx"

      - name: "fail2ban"
        backup: "./server/fail2ban"
        targets:
          linux: "/etc/fail2ban"
```

## Security considerations

Managing system files through dotfiles is powerful but requires care.

!!! warning "Review what gets sudo access"
    Every entry with `sudo: true` creates symlinks as root. Before running `tidydots restore`, verify that:

    - You trust the content of the backup files being symlinked
    - The target paths are correct (a typo could overwrite critical system files)
    - No unintended files are included

### Best practices

1. **Use dry-run first.** Always preview system-level operations before executing:

    ```bash
    tidydots restore -n
    ```

2. **Be specific with `files`.** Instead of symlinking entire directories into system paths, list specific files:

    ```yaml
    # Preferred: explicit file list
    entries:
      - name: "hooks"
        files: ["cleanup.hook", "update-grub.hook"]
        backup: "./pacman/hooks"
        targets:
          linux: "/usr/share/libalpm/hooks"

    # Avoid: entire directory in a system path
    entries:
      - name: "hooks"
        backup: "./pacman/hooks"
        targets:
          linux: "/usr/share/libalpm/hooks"
    ```

3. **Separate system entries from user entries.** Keep system-level applications in their own section for easy auditing:

    ```yaml
    applications:
      # User configs
      - name: "neovim"
        entries:
          - name: "nvim-config"
            backup: "./nvim"
            targets:
              linux: "~/.config/nvim"

      # --- System configs below ---
      - name: "system-hosts"
        sudo: true
        when: '{{ eq .OS "linux" }}'
        entries:
          - name: "hosts"
            files: ["hosts"]
            backup: "./system"
            targets:
              linux: "/etc"
    ```

4. **Use `when` to restrict scope.** System configs are often distro-specific. Always add a `when` expression to prevent accidental application on the wrong system:

    ```yaml
    # Pacman hooks only on Arch
    when: '{{ eq .Distro "arch" }}'
    ```

## Practical example: full system setup

```yaml
version: 3
backup_root: "."

applications:
  # --- User-level configs ---
  - name: "neovim"
    entries:
      - name: "nvim-config"
        backup: "./nvim"
        targets:
          linux: "~/.config/nvim"
    package:
      managers:
        pacman: "neovim"

  # --- System-level configs ---
  - name: "hosts"
    description: "Custom hosts file"
    sudo: true
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "hosts-file"
        files: ["hosts"]
        backup: "./system"
        targets:
          linux: "/etc"

  - name: "pacman-hooks"
    description: "Custom pacman hooks"
    sudo: true
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "hooks"
        files: ["cleanup.hook", "orphan-check.hook"]
        backup: "./pacman/hooks"
        targets:
          linux: "/usr/share/libalpm/hooks"

  - name: "systemd-services"
    description: "System-level services"
    sudo: true
    when: '{{ and (eq .OS "linux") (eq .Hostname "my-server") }}'
    entries:
      - name: "services"
        files: ["backup.service", "backup.timer"]
        backup: "./systemd"
        targets:
          linux: "/etc/systemd/system"
```

## Next steps

- [Multi-Machine Setups](multi-machine-setups.md) -- using `when` expressions for per-machine configs
- [Git Repositories](git-repositories.md) -- sudo git clones for system-level repos
- [Interactive TUI](interactive-tui.md) -- select which system configs to restore interactively
