package ssh

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		name     string
		exitCode int
		stderr   string
		want     failureKind
	}{
		{"success", 0, "", connectOK},
		{"publickey denied", 255, "user@host: Permission denied (publickey).", connectAuthDenied},
		{"connection refused", 255, "ssh: connect to host x port 22: Connection refused", connectUnreachable},
		{"timed out", 255, "ssh: connect to host x port 22: Connection timed out", connectUnreachable},
		{"unknown host", 255, "ssh: Could not resolve hostname x: Name or service not known", connectUnreachable},
		{"host key changed", 255, "REMOTE HOST IDENTIFICATION HAS CHANGED! Host key verification failed.", connectUnreachable},
		{"unknown nonzero", 1, "something weird happened", connectUnreachable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFailure(tc.exitCode, tc.stderr); got != tc.want {
				t.Errorf("classifyFailure(%d, %q) = %v, want %v", tc.exitCode, tc.stderr, got, tc.want)
			}
		})
	}
}

func TestMasterArgs(t *testing.T) {
	args := masterArgs("user@host", "/keys/id_ed25519", "/ctrl", "/known_hosts", []string{"-p", "2222"})
	joined := strings.Join(args, " ")

	mustContainPair(t, args, "-i", "/keys/id_ed25519")
	mustContainOpt(t, joined, "IdentitiesOnly=yes")
	mustContainOpt(t, joined, "BatchMode=yes")
	mustContainOpt(t, joined, "PreferredAuthentications=publickey")
	mustContainOpt(t, joined, "ConnectTimeout=8")
	mustContainOpt(t, joined, "ControlMaster=auto")
	mustContainOpt(t, joined, "StrictHostKeyChecking=accept-new")
	mustContainOpt(t, joined, "UserKnownHostsFile=/known_hosts")

	if !strings.Contains(joined, "-p 2222") {
		t.Errorf("extra args missing: %q", joined)
	}
	if args[len(args)-1] != "user@host" {
		t.Errorf("target should be last arg, got %q", args[len(args)-1])
	}
	if args[len(args)-2] != "-fN" {
		t.Errorf("-fN should precede target, got %q", args[len(args)-2])
	}
}

func TestInstallArgs(t *testing.T) {
	args := installArgs("user@host", "/known_hosts", nil, "ssh-ed25519 AAAA vibessh@me")
	joined := strings.Join(args, " ")

	mustContainOpt(t, joined, "PubkeyAuthentication=no")
	mustContainOpt(t, joined, "PreferredAuthentications=password,keyboard-interactive")
	mustContainOpt(t, joined, "ControlPath=none")
	mustContainOpt(t, joined, "StrictHostKeyChecking=accept-new")

	// target and remote command should be the final two args.
	if args[len(args)-2] != "user@host" {
		t.Errorf("target should precede remote command, got %q", args[len(args)-2])
	}
	if !strings.Contains(args[len(args)-1], "authorized_keys") {
		t.Errorf("remote command missing: %q", args[len(args)-1])
	}
}

func TestRemoteInstallCommand(t *testing.T) {
	cmd := remoteInstallCommand("ssh-ed25519 AAAAKEY vibessh@me")

	// The snippet must run under a POSIX shell, not the remote login shell, so a
	// fish (or other non-POSIX) login shell never tries to parse its sh syntax.
	// sh is used rather than bash so minimal hosts (Alpine/BusyBox) still work.
	if !strings.HasPrefix(cmd, "sh -c ") {
		t.Errorf("remote command should run under sh, got:\n%s", cmd)
	}

	for _, want := range []string{
		"ssh-ed25519 AAAAKEY vibessh@me",
		"authorized_keys",
		"chmod 700 ~/.ssh",
		"chmod 600 ~/.ssh/authorized_keys",
		"grep -qxF",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("remote command missing %q in:\n%s", want, cmd)
		}
	}
}

func TestShellSingleQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no quotes", "echo hi", "'echo hi'"},
		{"single quote", "printf '%s'", `'printf '\''%s'\'''`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellSingleQuote(tc.in); got != tc.want {
				t.Errorf("shellSingleQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// syncBuffer is a bytes.Buffer safe for concurrent writes and reads, so a test
// can inspect streamed output while the probe command is still running.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// TestRunProbeStreamsStderrWhileBlocked covers the Tailscale SSH check-mode
// case: the server writes an SSO URL to stderr and then blocks indefinitely
// waiting for the user to visit it. The probe must show that output while it
// waits, not buffer it until the command exits.
func TestRunProbeStreamsStderrWhileBlocked(t *testing.T) {
	const notice = "# To authenticate, visit: https://login.tailscale.com/a/deadbeef"

	// `exec sleep` replaces the shell, so killing the process really ends the
	// probe; a forked sleep would keep the inherited stderr pipe open and hang
	// cmd.Run past the kill.
	cmd := exec.Command("sh", "-c", "printf '%s\\n' "+shellSingleQuote(notice)+" >&2; exec sleep 5")
	live := &syncBuffer{}

	done := make(chan string, 1)
	go func() {
		captured, _ := runProbe(cmd, live)
		done <- captured
	}()

	deadline := time.After(2 * time.Second)
	for !strings.Contains(live.String(), notice) {
		select {
		case <-deadline:
			t.Fatalf("stderr was not streamed while the probe was still blocked; live output so far: %q", live.String())
		case <-done:
			t.Fatalf("probe exited before the streaming assertion could run")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill probe: %v", err)
	}

	select {
	case captured := <-done:
		if !strings.Contains(captured, notice) {
			t.Errorf("captured stderr = %q, want it to contain %q", captured, notice)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runProbe did not return after the probe was killed")
	}
}

func mustContainOpt(t *testing.T, joined, opt string) {
	t.Helper()
	if !strings.Contains(joined, "-o "+opt) {
		t.Errorf("missing -o %s in: %s", opt, joined)
	}
}

func mustContainPair(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return
		}
	}
	t.Errorf("missing pair %s %s in %v", flag, value, args)
}
