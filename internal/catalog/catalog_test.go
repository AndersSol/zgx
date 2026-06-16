package catalog

import "testing"

func TestLoad(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("Load() returnerte ingen kategorier")
	}
}

func TestAllApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}

	apps := AllApps(cats)
	if got, want := len(apps), 17; got != want {
		t.Fatalf("len(AllApps(...)) = %d, vil ha %d", got, want)
	}
}

func TestByIDKnownApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}

	for _, id := range []string{"base-system", "ollama", "zgx-python-env", "poetry"} {
		if _, ok := ByID(cats, id); !ok {
			t.Errorf("ByID(..., %q) fant ikke appen", id)
		}
	}
}

func TestCommandsArePresent(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}

	for _, app := range AllApps(cats) {
		if app.InstallCommand == "" {
			t.Errorf("%s mangler installCommand", app.ID)
		}
		if app.VerifyCommand == "" {
			t.Errorf("%s mangler verifyCommand", app.ID)
		}
	}
}

func TestDependenciesReferToExistingApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}

	for _, app := range AllApps(cats) {
		for _, depID := range app.Dependencies {
			if _, ok := ByID(cats, depID); !ok {
				t.Errorf("%s har ukjent dependency %q", app.ID, depID)
			}
		}
	}
}

func TestUninstallCommandSemantics(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returnerte feil: %v", err)
	}

	base, ok := ByID(cats, "base-system")
	if !ok {
		t.Fatal("base-system finnes ikke")
	}
	if base.UninstallCommand != nil {
		t.Fatal("base-system skal ha nil UninstallCommand")
	}

	ollama, ok := ByID(cats, "ollama")
	if !ok {
		t.Fatal("ollama finnes ikke")
	}
	if ollama.UninstallCommand == nil {
		t.Fatal("ollama skal ha UninstallCommand")
	}
}
