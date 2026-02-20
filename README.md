# vibessh

A terminal-native SSH client driven by a simple YAML config file.

## What it does

`vibessh` lists your configured hosts in an interactive TUI and SSHes into whichever one you pick. Subsequent connections within 10 minutes reuse the same SSH master socket, making reconnects near-instant.

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
# Interactive picker — arrow keys to navigate, Enter to connect, q to quit
vibessh

# Direct connect by hostname prefix
vibessh mymac

# Direct connect by exact address
vibessh homelinux.example.com

# Fall back to raw SSH for anything not in the config
vibessh user@somehost.com
```

## Requirements

- `~/.vibessh/hosts.yaml` must exist (see above)
- `ssh` must be in `PATH`

## How it works

1. Reads `~/.vibessh/hosts.yaml` to get your host list
2. Shows an interactive picker (built with [bubbletea](https://github.com/charmbracelet/bubbletea))
3. Calls `ssh` with ControlMaster flags, replacing the vibessh process entirely via `syscall.Exec`

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
