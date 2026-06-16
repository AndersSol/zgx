package cmd

import (
	"io"
	"strings"
	"testing"
)

// cobra registrerer disse selv; de er ikke del av zgx sin kommando-flate.
var cobraBuiltins = map[string]bool{"help": true, "completion": true}

// TestRootSubcommandSetExact verifiserer at den registrerte kommando-flaten er
// EKSAKT den intenderte: hver forventet kommando finnes, og ingen uventede er
// registrert (utenom cobras innebygde). Settet utledes fra rootCmd.Commands()
// — ikke en selv-rapportert literal — så testen fanger både manglende og
// utilsiktet tilkomne kommandoer (drift i begge retninger).
func TestRootSubcommandSetExact(t *testing.T) {
	want := map[string]bool{
		"config":   true,
		"discover": true, "connect": true,
		"list": true, "install": true, "verify": true, "uninstall": true,
		"health": true, "dns-register": true,
		"pair": true, "unpair": true, "pair-details": true,
	}

	have := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		name := c.Name()
		if cobraBuiltins[name] {
			continue
		}
		have[name] = true
		if !want[name] {
			t.Errorf("uventet subkommando registrert: %q", name)
		}
	}

	for name := range want {
		if !have[name] {
			t.Errorf("root mangler subkommando %q", name)
		}
	}
}

// TestStubsReturnError forsvarer fortsatt ærlig-exit-invarianten fra stub-fasen:
// nå via en ekte kommando-feilsti som SKAL returnere en feil (→ exit≠0).
func TestStubsReturnError(t *testing.T) {
	rootCmd.SetArgs([]string{"pair-details"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("kommando-feilsti returnerte nil — skal returnere feil for exit≠0")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("uventet feilmelding fra manglende arg: %v", err)
	}
}
