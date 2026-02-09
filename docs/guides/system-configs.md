# System Configs

Some configuration files live outside your home directory and require root/sudo access to manage. tidydots handles this through the `sudo` flag on individual entries, letting you manage system-level files like `/etc/hosts`, pacman hooks, and systemd units alongside your regular dotfiles.

## The `sudo` flag

The `sudo` flag tells tidydots to run symlink and file operations with elevated privileges. Set `sudo: true` on individual entries that need elevated access:

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
    Use `sudo: true` only on entries that actually need it. This gives you fine-grained control and makes it clear exactly which files require elevated access.

## Common examples

### /etc/hosts

Manage a custom hosts file for ad blocking or local development domains:

```yaml
applications:
  - name: "hosts-file"
    description: "Custom hosts file"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "hosts"
        files: ["hosts"]
        backup: "./system"
        sudo: true
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
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "hooks"
        files: ["cleanup.hook", "update-grub.hook"]
        backup: "./pacman/hooks"
        sudo: true
        targets:
          linux: "/usr/share/libalpm/hooks"
```

### Pacman configuration

```yaml
applications:
  - name: "pacman-config"
    description: "Pacman configuration"
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        files: ["pacman.conf"]
        backup: "./pacman"
        sudo: true
        targets:
          linux: "/etc"

      - name: "makepkg-conf"
        files: ["makepkg.conf"]
        backup: "./pacman"
        sudo: true
        targets:
          linux: "/etc"
```

### Systemd units

Manage custom systemd service files:

```yaml
applications:
  - name: "systemd-services"
    description: "Custom systemd service units"
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "backup-timer"
        files: ["backup.service", "backup.timer"]
        backup: "./systemd"
        sudo: true
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
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "sshd"
        files: ["sshd_config"]
        backup: "./ssh"
        sudo: true
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
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "pacman-conf"
        files: ["pacman.conf"]
        backup: "./arch/pacman"
        sudo: true
        targets:
          linux: "/etc"

      - name: "mkinitcpio"
        files: ["mkinitcpio.conf"]
        backup: "./arch"
        sudo: true
        targets:
          linux: "/etc"

  # Ubuntu-specific system configs
  - name: "ubuntu-system"
    description: "Ubuntu system configuration"
    when: '{{ eq .Distro "ubuntu" }}'
    entries:
      - name: "apt-sources"
        backup: "./ubuntu/apt"
        sudo: true
        targets:
          linux: "/etc/apt/sources.list.d"

  # Fedora-specific system configs
  - name: "fedora-system"
    description: "Fedora system configuration"
    when: '{{ eq .Distro "fedora" }}'
    entries:
      - name: "dnf-conf"
        files: ["dnf.conf"]
        backup: "./fedora"
        sudo: true
        targets:
          linux: "/etc/dnf"
```

## Combining with hostname filtering

For machine-specific system configs, combine `when` with hostname checks:

```yaml
applications:
  - name: "server-config"
    description: "Server-specific system configuration"
    when: '{{ and (eq .OS "linux") (eq .Hostname "my-server") }}'
    entries:
      - name: "nginx-conf"
        backup: "./server/nginx"
        sudo: true
        targets:
          linux: "/etc/nginx"

      - name: "fail2ban"
        backup: "./server/fail2ban"
        sudo: true
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
        sudo: true
        targets:
          linux: "/usr/share/libalpm/hooks"

    # Avoid: entire directory in a system path
    entries:
      - name: "hooks"
        backup: "./pacman/hooks"
        sudo: true
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
        when: '{{ eq .OS "linux" }}'
        entries:
          - name: "hosts"
            files: ["hosts"]
            backup: "./system"
            sudo: true
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
    when: '{{ eq .OS "linux" }}'
    entries:
      - name: "hosts-file"
        files: ["hosts"]
        backup: "./system"
        sudo: true
        targets:
          linux: "/etc"

  - name: "pacman-hooks"
    description: "Custom pacman hooks"
    when: '{{ eq .Distro "arch" }}'
    entries:
      - name: "hooks"
        files: ["cleanup.hook", "orphan-check.hook"]
        backup: "./pacman/hooks"
        sudo: true
        targets:
          linux: "/usr/share/libalpm/hooks"

  - name: "systemd-services"
    description: "System-level services"
    when: '{{ and (eq .OS "linux") (eq .Hostname "my-server") }}'
    entries:
      - name: "services"
        files: ["backup.service", "backup.timer"]
        backup: "./systemd"
        sudo: true
        targets:
          linux: "/etc/systemd/system"
```

## Next steps

- [Multi-Machine Setups](multi-machine-setups.md) -- using `when` expressions for per-machine configs
- [Git Repositories](git-repositories.md) -- sudo git clones for system-level repos
- [Interactive TUI](interactive-tui.md) -- select which system configs to restore interactively
