// Package keys manages the vibessh-owned SSH key used for passwordless login.
// The key lives under ~/.vibessh/keys and is never placed in ~/.ssh.
package keys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// keyType selects the key algorithm. Switch to "rsa" for a legacy server that
// rejects ed25519; the filename and generation flags follow from this constant.
const keyType = "ed25519"

// keyFileName is the private key filename, derived from the key type
// (id_ed25519, id_rsa, ...).
var keyFileName = "id_" + keyType

// generateKey shells out to ssh-keygen. It is a package var so tests can stub
// it without invoking the real binary.
var generateKey = func(privPath, comment string) error {
	args := []string{"-t", keyType, "-N", "", "-C", comment, "-f", privPath}
	if keyType == "rsa" {
		args = append([]string{"-b", "4096"}, args...)
	}
	cmd := exec.Command("ssh-keygen", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// keysDir returns ~/.vibessh/keys.
func keysDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".vibessh", "keys"), nil
}

// privateKeyPath returns the path to the managed private key.
func privateKeyPath() (string, error) {
	dir, err := keysDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, keyFileName), nil
}

// EnsureKey returns the path to the vibessh-managed private key, generating a
// fresh passphrase-less key on first use.
func EnsureKey() (string, error) {
	privPath, err := privateKeyPath()
	if err != nil {
		return "", err
	}

	switch _, err := os.Stat(privPath); {
	case err == nil:
		return privPath, nil
	case !os.IsNotExist(err):
		return "", fmt.Errorf("stat key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(privPath), 0700); err != nil {
		return "", fmt.Errorf("create keys dir: %w", err)
	}

	comment := "vibessh"
	if host, err := os.Hostname(); err == nil && host != "" {
		comment = "vibessh@" + host
	}

	if err := generateKey(privPath, comment); err != nil {
		return "", fmt.Errorf("could not generate vibessh key: %w", err)
	}

	return privPath, nil
}

// PublicKey returns the contents of the managed public key, trimmed of trailing
// whitespace.
func PublicKey() (string, error) {
	privPath, err := privateKeyPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(privPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("read public key: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
