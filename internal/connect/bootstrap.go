package connect

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const defaultSSHTimeout = 15 * time.Second

// Target beskriver en SSH-endepunktadresse.
type Target struct {
	Host string
	User string
	Port int
}

// Addr returnerer host:port, med port 22 som default.
func (t Target) Addr() string {
	port := t.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(t.Host, fmt.Sprintf("%d", port))
}

// AuthorizedKeysCommand bygger en idempotent remote shell-kommando som legger
// publicKeyLine i ~/.ssh/authorized_keys bare hvis den ikke alt finnes.
func AuthorizedKeysCommand(publicKeyLine string) string {
	escaped := shellSingleQuote(publicKeyLine)
	return "mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && grep -qxF " + escaped + " ~/.ssh/authorized_keys || printf '%s\\n' " + escaped + " >> ~/.ssh/authorized_keys"
}

// Bootstrap kobler til target med passord-auth og installerer publicKeyLine.
func Bootstrap(ctx context.Context, t Target, password, publicKeyLine string, hostKey ssh.HostKeyCallback) error {
	config := sshClientConfig(ctx, t.User, []ssh.AuthMethod{ssh.Password(password)}, hostKey)
	return runSSHCommand(ctx, t, config, AuthorizedKeysCommand(publicKeyLine), "bootstrap authorized_keys")
}

// TestKeyAuth kobler til med privatnøkkel og kjører true.
func TestKeyAuth(ctx context.Context, t Target, privateKeyPath string, hostKey ssh.HostKeyCallback) error {
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("read private key %q: %w", privateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("parse private key %q: %w", privateKeyPath, err)
	}

	config := sshClientConfig(ctx, t.User, []ssh.AuthMethod{ssh.PublicKeys(signer)}, hostKey)
	return runSSHCommand(ctx, t, config, "true", "test key auth")
}

// FingerprintSHA256 returnerer OpenSSH-kompatibelt SHA256-fingerprint for key.
func FingerprintSHA256(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
}

// KnownHostsCallback verifiserer mot knownHostsPath med TOFU for ukjente hosts.
func KnownHostsCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	return KnownHostsCallbackWithConfirm(knownHostsPath, func(hostname, fingerprint string) (bool, error) {
		return true, nil
	})
}

// KnownHostsCallbackWithConfirm verifiserer mot knownHostsPath og spør confirm
// før en ukjent host legges til. Kjente host-key-mismatch avvises alltid.
func KnownHostsCallbackWithConfirm(knownHostsPath string, confirm func(hostname, fingerprint string) (bool, error)) (ssh.HostKeyCallback, error) {
	if confirm == nil {
		return nil, fmt.Errorf("known_hosts confirm callback mangler")
	}
	if dir := filepath.Dir(knownHostsPath); dir != "." && dir != "" {
		if err := ensureSecureDir(dir); err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open known_hosts %q: %w", knownHostsPath, err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close known_hosts %q: %w", knownHostsPath, err)
	}
	if err := os.Chmod(knownHostsPath, 0o600); err != nil {
		return nil, fmt.Errorf("chmod known_hosts %q: %w", knownHostsPath, err)
	}

	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %q: %w", knownHostsPath, err)
	}

	var mu sync.Mutex
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		mu.Lock()
		defer mu.Unlock()

		err := cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			accepted, confirmErr := confirm(hostname, FingerprintSHA256(key))
			if confirmErr != nil {
				return fmt.Errorf("bekreft ukjent SSH host %q: %w", hostname, confirmErr)
			}
			if !accepted {
				return fmt.Errorf("ukjent SSH host %q avvist", hostname)
			}
			if appendErr := appendKnownHost(knownHostsPath, hostname, key); appendErr != nil {
				return appendErr
			}
			nextCB, reloadErr := knownhosts.New(knownHostsPath)
			if reloadErr != nil {
				return fmt.Errorf("reload known_hosts %q after TOFU append: %w", knownHostsPath, reloadErr)
			}
			cb = nextCB
			return nil
		}

		return err
	}, nil
}

func sshClientConfig(ctx context.Context, user string, auth []ssh.AuthMethod, hostKey ssh.HostKeyCallback) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         timeoutFromContext(ctx),
	}
}

func timeoutFromContext(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return time.Nanosecond
		}
		return timeout
	}
	return defaultSSHTimeout
}

func runSSHCommand(ctx context.Context, t Target, config *ssh.ClientConfig, command, action string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s canceled before dial: %w", action, err)
	}

	client, err := ssh.Dial("tcp", t.Addr(), config)
	if err != nil {
		return fmt.Errorf("%s: dial %s@%s: %w", action, t.User, t.Addr(), err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("%s: create ssh session: %w", action, err)
	}
	defer session.Close()

	type result struct {
		output []byte
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- result{output: output, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = client.Close()
		return fmt.Errorf("%s canceled: %w", action, ctx.Err())
	case res := <-done:
		if res.err != nil {
			message := strings.TrimSpace(string(res.output))
			if message == "" {
				return fmt.Errorf("%s failed: %w", action, res.err)
			}
			return fmt.Errorf("%s failed: %w: %s", action, res.err, message)
		}
		return nil
	}
}

func appendKnownHost(knownHostsPath, host string, key ssh.PublicKey) error {
	file, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("append known host %q: %w", knownHostsPath, err)
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, knownhosts.Line([]string{knownhosts.Normalize(host)}, key)); err != nil {
		return fmt.Errorf("write known host %q: %w", knownHostsPath, err)
	}
	return nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
