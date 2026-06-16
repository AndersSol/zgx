package connect

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyPair peker på en generert (eller eksisterende) ed25519-nøkkel på disk.
type KeyPair struct {
	PrivateKeyPath string
	PublicKeyPath  string
	PublicKeyLine  string
}

// GenerateKeyPair sikrer at et ed25519-nøkkelpar finnes i sshDir (typisk ~/.ssh).
// IDEMPOTENT + SIKKERHET: hvis privatnøkkelen ALLEREDE finnes, returner den
// eksisterende UTEN å regenerere/overskrive (å overskrive ville foreldreløst-gjøre
// nøkler som alt er lagt i remote authorized_keys). comment settes på pub-linja
// (f.eks. "zgx" eller user@host).
func GenerateKeyPair(sshDir, keyName, comment string) (KeyPair, error) {
	if keyName == "" {
		keyName = "id_ed25519"
	}

	privatePath := filepath.Join(sshDir, keyName)
	publicPath := privatePath + ".pub"
	pair := KeyPair{PrivateKeyPath: privatePath, PublicKeyPath: publicPath}

	if err := ensureSecureDir(sshDir); err != nil {
		return KeyPair{}, err
	}

	if _, err := os.Stat(privatePath); err == nil {
		publicLine, err := existingOrRegeneratedPublicKeyLine(privatePath, publicPath, comment)
		if err != nil {
			return KeyPair{}, err
		}
		pair.PublicKeyLine = publicLine
		return pair, nil
	} else if !os.IsNotExist(err) {
		return KeyPair{}, fmt.Errorf("stat private key %q: %w", privatePath, err)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate ed25519 key: %w", err)
	}
	privateBlock, err := ssh.MarshalPrivateKey(privateKey, comment)
	if err != nil {
		return KeyPair{}, fmt.Errorf("marshal private key: %w", err)
	}
	privateBytes := pem.EncodeToMemory(privateBlock)
	if privateBytes == nil {
		return KeyPair{}, fmt.Errorf("encode private key PEM: empty output")
	}
	if err := os.WriteFile(privatePath, privateBytes, 0o600); err != nil {
		return KeyPair{}, fmt.Errorf("write private key %q: %w", privatePath, err)
	}
	if err := os.Chmod(privatePath, 0o600); err != nil {
		return KeyPair{}, fmt.Errorf("chmod private key %q: %w", privatePath, err)
	}

	publicLine, err := publicKeyLineFromPrivate(privateKey, comment)
	if err != nil {
		return KeyPair{}, err
	}
	if err := writePublicKey(publicPath, publicLine); err != nil {
		return KeyPair{}, err
	}

	pair.PublicKeyLine = publicLine
	return pair, nil
}

func existingOrRegeneratedPublicKeyLine(privatePath, publicPath, comment string) (string, error) {
	publicBytes, err := os.ReadFile(publicPath)
	if err == nil {
		line := strings.TrimSpace(string(publicBytes))
		if line == "" {
			return "", fmt.Errorf("public key %q is empty", publicPath)
		}
		return line, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read public key %q: %w", publicPath, err)
	}

	privateBytes, err := os.ReadFile(privatePath)
	if err != nil {
		return "", fmt.Errorf("read private key %q to regenerate public key: %w", privatePath, err)
	}
	rawPrivate, err := ssh.ParseRawPrivateKey(privateBytes)
	if err != nil {
		return "", fmt.Errorf("parse private key %q to regenerate public key: %w", privatePath, err)
	}
	publicLine, err := publicKeyLineFromRawPrivate(rawPrivate, comment)
	if err != nil {
		return "", fmt.Errorf("regenerate public key from %q: %w", privatePath, err)
	}
	if err := writePublicKey(publicPath, publicLine); err != nil {
		return "", err
	}
	return publicLine, nil
}

func publicKeyLineFromRawPrivate(rawPrivate any, comment string) (string, error) {
	switch privateKey := rawPrivate.(type) {
	case ed25519.PrivateKey:
		return publicKeyLineFromPrivate(privateKey, comment)
	case *ed25519.PrivateKey:
		return publicKeyLineFromPrivate(*privateKey, comment)
	default:
		return "", fmt.Errorf("private key is %T, want ed25519.PrivateKey", rawPrivate)
	}
}

func publicKeyLineFromPrivate(privateKey ed25519.PrivateKey, comment string) (string, error) {
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return "", fmt.Errorf("wrap public key: %w", err)
	}
	return appendAuthorizedKeyComment(ssh.MarshalAuthorizedKey(publicKey), comment), nil
}

func appendAuthorizedKeyComment(publicKey []byte, comment string) string {
	line := strings.TrimSpace(string(publicKey))
	if comment != "" {
		line += " " + comment
	}
	return line
}

func writePublicKey(path, publicLine string) error {
	if err := os.WriteFile(path, []byte(publicLine+"\n"), 0o644); err != nil {
		return fmt.Errorf("write public key %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("chmod public key %q: %w", path, err)
	}
	return nil
}

func ensureSecureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create directory %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("chmod directory %q: %w", path, err)
	}
	return nil
}
