# Passwordless key auth for vibessh

Date: 2026-06-24
Status: Approved design, pending implementation plan

## Goal

Let the user log into a configured host without typing a password, set up
automatically. The user types the host password exactly once (the first time
they connect to a host that is not yet set up); every connection after that is
passwordless and prompt-free. No commands to remember: the user just selects a
host in the TUI and vibessh handles the key setup.

## Background

vibessh today reads `~/.vibessh/hosts.yaml`, shows a bubbletea picker, and on
selection calls `syscall.Exec` to replace itself with `ssh`, passing
ControlMaster flags. The control sockets in `~/.vibessh/ctrl/` make reconnects
within 10 minutes passwordless, but that window expires and the first connection
of each session still prompts for a password. This feature adds permanent
passwordless auth via an SSH public key.

Because `Connect` uses `syscall.Exec` (it replaces the vibessh process with
`ssh` and cannot run anything afterward), all setup must happen before the
hand-off.

## Decisions (locked with the user)

- **Trigger:** auto on connect. When the user picks a host that still needs a
  password, vibessh notices, asks for the password once, installs the key, then
  connects. No separate command.
- **Key source:** a vibessh-managed key in `~/.vibessh/keys/`. vibessh never
  touches `~/.ssh`.
- **Passphrase:** none. True zero-typing after the one-time setup. The private
  key file (mode `0600`) is itself the credential.
- **Key type:** ed25519 by default. RSA-4096 is a one-line package constant
  change if a legacy server ever requires it.
- **Detection/install mechanism:** Approach A, stateless probe via master-open
  plus a native key install (no `ssh-copy-id` dependency, no config schema
  change, self-healing).

## Architecture

### New package: `internal/keys`

Owns the vibessh-managed key.

- `EnsureKey() (privPath string, err error)`: if
  `~/.vibessh/keys/id_ed25519` is missing, generate it via
  `ssh-keygen -t ed25519 -N "" -C "vibessh@<localhostname>" -f <path>`.
  Creates `~/.vibessh/keys/` at `0700` and the private key at `0600`. Returns
  the private key path.
- `PublicKey() (string, error)`: reads `id_ed25519.pub`.

The key type lives in a package constant so switching to RSA-4096 is a
one-line change. `ssh-keygen` ships with the same OpenSSH package as `ssh`, so
this adds no new runtime dependency.

### Extended package: `internal/ssh`

A setup step is inserted before the `syscall.Exec` hand-off in the connect flow.

- `ensureKeyAuth(target string, extraArgs []string) error`: orchestrator.
  Ensures the key exists, opens the master via key-only auth, installs the key
  on demand.
- `openMaster(target, keyPath string, extraArgs []string) error`: runs
  `ssh -i <key> -o IdentitiesOnly=yes -o BatchMode=yes
  -o PreferredAuthentications=publickey -o ConnectTimeout=8 <ControlMaster flags>
  -fN <target>`. Opens (or reuses) the background master socket using only the
  key. Captures stderr and returns a classified error.
- `installPublicKey(target string, extraArgs []string) error`: the native
  `ssh-copy-id` replacement. Runs an interactive (password-auth) ssh that
  appends the pubkey to the remote `~/.ssh/authorized_keys` with correct perms,
  deduping if the key is already present. Forces
  `PreferredAuthentications=password,keyboard-interactive` and
  `PubkeyAuthentication=no` so it goes straight to the password prompt and never
  trips "too many authentication failures". Uses `ControlPath=none` so this
  one-shot does not pollute the master socket. This is where the user types the
  password, once.
- `classifyFailure(exitCode int, stderr string) failureKind`: pure function
  mapping ssh's result to `ok` / `authDenied` / `unreachable`. Scans stderr:
  `Permission denied` => `authDenied`; `Connection refused` / `timed out` /
  `Could not resolve` => `unreachable`.

### Modified: `Connect` / `ConnectRaw`

- `Connect` (configured hosts) runs `ensureKeyAuth` before the hand-off, then
  `syscall.Exec`s `ssh -i <keyPath> -o IdentitiesOnly=yes <ControlMaster flags>
  <target>`, reusing the live master socket (instant, no auth).
- `ConnectRaw` (off-config fallback targets) gets `-i <key>` so it benefits when
  the key is already installed, but skips the auto-install probe. vibessh does
  not silently push keys onto arbitrary throwaway hosts.

### Config

No change to `hosts.yaml`. The flow is stateless and self-healing: if a server
is rebuilt and the key is gone, the next connect simply re-installs it.

## Data flow: selecting a configured host

```
pick host in TUI
   |
   v
keys.EnsureKey()                  generate managed key if first run
   |
   v
openMaster(key-only, BatchMode)  -- ok ---------------> hand off
   |                                                       |
   | authDenied                          unreachable       |
   v                                          |            |
installPublicKey (password once)             v            |
   |                              print ssh's real error,  |
   v                              do NOT prompt for pw      |
openMaster(key-only) again                                 |
   |                                                        |
   +-----------------------------------------------------> syscall.Exec
                                                            ssh -i key
                                                            <CM flags> target
                                                            (reuses live master,
                                                             instant, no auth)
```

The master-open is both the probe and the connection warm-up, so there is no
throwaway round-trip. Within the 10-minute persist window the socket already
exists, so master-open returns instantly and the user goes straight to the
session.

## Error handling

| Situation | Behavior |
|---|---|
| `ssh-keygen` missing / keygen fails | Abort with `could not generate vibessh key: <err>`. No connect. |
| Master opens with key | Proceed to session. |
| Master fails, auth denied | Trigger the install flow (password prompt). |
| Master fails, host unreachable | Do not prompt for a password. Print ssh's real error and abort. |
| Install succeeds, re-open still auth-denied | `key installed but auth still failing - check remote ~/.ssh perms`. Abort. |
| Install fails: server refuses password auth (pubkey-only, none installed) | Print the pubkey plus instructions to paste it into `~/.ssh/authorized_keys` manually. The one genuine dead-end, made actionable. |
| Install fails: wrong password (ssh gives up) | `authentication failed; key not installed`. Abort, do not exec. |

## Testing

- `classifyFailure`: pure unit tests over representative stderr strings and exit
  codes.
- Arg builders (`masterArgs`, install args, modified connect args): assert the
  expected flags are present. Mirrors the existing `ConnectFlags` test.
- `internal/keys`: test path and perms logic with a temp `HOME`; the
  `ssh-keygen` exec sits behind a stubbable func var so tests assert dir/file
  creation without running it. `PublicKey` reads the `.pub`.
- Live SSH round-trips (`openMaster`, `installPublicKey`) stay behind thin
  functions and are not unit-tested.
- Manual verification: connect to a real host, confirm one password prompt then
  passwordless after; then wipe the remote `authorized_keys` and confirm the
  next connect self-heals.
- Gate before done: `go build ./...`, `go vet ./...`, `go test ./...`.

## Out of scope

- Key rotation / revocation.
- Per-host distinct keys (a single managed key is reused across hosts).
- ssh-agent and passphrase-protected keys.
- A manual "re-run setup" keybinding (the auto flow is self-healing, so it is
  unnecessary).
```
