package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	connectpkg "github.com/AndersSol/zgx/internal/connect"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(connectCmd())
}

func connectCmd() *cobra.Command {
	var (
		user       string
		port       int
		alias      string
		knownHosts string
		password   string
	)

	cmd := &cobra.Command{
		Use:   "connect <host>",
		Short: "Sett opp SSH-nøkkeltilgang til en enhet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			if alias == "" {
				alias = host
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("finn hjemmemappe: %w", err)
			}
			sshDir := filepath.Join(homeDir, ".ssh")
			if knownHosts == "" {
				knownHosts = filepath.Join(sshDir, "known_hosts")
			}

			out := cmd.OutOrStdout()
			if password == "" {
				fmt.Fprintf(out, "Passord for %s@%s: ", user, host)
				passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(out)
				if err != nil {
					return fmt.Errorf("les passord: %w", err)
				}
				password = string(passwordBytes)
			}

			fmt.Fprintln(out, "Genererer eller gjenbruker SSH-nøkkel ...")
			keyPair, err := connectpkg.GenerateKeyPair(sshDir, "id_ed25519", fmt.Sprintf("%s@%s", user, host))
			if err != nil {
				return err
			}

			hostKey, err := connectpkg.KnownHostsCallbackWithConfirm(knownHosts, func(hostname, fingerprint string) (bool, error) {
				fmt.Fprintf(out, "Ukjent host %s. ED25519-fingerprint: %s. Stoler du på denne? Skriv ja: ", hostname, fingerprint)
				answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return false, err
				}
				return strings.TrimSpace(answer) == "ja", nil
			})
			if err != nil {
				return err
			}

			target := connectpkg.Target{Host: host, User: user, Port: port}
			fmt.Fprintf(out, "Legger til offentlig nøkkel på %s@%s ...\n", user, host)
			if err := connectpkg.Bootstrap(context.Background(), target, password, keyPair.PublicKeyLine, hostKey); err != nil {
				return err
			}

			configPath := filepath.Join(sshDir, "config")
			fmt.Fprintf(out, "Skriver SSH-konfigurasjon for alias %q ...\n", alias)
			if err := connectpkg.WriteHostConfig(configPath, alias, host, user, port, keyPair.PrivateKeyPath); err != nil {
				return err
			}

			fmt.Fprintln(out, "Tester nøkkel-tilgang ...")
			if err := connectpkg.TestKeyAuth(context.Background(), target, keyPair.PrivateKeyPath, hostKey); err != nil {
				return err
			}
			fmt.Fprintln(out, "Nøkkel-tilgang virker.")
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "hp", "SSH-bruker på enheten")
	cmd.Flags().IntVar(&port, "port", 22, "SSH-port på enheten")
	cmd.Flags().StringVar(&alias, "alias", "", "Host-alias i ~/.ssh/config (default: host)")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known_hosts-fil (default: ~/.ssh/known_hosts)")
	cmd.Flags().StringVar(&password, "password", "", "SSH-passord (promptes skjult hvis tomt)")

	return cmd
}
