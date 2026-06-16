package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AndersSol/zgx/internal/catalog"
)

func TestPipesToShell(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{command: "curl -fsSL x | sh", want: true},
		{command: "curl x | sudo sh", want: true},
		{command: "wget x | bash", want: true},
		{command: "curl -fsSL x -o install.sh && bash install.sh", want: true},
		{command: "sudo apt install -y x", want: false},
		{command: "curl x -o f", want: false},
	}

	for _, tt := range tests {
		if got := PipesToShell(tt.command); got != tt.want {
			t.Errorf("PipesToShell(%q) = %v, vil ha %v", tt.command, got, tt.want)
		}
	}
}

func TestRenderPlan(t *testing.T) {
	plan := RenderPlan("Installerer:", []PlanItem{
		{ID: "ollama", Command: "curl -fsSL x | sudo sh"},
		{ID: "curl", Command: "sudo apt install -y curl"},
	})

	for _, want := range []string{"ollama", "curl -fsSL x | sudo sh", "curl", "sudo apt install -y curl"} {
		if !strings.Contains(plan, want) {
			t.Fatalf("RenderPlan mangler %q:\n%s", want, plan)
		}
	}

	lines := strings.Split(plan, "\n")
	if !strings.Contains(lines[1], "⚠") {
		t.Fatalf("pipe-to-shell-linjen mangler markør: %q", lines[1])
	}
	if strings.Contains(lines[3], "⚠") {
		t.Fatalf("apt-linjen fikk uventet markør: %q", lines[3])
	}
}

func TestInstallPlanShownIncludesExpandedDeps(t *testing.T) {
	cats, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() feilet: %v", err)
	}

	items, err := installPlanItems(cats, []string{"jupyter-lab"})
	if err != nil {
		t.Fatalf("installPlanItems() feilet: %v", err)
	}

	plan := RenderPlan("Installerer:", items)
	for _, want := range []string{"miniforge", "curl -fsSL \"$MINIFORGE_URL\"", "⚠"} {
		if !strings.Contains(plan, want) {
			t.Fatalf("install-plan mangler %q:\n%s", want, plan)
		}
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "ja\n", want: true},
		{input: "JA\n", want: true},
		{input: "nei\n", want: false},
		{input: "y\n", want: false},
		{input: "", want: false},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		got, err := Confirm(strings.NewReader(tt.input), &out, "Skriv ja: ")
		if err != nil {
			t.Fatalf("Confirm(%q) returnerte feil: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("Confirm(%q) = %v, vil ha %v", tt.input, got, tt.want)
		}
		if out.String() != "Skriv ja: " {
			t.Errorf("Confirm skrev prompt %q, vil ha %q", out.String(), "Skriv ja: ")
		}
	}
}
