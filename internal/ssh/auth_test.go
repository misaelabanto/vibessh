package ssh

import (
	"strings"
	"testing"
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
