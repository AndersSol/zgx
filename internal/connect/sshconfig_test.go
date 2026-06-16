package connect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostBlockContainsMarkersAndFields(t *testing.T) {
	block := HostBlock("zgx-abc123", "zgx-abc123.local", "anders", 22, "~/.ssh/id_ed25519")

	for _, want := range []string{
		"# >>> zgx managed: zgx-abc123 >>>",
		"Host zgx-abc123",
		"    HostName zgx-abc123.local",
		"    User anders",
		"    Port 22",
		"    IdentityFile ~/.ssh/id_ed25519",
		"    IdentitiesOnly yes",
		"# <<< zgx managed: zgx-abc123 <<<",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("HostBlock missing %q in:\n%s", want, block)
		}
	}
}

func TestWriteHostConfigCreatesConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".ssh", "config")

	if err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local", "anders", 22, "~/.ssh/id_ed25519"); err != nil {
		t.Fatalf("WriteHostConfig() error = %v", err)
	}

	assertFileMode(t, configPath, 0o600)
	content := readString(t, configPath)
	if !strings.Contains(content, "Host zgx-abc123") {
		t.Fatalf("config missing Host block:\n%s", content)
	}
}

func TestWriteHostConfigIsIdempotent(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".ssh", "config")

	for i := 0; i < 2; i++ {
		if err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local", "anders", 22, "~/.ssh/id_ed25519"); err != nil {
			t.Fatalf("WriteHostConfig() call %d error = %v", i+1, err)
		}
	}

	content := readString(t, configPath)
	if got := strings.Count(content, "# >>> zgx managed: zgx-abc123 >>>"); got != 1 {
		t.Fatalf("start marker count = %d, want 1\n%s", got, content)
	}
	if got := strings.Count(content, "# <<< zgx managed: zgx-abc123 <<<"); got != 1 {
		t.Fatalf("end marker count = %d, want 1\n%s", got, content)
	}
}

func TestWriteHostConfigUpdatesExistingManagedBlock(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".ssh", "config")

	if err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local", "anders", 22, "~/.ssh/id_ed25519"); err != nil {
		t.Fatalf("WriteHostConfig(port 22) error = %v", err)
	}
	if err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local", "anders", 2222, "~/.ssh/id_ed25519"); err != nil {
		t.Fatalf("WriteHostConfig(port 2222) error = %v", err)
	}

	content := readString(t, configPath)
	if !strings.Contains(content, "    Port 2222") {
		t.Fatalf("config missing updated port:\n%s", content)
	}
	if strings.Contains(content, "    Port 22\n") {
		t.Fatalf("config still contains old port:\n%s", content)
	}
	if got := strings.Count(content, "# >>> zgx managed: zgx-abc123 >>>"); got != 1 {
		t.Fatalf("start marker count = %d, want 1\n%s", got, content)
	}
}

func TestWriteHostConfigPreservesUnrelatedConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".ssh", "config")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	existing := "Host github.com\n  User git\n"
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile(existing) error = %v", err)
	}

	if err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local", "anders", 22, "~/.ssh/id_ed25519"); err != nil {
		t.Fatalf("WriteHostConfig() error = %v", err)
	}

	content := readString(t, configPath)
	if !strings.Contains(content, existing) {
		t.Fatalf("config did not preserve unrelated lines:\n%s", content)
	}
}

func TestWriteHostConfigRejectsNewlineInjection(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".ssh", "config")
	existing := "Host github.com\n  User git\n"
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile(existing) error = %v", err)
	}

	err := WriteHostConfig(configPath, "zgx-abc123", "zgx-abc123.local\n  ProxyCommand evil", "anders", 22, "~/.ssh/id_ed25519")
	if err == nil {
		t.Fatal("WriteHostConfig() returned nil for newline injection")
	}

	content := readString(t, configPath)
	if content != existing {
		t.Fatalf("config changed after rejected injection:\n%s", content)
	}
	if strings.Contains(content, "ProxyCommand") {
		t.Fatalf("config contains injected ProxyCommand:\n%s", content)
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}
