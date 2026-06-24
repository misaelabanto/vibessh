package keys

import (
	"os"
	"path/filepath"
	"testing"
)

// stubGenerate replaces generateKey for a test, writing fake key files, and
// restores the original afterward. It returns a pointer to the call counter.
func stubGenerate(t *testing.T) *int {
	t.Helper()
	calls := 0
	original := generateKey
	generateKey = func(privPath, comment string) error {
		calls++
		if err := os.WriteFile(privPath, []byte("PRIVATE"), 0600); err != nil {
			return err
		}
		return os.WriteFile(privPath+".pub", []byte("ssh-ed25519 AAAATEST "+comment+"\n"), 0644)
	}
	t.Cleanup(func() { generateKey = original })
	return &calls
}

func TestEnsureKeyGeneratesWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := stubGenerate(t)

	privPath, err := EnsureKey()
	if err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}

	want := filepath.Join(home, ".vibessh", "keys", "id_ed25519")
	if privPath != want {
		t.Errorf("path = %q, want %q", privPath, want)
	}
	if *calls != 1 {
		t.Errorf("generateKey called %d times, want 1", *calls)
	}
	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not created: %v", err)
	}

	info, err := os.Stat(filepath.Dir(privPath))
	if err != nil {
		t.Fatalf("stat keys dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("keys dir perm = %o, want 700", perm)
	}
}

func TestEnsureKeyIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	keysPath := filepath.Join(home, ".vibessh", "keys")
	if err := os.MkdirAll(keysPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keysPath, "id_ed25519"), []byte("PRIVATE"), 0600); err != nil {
		t.Fatal(err)
	}

	original := generateKey
	generateKey = func(privPath, comment string) error {
		t.Fatalf("generateKey should not be called when key exists")
		return nil
	}
	t.Cleanup(func() { generateKey = original })

	if _, err := EnsureKey(); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
}

func TestPublicKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubGenerate(t)

	if _, err := EnsureKey(); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}

	pub, err := PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if pub != "ssh-ed25519 AAAATEST vibessh@"+hostnameOrEmpty() {
		// Hostname is environment-dependent; only assert the stable prefix.
		if got := pub[:len("ssh-ed25519 AAAATEST")]; got != "ssh-ed25519 AAAATEST" {
			t.Errorf("PublicKey = %q, want ssh-ed25519 prefix", pub)
		}
	}
	if pub != "" && pub[len(pub)-1] == '\n' {
		t.Errorf("PublicKey should be trimmed, got trailing newline")
	}
}

func hostnameOrEmpty() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}
