# vibessh

A terminal-native SSH client driven by a simple YAML config file.

## What it does

`vibessh` lists your configured hosts in an interactive TUI and SSHes into whichever one you pick. It sets up passwordless key auth automatically on first connect, so you type a host's password once and never again. Subsequent connections within 10 minutes also reuse the same SSH master socket, making reconnects near-instant.

## Setup

Create `~/.vibessh/hosts.yaml`:

```yaml
hosts:
  - hostname: mymac
    address: your-vps.com   # or any reachable address
    port: 2222              # optional, default 22
    user: misael            # optional
    os: darwin
  - hostname: homelinux
    address: homelinux.example.com
    user: misael
    os: linux
```

## Usage

```bash
# Interactive picker: arrow keys to navigate, Enter to connect, q to quit
# a add a host, e edit the selected host, d delete it (with confirmation)
vibessh

# Direct connect by hostname prefix
vibessh mymac

# Direct connect by exact address
vibessh homelinux.example.com

# Fall back to raw SSH for anything not in the config
vibessh user@somehost.com
```

## Passwordless login

The first time you connect to a host that still asks for a password, vibessh sets up key-based auth for you:

1. It generates a managed ed25519 key in `~/.vibessh/keys/` on first run (once, no passphrase). Your `~/.ssh` is never touched.
2. It installs the public key on the host over a single password-authenticated session. You type the host password once.
3. Every later connection uses the key, so you are never prompted again.

This is automatic: just pick the host. There is no command to remember and no `hosts.yaml` change. It is also self-healing: if you rebuild a server and the key is gone, the next connection re-installs it.

Host-key trust is kept in `~/.vibessh/known_hosts` (new hosts are accepted automatically, changed keys are still rejected), so vibessh stays out of `~/.ssh` entirely.

If a host only allows key auth and has no key installed yet, vibessh cannot install the key for you. It prints the public key so you can add it to the host's `~/.ssh/authorized_keys` manually.

## Requirements

- `~/.vibessh/hosts.yaml` must exist (see above)
- `ssh` and `ssh-keygen` (which ships with it) must be in `PATH`

## How it works

1. Reads `~/.vibessh/hosts.yaml` to get your host list
2. Shows an interactive picker (built with [bubbletea](https://github.com/charmbracelet/bubbletea))
3. Ensures key auth: opens a key-only ControlMaster connection, and if the host still needs a password, installs the managed public key over one password session before retrying
4. Calls `ssh` with ControlMaster flags, replacing the vibessh process entirely via `syscall.Exec`

SSH ControlMaster sockets are stored in `~/.vibessh/ctrl/`. A socket persists for 10 minutes after the last connection closes, so reconnecting to the same host is instant.

## Reaching machines behind NAT

If your home machine isn't directly reachable, a cheap SSH reverse tunnel via a VPS works well from any client including Termux on Android.

On the home machine, install `autossh` and create `~/.config/systemd/user/vibestunnel.service`:

```ini
[Unit]
Description=vibessh reverse tunnel
After=network-online.target

[Service]
ExecStart=autossh -M 0 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -o ExitOnForwardFailure=yes -N \
  -R 2222:localhost:22 YOU@your-vps.com
Restart=always

[Install]
WantedBy=default.target
```

```bash
systemctl --user enable --now vibestunnel
```

On the VPS add `GatewayPorts yes` to `/etc/ssh/sshd_config` and restart sshd. Then set `address: your-vps.com` and `port: 2222` in your hosts.yaml.

## Build

```bash
go mod tidy
go build -o vibessh .
```

## Install

```bash
go install github.com/misaelabanto/vibessh@latest
```
