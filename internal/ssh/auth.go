package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/misaelabanto/vibessh/internal/keys"
)

// failureKind classifies the outcome of a key-only connection probe.
type failureKind int

const (
	connectOK failureKind = iota
	connectAuthDenied
	connectUnreachable
)

// classifyFailure maps an ssh exit code and stderr to a failure kind. Anything
// that is not a clear "permission denied" is treated as unreachable so vibessh
// never prompts for a password against a host it cannot characterize.
func classifyFailure(exitCode int, stderr string) failureKind {
	if exitCode == 0 {
		return connectOK
	}
	if strings.Contains(strings.ToLower(stderr), "permission denied") {
		return connectAuthDenied
	}
	return connectUnreachable
}

// ensureKeyAuth makes sure key-based auth works for target, installing the
// managed public key (one interactive password) when the host still requires a
// password. It returns the key path to use for the interactive session.
func ensureKeyAuth(target string, extraArgs []string) (string, error) {
	keyPath, err := keys.EnsureKey()
	if err != nil {
		return "", err
	}

	kind, stderr, err := openMaster(target, keyPath, extraArgs)
	switch kind {
	case connectOK:
		return keyPath, nil
	case connectUnreachable:
		if stderr != "" {
			fmt.Fprint(os.Stderr, stderr)
		}
		return "", fmt.Errorf("cannot reach %s", target)
	}

	// connectAuthDenied: install the key (one password), then retry.
	if err := installPublicKey(target, extraArgs); err != nil {
		return "", err
	}

	kind, stderr, _ = openMaster(target, keyPath, extraArgs)
	if kind != connectOK {
		if stderr != "" {
			fmt.Fprint(os.Stderr, stderr)
		}
		return "", fmt.Errorf("key installed but auth still failing - check remote ~/.ssh permissions")
	}
	return keyPath, nil
}

// openMaster opens (or reuses) the background ControlMaster socket using only
// key auth. It never prompts for a password (BatchMode), so a host that still
// needs the key fails fast and cleanly.
func openMaster(target, keyPath string, extraArgs []string) (failureKind, string, error) {
	ctrlDir, err := controlDir()
	if err != nil {
		return connectUnreachable, "", err
	}
	if err := os.MkdirAll(ctrlDir, 0700); err != nil {
		return connectUnreachable, "", fmt.Errorf("create control dir: %w", err)
	}
	knownHosts, err := knownHostsFile()
	if err != nil {
		return connectUnreachable, "", err
	}

	cmd := exec.Command("ssh", masterArgs(target, keyPath, ctrlDir, knownHosts, extraArgs)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			// ssh failed to start (e.g. not in PATH).
			return connectUnreachable, stderr.String(), runErr
		}
		exitCode = exitErr.ExitCode()
	}

	return classifyFailure(exitCode, stderr.String()), stderr.String(), runErr
}

// masterArgs builds the argument list for the key-only master-open probe.
func masterArgs(target, keyPath, ctrlDir, knownHosts string, extraArgs []string) []string {
	args := []string{
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "PreferredAuthentications=publickey",
		"-o", "ConnectTimeout=" + connectTimeout,
	}
	args = append(args, controlFlags(ctrlDir)...)
	args = append(args, hostKeyFlags(knownHosts)...)
	args = append(args, extraArgs...)
	args = append(args, "-fN", target)
	return args
}

// installPublicKey appends the managed public key to the remote
// authorized_keys over a one-time password-authenticated ssh session.
func installPublicKey(target string, extraArgs []string) error {
	pubKey, err := keys.PublicKey()
	if err != nil {
		return err
	}
	knownHosts, err := knownHostsFile()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%s is not set up for passwordless login yet.\n", target)
	fmt.Fprintln(os.Stderr, "Enter the host password once to install your vibessh key:")

	cmd := exec.Command("ssh", installArgs(target, knownHosts, extraArgs, pubKey)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Could not install the key automatically.")
		fmt.Fprintln(os.Stderr, "If this host only allows key auth, add this line to ~/.ssh/authorized_keys on it:")
		fmt.Fprintf(os.Stderr, "\n  %s\n\n", pubKey)
		return fmt.Errorf("key installation failed: %w", err)
	}
	return nil
}

// installArgs builds the argument list for the password-authenticated key
// install. It forces password auth and bypasses the ControlMaster socket so
// the one-shot install never becomes the persistent master.
func installArgs(target, knownHosts string, extraArgs []string, pubKey string) []string {
	args := []string{
		"-o", "PreferredAuthentications=password,keyboard-interactive",
		"-o", "PubkeyAuthentication=no",
		"-o", "ControlPath=none",
	}
	args = append(args, hostKeyFlags(knownHosts)...)
	args = append(args, extraArgs...)
	args = append(args, target, remoteInstallCommand(pubKey))
	return args
}

// remoteInstallCommand returns the command to run on the remote host that
// appends pubKey to authorized_keys with correct permissions, deduping if
// already present.
//
// The snippet uses POSIX sh syntax (brace grouping, &&/||) that non-POSIX login
// shells such as fish cannot parse. Since ssh executes a remote command through
// the user's login shell, we wrap the snippet in `bash -c '...'` so it always
// runs under bash regardless of which shell the remote account uses.
func remoteInstallCommand(pubKey string) string {
	script := fmt.Sprintf(
		"umask 077; mkdir -p ~/.ssh && touch ~/.ssh/authorized_keys && "+
			"{ grep -qxF %q ~/.ssh/authorized_keys || printf '%%s\\n' %q >> ~/.ssh/authorized_keys; } && "+
			"chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys",
		pubKey, pubKey,
	)
	return "bash -c " + shellSingleQuote(script)
}

// shellSingleQuote wraps s in single quotes for safe embedding in a remote
// command line, escaping any embedded single quotes. The '\'' sequence is
// understood by bash, sh, zsh, and fish alike, so the wrapped string survives
// the remote login shell intact before bash -c re-parses it.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
