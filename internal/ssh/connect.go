package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/misael/vibessh/internal/tailscale"
)

const (
	controlPersist = "10m"
	controlMaster  = "auto"
)

// controlDir returns the path to the ControlMaster socket directory.
func controlDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".vibessh", "ctrl"), nil
}

// ConnectFlags returns the SSH flags for ControlMaster reuse.
func ConnectFlags(ctrlDir string) []string {
	controlPath := filepath.Join(ctrlDir, "%r@%h:%p")
	return []string{
		"-o", "ControlMaster=" + controlMaster,
		"-o", "ControlPath=" + controlPath,
		"-o", "ControlPersist=" + controlPersist,
	}
}

// Connect SSHes into the given node using its Tailscale IP.
func Connect(node tailscale.Node) error {
	target := node.IP.String()
	if !node.IP.IsValid() {
		// Fall back to DNS name if no IP is available.
		if node.DNSName != "" {
			target = node.DNSName
		} else {
			target = node.Hostname
		}
	}
	return connectTo(target)
}

// ConnectRaw SSHes to an arbitrary target string (hostname, IP, user@host, etc.).
func ConnectRaw(target string) error {
	return connectTo(target)
}

func connectTo(target string) error {
	ctrlDir, err := controlDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ctrlDir, 0700); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}

	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH")
	}

	args := []string{"ssh"}
	args = append(args, ConnectFlags(ctrlDir)...)
	args = append(args, target)

	return syscall.Exec(sshBin, args, os.Environ())
}
