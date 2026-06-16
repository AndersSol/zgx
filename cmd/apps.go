package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AndersSol/zgx/internal/catalog"
	"github.com/AndersSol/zgx/internal/connect"
	"github.com/AndersSol/zgx/internal/install"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(
		listAppsCmd(),
		installAppsCmd(),
		verifyAppsCmd(),
		uninstallAppsCmd(),
	)
}

func listAppsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List apper i den kuraterte AI-stacken",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cats, err := catalog.Load()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			for i, cat := range cats {
				if i > 0 {
					fmt.Fprintln(out)
				}
				fmt.Fprintln(out, cat.Name)
				for _, app := range cat.Apps {
					fmt.Fprintf(out, "  %s %s  — %s\n", app.Icon, app.ID, app.Description)
				}
			}

			return nil
		},
	}
}

type appCommandOptions struct {
	host       string
	user       string
	port       int
	identity   string
	knownHosts string
	all        bool
	yes        bool
}

func installAppsCmd() *cobra.Command {
	opts := defaultAppCommandOptions()

	cmd := &cobra.Command{
		Use:   "install [app...]",
		Short: "Installer apper på enheten over SSH",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cats, err := catalog.Load()
			if err != nil {
				return err
			}

			ids, _, err := resolveAppArgs(cats, args, opts.all)
			if err != nil {
				return err
			}

			items, err := installPlanItems(cats, ids)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprint(out, RenderPlan("Installerer:", items))

			hasPipeToShell := planHasPipeToShell(items)
			if (opts.all || hasPipeToShell) && !opts.yes {
				prompt := fmt.Sprintf("Installer alle %d apper? Skriv ja: ", len(ids))
				if hasPipeToShell {
					prompt = "Planen kjører kommandoer som laster ned og kjører ekstern kode. Fortsett? Skriv ja: "
				}
				ok, err := Confirm(os.Stdin, out, prompt)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "\nAvbrutt.")
					return nil
				}
				fmt.Fprintln(out)
			}

			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			engine, _, err := buildEngine(opts.host, opts.user, opts.port, opts.identity, opts.knownHosts)
			if err != nil {
				return err
			}

			report, err := engine.Install(context.Background(), cats, ids, password)
			if err != nil {
				return err
			}
			writeInstallReport(out, report, "Installert")
			return failOnReport(report)
		},
	}

	addCommonAppFlags(cmd, &opts, true)
	return cmd
}

func verifyAppsCmd() *cobra.Command {
	opts := defaultAppCommandOptions()

	cmd := &cobra.Command{
		Use:   "verify [app...]",
		Short: "Verifiser at apper er installert",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cats, err := catalog.Load()
			if err != nil {
				return err
			}

			ids, _, err := resolveAppArgs(cats, args, opts.all)
			if err != nil {
				return err
			}

			engine, _, err := buildEngine(opts.host, opts.user, opts.port, opts.identity, opts.knownHosts)
			if err != nil {
				return err
			}

			result, err := engine.Verify(context.Background(), cats, ids)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			for _, id := range ids {
				if result[id] {
					fmt.Fprintf(out, "✓ %s installert\n", id)
				} else {
					fmt.Fprintf(out, "✗ %s mangler\n", id)
				}
			}
			return nil
		},
	}

	addCommonAppFlags(cmd, &opts, false)
	return cmd
}

func uninstallAppsCmd() *cobra.Command {
	opts := defaultAppCommandOptions()

	cmd := &cobra.Command{
		Use:   "uninstall [app...]",
		Short: "Avinstaller apper fra enheten",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cats, err := catalog.Load()
			if err != nil {
				return err
			}

			ids, apps, err := resolveAppArgs(cats, args, opts.all)
			if err != nil {
				return err
			}

			items, err := uninstallPlanItems(apps)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprint(out, RenderPlan("Avinstallerer:", items))

			if !opts.yes {
				prompt := fmt.Sprintf("Avinstaller %d apper? Skriv ja: ", len(ids))
				if opts.all {
					prompt = fmt.Sprintf("Avinstaller alle %d apper? Skriv ja: ", len(ids))
				}
				ok, err := Confirm(os.Stdin, out, prompt)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "\nAvbrutt.")
					return nil
				}
				fmt.Fprintln(out)
			}

			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			engine, _, err := buildEngine(opts.host, opts.user, opts.port, opts.identity, opts.knownHosts)
			if err != nil {
				return err
			}

			report, err := engine.Uninstall(context.Background(), cats, ids, password)
			if err != nil {
				return err
			}
			writeInstallReport(out, report, "Avinstallert")
			return failOnReport(report)
		},
	}

	addCommonAppFlags(cmd, &opts, true)
	return cmd
}

func defaultAppCommandOptions() appCommandOptions {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return appCommandOptions{user: "hp", port: 22}
	}
	return appCommandOptions{
		user:       "hp",
		port:       22,
		identity:   filepath.Join(homeDir, ".ssh", "id_ed25519"),
		knownHosts: filepath.Join(homeDir, ".ssh", "known_hosts"),
	}
}

func addCommonAppFlags(cmd *cobra.Command, opts *appCommandOptions, withYes bool) {
	cmd.Flags().StringVar(&opts.host, "host", "", "SSH-host for enheten")
	cmd.Flags().StringVar(&opts.user, "user", opts.user, "SSH-bruker på enheten")
	cmd.Flags().IntVar(&opts.port, "port", opts.port, "SSH-port på enheten")
	cmd.Flags().StringVar(&opts.identity, "identity", opts.identity, "privat SSH-nøkkel")
	cmd.Flags().StringVar(&opts.knownHosts, "known-hosts", opts.knownHosts, "known_hosts-fil")
	cmd.Flags().BoolVar(&opts.all, "all", false, "velg alle apper i katalogen")
	if withYes {
		cmd.Flags().BoolVar(&opts.yes, "yes", false, "hopp over bekreftelse")
	}
	_ = cmd.MarkFlagRequired("host")
}

func buildEngine(host, user string, port int, identity, knownHosts string) (*install.Engine, []catalog.Category, error) {
	hostKey, err := connect.KnownHostsCallback(expandHome(knownHosts))
	if err != nil {
		return nil, nil, err
	}

	cats, err := catalog.Load()
	if err != nil {
		return nil, nil, err
	}

	runner := install.SSHRunner{
		Target:         connect.Target{Host: host, User: user, Port: port},
		HostKey:        hostKey,
		PrivateKeyPath: expandHome(identity),
	}
	return &install.Engine{Runner: runner}, cats, nil
}

func resolveAppArgs(cats []catalog.Category, args []string, all bool) ([]string, []catalog.App, error) {
	if all && len(args) > 0 {
		return nil, nil, errors.New("bruk enten app-id-er eller --all, ikke begge")
	}
	if !all && len(args) == 0 {
		return nil, nil, errors.New("oppgi minst én app eller --all")
	}

	if all {
		apps := catalog.AllApps(cats)
		ids := make([]string, 0, len(apps))
		for _, app := range apps {
			ids = append(ids, app.ID)
		}
		return ids, apps, nil
	}

	ids := make([]string, 0, len(args))
	apps := make([]catalog.App, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, id := range args {
		if _, ok := seen[id]; ok {
			continue
		}
		app, ok := catalog.ByID(cats, id)
		if !ok {
			return nil, nil, fmt.Errorf("ukjent app-id %q", id)
		}
		ids = append(ids, id)
		apps = append(apps, app)
		seen[id] = struct{}{}
	}
	return ids, apps, nil
}

func installPlanItems(cats []catalog.Category, ids []string) ([]PlanItem, error) {
	apps, err := install.InstallPlan(cats, ids)
	if err != nil {
		return nil, err
	}

	items := make([]PlanItem, 0, len(apps))
	for _, app := range apps {
		items = append(items, PlanItem{ID: app.ID, Command: app.InstallCommand})
	}
	return items, nil
}

func planHasPipeToShell(items []PlanItem) bool {
	for _, item := range items {
		if PipesToShell(item.Command) {
			return true
		}
	}
	return false
}

func uninstallPlanItems(apps []catalog.App) ([]PlanItem, error) {
	ordered, err := catalog.InstallOrder(apps)
	if err != nil {
		return nil, err
	}
	slices.Reverse(ordered)

	items := make([]PlanItem, 0, len(ordered))
	for _, app := range ordered {
		command := "(kan ikke avinstalleres)"
		if app.UninstallCommand != nil {
			command = *app.UninstallCommand
		}
		items = append(items, PlanItem{ID: app.ID, Command: command})
	}
	return items, nil
}

func readSudoPassword(out io.Writer) (string, error) {
	fmt.Fprint(out, "Sudo-passord: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return "", fmt.Errorf("les sudo-passord: %w", err)
	}
	return string(passwordBytes), nil
}

func writeInstallReport(out io.Writer, report install.Report, installedLabel string) {
	fmt.Fprintf(out, "%s: %s\n", installedLabel, listOrDash(report.Installed))
	fmt.Fprintf(out, "Allerede installert: %s\n", listOrDash(report.AlreadyInstalled))
	if len(report.Skipped) > 0 {
		fmt.Fprintf(out, "Hoppet over: %s\n", listOrDash(report.Skipped))
	}
	fmt.Fprintf(out, "FEILET: %s\n", listOrDash(report.Failed))
}

func failOnReport(report install.Report) error {
	if len(report.Failed) == 0 {
		return nil
	}
	return fmt.Errorf("feilet for: %s", strings.Join(report.Failed, ", "))
}

func listOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func expandHome(path string) string {
	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return homeDir
		}
	}
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
