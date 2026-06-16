package cmd

import (
	"context"
	"fmt"

	"github.com/AndersSol/zgx/internal/connect"
	"github.com/AndersSol/zgx/internal/install"
	"github.com/AndersSol/zgx/internal/pairing"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(
		pairCmd(),
		unpairCmd(),
		pairDetailsCmd(),
	)
}

func pairCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "pair <host>",
		Short: "Par to enheter over ConnectX",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			nics, err := pairing.Pair(context.Background(), runner, password)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, "Konfigurerte ConnectX-NIC-er:")
			writeNICs(out, nics)
			fmt.Fprintln(out, "Netplan skrevet og anvendt.")
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func unpairCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "unpair <host>",
		Short: "Opphev ConnectX-paring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			if err := pairing.Unpair(context.Background(), runner, password); err != nil {
				return err
			}

			fmt.Fprintln(out, "ConnectX-konfigurasjon fjernet.")
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func pairDetailsCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "pair-details <host>",
		Short: "Vis paringsdetaljer og ConnectX-NIC-er",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			nics, err := pairing.PairDetails(context.Background(), runner)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(nics) == 0 {
				fmt.Fprintln(out, "Ingen ConnectX-NIC-er funnet.")
				return nil
			}
			writeNICs(out, nics)
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func buildPairRunner(host string, opts systemCommandOptions) (install.SSHRunner, error) {
	hostKey, err := connect.KnownHostsCallback(expandHome(opts.knownHosts))
	if err != nil {
		return install.SSHRunner{}, err
	}
	return install.SSHRunner{
		Target:         connect.Target{Host: host, User: opts.user, Port: opts.port},
		HostKey:        hostKey,
		PrivateKeyPath: expandHome(opts.identity),
	}, nil
}

func writeNICs(out interface{ Write([]byte) (int, error) }, nics []pairing.NIC) {
	for _, nic := range nics {
		ip := nic.IPv4Address
		if ip == "" {
			ip = "(ingen IP)"
		}
		fmt.Fprintf(out, "%s  %s\n", nic.LinuxDeviceName, ip)
	}
}
