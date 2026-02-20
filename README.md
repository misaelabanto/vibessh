# vibessh

A terminal-native SSH client that uses Tailscale for node discovery.

## What it does

`vibessh` lists your Tailscale peers in an interactive TUI and SSHes into whichever one you pick. Subsequent connections within 10 minutes reuse the same SSH master socket, making reconnects near-instant.

## Usage

```bash
# Interactive picker — arrow keys to navigate, Enter to connect, q to quit
vibessh

# Direct connect by hostname prefix, DNS name prefix, or IP
vibessh mymac
vibessh 100.64.0.5
```

## Requirements

- Tailscale must be running (`tailscale up`)
- `ssh` must be in `PATH`
- You need read access to the Tailscale socket (`/var/run/tailscale/tailscaled.sock`)

## How it works

1. Queries the local Tailscale daemon via its Unix socket to get your peer list
2. Shows an interactive picker (built with [bubbletea](https://github.com/charmbracelet/bubbletea))
3. Calls `ssh` with ControlMaster flags, replacing the vibessh process entirely via `syscall.Exec`

SSH ControlMaster sockets are stored in `~/.vibessh/ctrl/`. A socket persists for 10 minutes after the last connection closes, so reconnecting to the same host is instant.

## Build

```bash
go mod tidy
go build -o vibessh .
```

## Install

```bash
go install github.com/misaelabanto/vibessh@latest
```
