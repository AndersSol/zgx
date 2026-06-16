// Package cmd implementerer zgx-CLI-en — en tynn front-end over motoren i
// internal/. Front-ender (CLI, TUI, fremtidig macOS-app) holder ingen logikk
// selv.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// rootCmd er basiskommandoen som kjøres som `zgx`.
var rootCmd = &cobra.Command{
	Use:     "zgx",
	Short:   "Konfigurer HP ZGX nano over SSH",
	Version: version,
	Long: `zgx oppdager, kobler til og konfigurerer HP ZGX nano-enheter over SSH.

Portabel CLI portet fra HPInc/ZGX-Toolkit (X11/MIT).`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute kjører root-kommandoen og returnerer feilen til main, som eier
// exit-koden. Holder kommando-kjøring testbar (ingen os.Exit her).
func Execute() error {
	return rootCmd.Execute()
}

// stubCmd bygger en ikke-implementert subkommando. Returnerer en feil slik at
// exit-koden er ærlig (ikke-implementert ≠ suksess) frem til den får logikk.
func stubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s: ikke implementert ennå", cmd.Name())
		},
	}
}
