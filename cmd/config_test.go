package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/AndersSol/zgx/internal/config"
)

func TestConfigCommandsUseXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var addOut bytes.Buffer
	add := configAddCmd()
	add.SetOut(&addOut)
	add.SetArgs([]string{"nano", "nano.local", "--user", "root", "--port", "2222", "--identity", "/tmp/id"})
	if err := add.Execute(); err != nil {
		t.Fatalf("config add feilet: %v", err)
	}
	if got := addOut.String(); got != "Lagret nano.\n" {
		t.Fatalf("config add output = %q", got)
	}

	path, err := configpkg.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath feilet: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("zgx", "config.json")) {
		t.Fatalf("DefaultPath = %q", path)
	}

	var listOut bytes.Buffer
	list := configListCmd()
	list.SetOut(&listOut)
	list.SetArgs([]string{})
	if err := list.Execute(); err != nil {
		t.Fatalf("config list feilet: %v", err)
	}
	if got := listOut.String(); got != "nano  root@nano.local:2222\n" {
		t.Fatalf("config list output = %q", got)
	}

	var removeOut bytes.Buffer
	remove := configRemoveCmd()
	remove.SetOut(&removeOut)
	remove.SetArgs([]string{"nano"})
	if err := remove.Execute(); err != nil {
		t.Fatalf("config remove feilet: %v", err)
	}
	if got := removeOut.String(); got != "Fjernet nano.\n" {
		t.Fatalf("config remove output = %q", got)
	}
}
