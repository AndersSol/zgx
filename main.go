// Command zgx konfigurerer HP ZGX nano-enheter over SSH.
package main

import (
	"fmt"
	"os"

	"github.com/AndersSol/zgx/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "feil:", err)
		os.Exit(1)
	}
}
