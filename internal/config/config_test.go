package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zgx", "config.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returnerte feil for fraværende fil: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returnerte nil config")
	}
	if len(cfg.Devices) != 0 {
		t.Fatalf("Load fraværende fil ga %d enheter, vil ha 0", len(cfg.Devices))
	}
}

func TestSaveLoadRoundTripAndFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zgx", "config.json")
	want := &Config{Devices: []Device{
		{Alias: "zgx-a", Host: "zgx-a.local", User: "root", Port: 22},
		{Alias: "zgx-b", Host: "zgx-b.local", User: "hp", Port: 2222, Identity: "/tmp/id"},
	}}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save feilet: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat config-fil feilet: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("fil-perms = %o, vil ha 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat config-mappe feilet: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("mappe-perms = %o, vil ha 700", got)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load feilet: %v", err)
	}
	if len(got.Devices) != 2 {
		t.Fatalf("Load ga %d enheter, vil ha 2", len(got.Devices))
	}
	if got.Devices[0] != want.Devices[0] || got.Devices[1] != want.Devices[1] {
		t.Fatalf("round-trip mismatch: got %#v want %#v", got.Devices, want.Devices)
	}
}

func TestLoadCorruptJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{ ikke json"), 0o600); err != nil {
		t.Fatalf("skriv korrupt config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load returnerte nil-feil for korrupt JSON")
	}
}

func TestUpsertSameAliasUpdatesWithoutDuplicate(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Device{Alias: "nano", Host: "old.local", User: "hp", Port: 22})
	cfg.Upsert(Device{Alias: "nano", Host: "new.local", User: "root", Port: 2222, Identity: "/tmp/id"})

	if len(cfg.Devices) != 1 {
		t.Fatalf("Upsert ga %d enheter, vil ha 1", len(cfg.Devices))
	}
	got, ok := cfg.Get("nano")
	if !ok {
		t.Fatal("Get fant ikke oppdatert alias")
	}
	if got.Host != "new.local" || got.User != "root" || got.Port != 2222 || got.Identity != "/tmp/id" {
		t.Fatalf("Upsert oppdaterte ikke enheten: %#v", got)
	}
}

func TestRemove(t *testing.T) {
	cfg := &Config{}
	cfg.Upsert(Device{Alias: "nano", Host: "nano.local", User: "hp", Port: 22})

	if !cfg.Remove("nano") {
		t.Fatal("Remove eksisterende alias returnerte false")
	}
	if _, ok := cfg.Get("nano"); ok {
		t.Fatal("Remove fjernet ikke alias")
	}
	if cfg.Remove("missing") {
		t.Fatal("Remove ikke-eksisterende alias returnerte true")
	}
}
