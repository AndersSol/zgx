package cmd

import (
	"fmt"

	configpkg "github.com/AndersSol/zgx/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd())
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Administrer lagrede enheter",
	}
	cmd.AddCommand(
		configAddCmd(),
		configListCmd(),
		configRemoveCmd(),
	)
	return cmd
}

func configAddCmd() *cobra.Command {
	var device configpkg.Device
	cmd := &cobra.Command{
		Use:   "add <alias> <host>",
		Short: "Lagre eller oppdater en enhet",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			device.Alias = args[0]
			device.Host = args[1]

			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			cfg.Upsert(device)
			if err := configpkg.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Lagret %s.\n", device.Alias)
			return nil
		},
	}
	cmd.Flags().StringVar(&device.User, "user", "hp", "SSH-bruker på enheten")
	cmd.Flags().IntVar(&device.Port, "port", 22, "SSH-port på enheten")
	cmd.Flags().StringVar(&device.Identity, "identity", "", "privat SSH-nøkkel")
	return cmd
}

func configListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Vis lagrede enheter",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(cfg.Devices) == 0 {
				fmt.Fprintln(out, "Ingen enheter lagret.")
				return nil
			}
			for _, device := range cfg.Devices {
				fmt.Fprintf(out, "%s  %s@%s:%d\n", device.Alias, device.User, device.Host, device.Port)
			}
			return nil
		},
	}
}

func configRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <alias>",
		Short: "Fjern en lagret enhet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]
			cfg, path, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			if !cfg.Remove(alias) {
				fmt.Fprintf(cmd.OutOrStdout(), "Fant ikke %s.\n", alias)
				return fmt.Errorf("fant ikke %s", alias)
			}
			if err := configpkg.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fjernet %s.\n", alias)
			return nil
		},
	}
}

func loadDefaultConfig() (*configpkg.Config, string, error) {
	path, err := configpkg.DefaultPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := configpkg.Load(path)
	if err != nil {
		return nil, "", err
	}
	return cfg, path, nil
}
