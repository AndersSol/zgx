package connect

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestAuthorizedKeysCommand(t *testing.T) {
	publicKeyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI a'b"

	command := AuthorizedKeysCommand(publicKeyLine)
	if !strings.Contains(command, "grep -qxF") {
		t.Fatalf("command missing idempotent grep guard:\n%s", command)
	}
	if !strings.Contains(command, "'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI a'\\''b'") {
		t.Fatalf("command missing escaped key:\n%s", command)
	}
	if !strings.Contains(command, "a'\\''b") {
		t.Fatalf("apostrophe was not single-quote escaped:\n%s", command)
	}
	if gotAgain := AuthorizedKeysCommand(publicKeyLine); gotAgain != command {
		t.Fatalf("AuthorizedKeysCommand is not deterministic:\nfirst:  %s\nsecond: %s", command, gotAgain)
	}
}

func TestKnownHostsCallbackTOFU(t *testing.T) {
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	hostPub := testHostPublicKey(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

	cb, err := KnownHostsCallback(knownHostsPath)
	if err != nil {
		t.Fatalf("KnownHostsCallback() error = %v", err)
	}
	if err := cb("zgx-test:22", addr, hostPub); err != nil {
		t.Fatalf("first callback TOFU append error = %v", err)
	}

	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("ReadFile(known_hosts) error = %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(content)), "\n") + 1; lines != 1 {
		t.Fatalf("known_hosts line count = %d, want 1:\n%s", lines, content)
	}

	if err := cb("zgx-test:22", addr, hostPub); err != nil {
		t.Fatalf("second callback with same key error = %v", err)
	}

	otherHostPub := testHostPublicKey(t)
	if err := cb("zgx-test:22", addr, otherHostPub); err == nil {
		t.Fatal("callback accepted changed host key; want mismatch/MITM error")
	}
}

func TestKnownHostsConfirmRejectsUnknownWhenConfirmFalse(t *testing.T) {
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	hostPub := testHostPublicKey(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

	cb, err := KnownHostsCallbackWithConfirm(knownHostsPath, func(hostname, fingerprint string) (bool, error) {
		if hostname != "zgx-test:22" {
			t.Fatalf("confirm hostname = %q, want zgx-test:22", hostname)
		}
		if !strings.HasPrefix(fingerprint, "SHA256:") {
			t.Fatalf("confirm fingerprint = %q, want SHA256 prefix", fingerprint)
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("KnownHostsCallbackWithConfirm() error = %v", err)
	}

	if err := cb("zgx-test:22", addr, hostPub); err == nil {
		t.Fatal("callback accepted unknown host when confirm returned false")
	}
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("ReadFile(known_hosts) error = %v", err)
	}
	if strings.TrimSpace(string(content)) != "" {
		t.Fatalf("known_hosts was updated despite reject:\n%s", content)
	}
}

func TestKnownHostsConfirmAcceptsWhenConfirmTrue(t *testing.T) {
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	hostPub := testHostPublicKey(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

	cb, err := KnownHostsCallbackWithConfirm(knownHostsPath, func(hostname, fingerprint string) (bool, error) {
		if hostname != "zgx-test:22" {
			t.Fatalf("confirm hostname = %q, want zgx-test:22", hostname)
		}
		if !strings.HasPrefix(fingerprint, "SHA256:") {
			t.Fatalf("confirm fingerprint = %q, want SHA256 prefix", fingerprint)
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("KnownHostsCallbackWithConfirm() error = %v", err)
	}

	if err := cb("zgx-test:22", addr, hostPub); err != nil {
		t.Fatalf("callback rejected unknown host after confirm true: %v", err)
	}
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("ReadFile(known_hosts) error = %v", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		t.Fatal("known_hosts was not updated after confirm true")
	}
}

func TestKnownHostsConfirmStillRejectsMismatch(t *testing.T) {
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	hostPub := testHostPublicKey(t)
	otherHostPub := testHostPublicKey(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

	cb, err := KnownHostsCallbackWithConfirm(knownHostsPath, func(hostname, fingerprint string) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("KnownHostsCallbackWithConfirm() error = %v", err)
	}
	if err := cb("zgx-test:22", addr, hostPub); err != nil {
		t.Fatalf("initial callback error = %v", err)
	}

	confirmCalled := false
	cb, err = KnownHostsCallbackWithConfirm(knownHostsPath, func(hostname, fingerprint string) (bool, error) {
		confirmCalled = true
		return true, nil
	})
	if err != nil {
		t.Fatalf("KnownHostsCallbackWithConfirm(reload) error = %v", err)
	}
	if err := cb("zgx-test:22", addr, otherHostPub); err == nil {
		t.Fatal("callback accepted changed host key; want mismatch/MITM error")
	}
	if confirmCalled {
		t.Fatal("confirm was called for known-host mismatch")
	}
}

func TestFingerprintSHA256(t *testing.T) {
	fingerprint := FingerprintSHA256(testHostPublicKey(t))
	if !strings.HasPrefix(fingerprint, "SHA256:") {
		t.Fatalf("FingerprintSHA256() = %q, want SHA256 prefix", fingerprint)
	}
}

func testHostPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error = %v", err)
	}
	return signer.PublicKey()
}
