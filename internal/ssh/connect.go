package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/misaelabanto/vibessh/internal/hosts"
	"github.com/misaelabanto/vibessh/internal/keys"
)

const (
	controlPersist = "10m"
	controlMaster  = "auto"
	connectTimeout = "8"
)

// vibesshDir returns the ~/.vibessh base directory.
func vibesshDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".vibessh"), nil
}

// controlDir returns the path to the ControlMaster socket directory.
func controlDir() (string, error) {
	dir, err := vibesshDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ctrl"), nil
}

// knownHostsFile returns vibessh's own known_hosts path, kept out of ~/.ssh.
func knownHostsFile() (string, error) {
	dir, err := vibesshDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// controlFlags returns the SSH flags for ControlMaster reuse.
func controlFlags(ctrlDir string) []string {
	controlPath := filepath.Join(ctrlDir, "%r@%h:%p")
	return []string{
		"-o", "ControlMaster=" + controlMaster,
		"-o", "ControlPath=" + controlPath,
		"-o", "ControlPersist=" + controlPersist,
	}
}

// hostKeyFlags keep host-key trust inside ~/.vibessh and auto-accept new hosts,
// while still rejecting a host whose key has changed.
func hostKeyFlags(knownHosts string) []string {
	return []string{
		"-o", "UserKnownHostsFile=" + knownHosts,
		"-o", "StrictHostKeyChecking=accept-new",
	}
}

// nodeTarget builds the ssh target string and any port flag for a node.
func nodeTarget(node hosts.Node) (target string, portArgs []string) {
	target = node.Address
	if node.User != "" {
		target = node.User + "@" + target
	}
	if node.Port != 0 && node.Port != 22 {
		portArgs = []string{"-p", fmt.Sprintf("%d", node.Port)}
	}
	return target, portArgs
}

// Connect SSHes into the given node, setting up passwordless key auth first so
// the interactive session never asks for a password.
func Connect(node hosts.Node) error {
	target, portArgs := nodeTarget(node)

	keyPath, err := ensureKeyAuth(target, portArgs)
	if err != nil {
		return err
	}

	return execSession(target, keyPath, portArgs)
}

// ConnectRaw SSHes to an arbitrary target string (hostname, IP, user@host, etc.).
// It uses the managed key when one is available so an already-set-up host stays
// passwordless, but it does not run the auto-install probe against arbitrary
// hosts.
func ConnectRaw(target string) error {
	keyPath, _ := keys.EnsureKey() // best effort; empty path means no identity
	return execSession(target, keyPath, nil)
}

// execSession replaces the current process with an interactive ssh session,
// reusing the (already-opened) ControlMaster socket.
func execSession(target, keyPath string, extraArgs []string) error {
	ctrlDir, err := controlDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ctrlDir, 0700); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}
	knownHosts, err := knownHostsFile()
	if err != nil {
		return err
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH")
	}

	args := []string{"ssh"}
	args = append(args, controlFlags(ctrlDir)...)
	args = append(args, hostKeyFlags(knownHosts)...)
	if keyPath != "" {
		args = append(args, "-i", keyPath, "-o", "IdentitiesOnly=yes")
	}
	args = append(args, extraArgs...)
	args = append(args, target)

	return syscall.Exec(sshBin, args, os.Environ())
}
