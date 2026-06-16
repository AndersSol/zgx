package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/discovery"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(discoverCmd())
}

func discoverCmd() *cobra.Command {
	var timeoutSeconds int

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Finn ZGX-enheter på nettet (mDNS)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			devices, err := discovery.DiscoverTimeout(time.Duration(timeoutSeconds) * time.Second)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(devices) == 0 {
				fmt.Fprintln(out, "Ingen ZGX-enheter funnet.")
				return nil
			}

			for _, device := range devices {
				fmt.Fprintf(out, "%s  %s:%d  (%s)\n",
					device.Hostname,
					strings.Join(device.Addresses, ","),
					device.Port,
					device.Name,
				)
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 5, "Hvor lenge discovery skal kjøre, i sekunder")
	return cmd
}
