package connect

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateKeyPairCreatesEd25519Pair(t *testing.T) {
	sshDir := t.TempDir()

	pair, err := GenerateKeyPair(sshDir, "", "zgx")
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if pair.PrivateKeyPath != filepath.Join(sshDir, "id_ed25519") {
		t.Errorf("PrivateKeyPath = %q, want default id_ed25519 path", pair.PrivateKeyPath)
	}
	if pair.PublicKeyPath != filepath.Join(sshDir, "id_ed25519.pub") {
		t.Errorf("PublicKeyPath = %q, want default id_ed25519.pub path", pair.PublicKeyPath)
	}
	assertFileMode(t, pair.PrivateKeyPath, 0o600)
	assertFileMode(t, pair.PublicKeyPath, 0o644)

	if !strings.HasPrefix(pair.PublicKeyLine, "ssh-ed25519 ") {
		t.Fatalf("PublicKeyLine = %q, want ssh-ed25519 prefix", pair.PublicKeyLine)
	}
	if !strings.Contains(pair.PublicKeyLine, " zgx") {
		t.Errorf("PublicKeyLine = %q, want comment", pair.PublicKeyLine)
	}
	if strings.HasSuffix(pair.PublicKeyLine, "\n") {
		t.Errorf("PublicKeyLine has trailing newline")
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pair.PublicKeyLine)); err != nil {
		t.Fatalf("PublicKeyLine is not parseable: %v", err)
	}
}

func TestGenerateKeyPairIsIdempotentAndDoesNotRegeneratePrivateKey(t *testing.T) {
	sshDir := t.TempDir()

	pair, err := GenerateKeyPair(sshDir, "zgx_ed25519", "zgx")
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	privateBefore, err := os.ReadFile(pair.PrivateKeyPath)
	if err != nil {
		t.Fatalf("ReadFile(private) error = %v", err)
	}

	if _, err := GenerateKeyPair(sshDir, "zgx_ed25519", "zgx"); err != nil {
		t.Fatalf("GenerateKeyPair() second call error = %v", err)
	}
	privateAfter, err := os.ReadFile(pair.PrivateKeyPath)
	if err != nil {
		t.Fatalf("ReadFile(private after) error = %v", err)
	}

	if !bytes.Equal(privateAfter, privateBefore) {
		t.Fatal("private key changed on second GenerateKeyPair call")
	}
}

func TestGenerateKeyPairRegeneratesMissingPublicKeyFromPrivateKey(t *testing.T) {
	sshDir := t.TempDir()

	pair, err := GenerateKeyPair(sshDir, "zgx_ed25519", "zgx")
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	privateBefore, err := os.ReadFile(pair.PrivateKeyPath)
	if err != nil {
		t.Fatalf("ReadFile(private) error = %v", err)
	}
	if err := os.Remove(pair.PublicKeyPath); err != nil {
		t.Fatalf("Remove(public) error = %v", err)
	}

	regenerated, err := GenerateKeyPair(sshDir, "zgx_ed25519", "zgx")
	if err != nil {
		t.Fatalf("GenerateKeyPair() after removing public error = %v", err)
	}
	privateAfter, err := os.ReadFile(pair.PrivateKeyPath)
	if err != nil {
		t.Fatalf("ReadFile(private after) error = %v", err)
	}
	if !bytes.Equal(privateAfter, privateBefore) {
		t.Fatal("private key changed while regenerating missing public key")
	}
	assertFileMode(t, regenerated.PublicKeyPath, 0o644)

	rawPrivate, err := ssh.ParseRawPrivateKey(privateAfter)
	if err != nil {
		t.Fatalf("ParseRawPrivateKey() error = %v", err)
	}
	privateKey, ok := rawPrivate.(*ed25519.PrivateKey)
	if !ok {
		t.Fatalf("private key type = %T, want *ed25519.PrivateKey", rawPrivate)
	}
	wantPublic, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatalf("NewPublicKey(private.Public()) error = %v", err)
	}
	gotPublic, _, _, _, err := ssh.ParseAuthorizedKey([]byte(regenerated.PublicKeyLine))
	if err != nil {
		t.Fatalf("ParseAuthorizedKey(regenerated) error = %v", err)
	}
	if !bytes.Equal(gotPublic.Marshal(), wantPublic.Marshal()) {
		t.Fatalf("regenerated public key does not match private key")
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %v, want %v", path, got, want)
	}
}
