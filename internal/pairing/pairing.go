// Package pairing oppdager og konfigurerer ConnectX-NIC-er over en testbar
// command runner.
package pairing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/install"
)

const (
	NetplanPath = "/etc/netplan/40-zgx-connectx.yaml"
	LshwCommand = "lshw -class network -json"
)

const (
	lshwTimeout  = 15 * time.Second
	ipTimeout    = 10 * time.Second
	sudoTimeout  = 30 * time.Second
	noRetry      = 0
	sudoRetry    = 0
	mellanoxTerm = "mellanox"
)

var linuxDeviceNamePattern = regexp.MustCompile(`^enp[a-zA-Z0-9_-]+$`)

// NIC beskriver et ConnectX nettverksinterface.
type NIC struct {
	LinuxDeviceName string
	IPv4Address     string
}

// ParseConnectXNICs filtrerer lshw-nettverksobjekter ned til Mellanox/ConnectX
// NIC-er med ett enkelt enp logicalname. IPv4Address fylles ikke her.
func ParseConnectXNICs(lshwJSON []byte) ([]NIC, error) {
	var entries []struct {
		Product     string          `json:"product"`
		Vendor      string          `json:"vendor"`
		LogicalName json.RawMessage `json:"logicalname"`
	}
	if err := json.Unmarshal(lshwJSON, &entries); err != nil {
		return nil, fmt.Errorf("parse lshw network JSON: %w", err)
	}

	nics := make([]NIC, 0, len(entries))
	for _, entry := range entries {
		if !containsMellanox(entry.Product) && !containsMellanox(entry.Vendor) {
			continue
		}

		var logicalName string
		if err := json.Unmarshal(entry.LogicalName, &logicalName); err != nil {
			continue
		}
		if !strings.HasPrefix(logicalName, "enp") {
			continue
		}

		nics = append(nics, NIC{LinuxDeviceName: logicalName})
	}

	return nics, nil
}

// IPCommand bygger kommandoen som henter første IPv4-adresse for et interface.
func IPCommand(deviceName string) string {
	return fmt.Sprintf("ip a l %s | awk '/inet / {print $2}'", deviceName)
}

// ParseIPv4 returnerer første IP-linje uten CIDR-suffiks.
func ParseIPv4(ipOutput string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(ipOutput), "\n")
	line = strings.TrimSpace(line)
	ip, _, _ := strings.Cut(line, "/")
	return ip
}

// BuildNetplan bygger netplan-YAML-en kilden skriver for ConnectX-interface.
func BuildNetplan(nics []NIC) (string, error) {
	lines := []string{
		"network:",
		"  version: 2",
		"  ethernets:",
	}
	for _, nic := range nics {
		if !linuxDeviceNamePattern.MatchString(nic.LinuxDeviceName) {
			return "", fmt.Errorf("ugyldig Linux device name %q", nic.LinuxDeviceName)
		}
		lines = append(lines,
			fmt.Sprintf("    %s:", nic.LinuxDeviceName),
			"      link-local: [ ipv4 ]",
		)
	}
	return strings.Join(lines, "\n"), nil
}

// WriteNetplanCommand bygger sudo-kommandoen som skriver og sikrer netplan-filen.
func WriteNetplanCommand(config string) string {
	inner := fmt.Sprintf("echo '%s' > %s && chmod 600 %s", singleQuoteEscape(config), NetplanPath, NetplanPath)
	return "sudo -S sh -c " + singleQuote(inner)
}

func ApplyNetplanCommand() string {
	return "sudo -S netplan apply"
}

func RemoveNetplanCommand() string {
	return "sudo -S rm -f " + NetplanPath
}

// PairDetails henter ConnectX-NIC-er og deres nåværende IPv4-adresser.
func PairDetails(ctx context.Context, runner install.Runner) ([]NIC, error) {
	if runner == nil {
		return nil, fmt.Errorf("pairing: Runner mangler")
	}

	result, err := runner.Run(ctx, LshwCommand, "", lshwTimeout, noRetry)
	if err != nil {
		return nil, fmt.Errorf("kjør lshw: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("lshw feilet med exit %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	nics, err := ParseConnectXNICs([]byte(result.Stdout))
	if err != nil {
		return nil, err
	}

	for i := range nics {
		if !linuxDeviceNamePattern.MatchString(nics[i].LinuxDeviceName) {
			return nil, fmt.Errorf("ugyldig Linux device name %q", nics[i].LinuxDeviceName)
		}
		result, err := runner.Run(ctx, IPCommand(nics[i].LinuxDeviceName), "", ipTimeout, noRetry)
		if err != nil {
			return nil, fmt.Errorf("hent IP for %s: %w", nics[i].LinuxDeviceName, err)
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("hent IP for %s feilet med exit %d: %s", nics[i].LinuxDeviceName, result.ExitCode, strings.TrimSpace(result.Stderr))
		}
		nics[i].IPv4Address = ParseIPv4(result.Stdout)
	}

	return nics, nil
}

// Pair oppdager ConnectX-NIC-er, skriver netplan og anvender konfigurasjonen.
func Pair(ctx context.Context, runner install.Runner, sudoPassword string) ([]NIC, error) {
	nics, err := PairDetails(ctx, runner)
	if err != nil {
		return nil, err
	}
	if len(nics) == 0 {
		return nil, fmt.Errorf("ingen ConnectX-NIC-er funnet")
	}

	config, err := BuildNetplan(nics)
	if err != nil {
		return nil, err
	}

	if err := runSudo(ctx, runner, WriteNetplanCommand(config), sudoPassword, "skriv netplan"); err != nil {
		return nil, err
	}
	if err := runSudo(ctx, runner, ApplyNetplanCommand(), sudoPassword, "anvend netplan"); err != nil {
		return nil, err
	}

	return nics, nil
}

// Unpair fjerner ConnectX-netplan-filen og anvender netplan.
func Unpair(ctx context.Context, runner install.Runner, sudoPassword string) error {
	if runner == nil {
		return fmt.Errorf("pairing: Runner mangler")
	}
	if err := runSudo(ctx, runner, RemoveNetplanCommand(), sudoPassword, "fjern netplan"); err != nil {
		return err
	}
	return runSudo(ctx, runner, ApplyNetplanCommand(), sudoPassword, "anvend netplan")
}

func runSudo(ctx context.Context, runner install.Runner, command, sudoPassword, label string) error {
	result, err := runner.Run(ctx, command, sudoPassword, sudoTimeout, sudoRetry)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s feilet med exit %d: %s", label, result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func containsMellanox(value string) bool {
	return strings.Contains(strings.ToLower(value), mellanoxTerm)
}

func singleQuoteEscape(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

func singleQuote(value string) string {
	return "'" + singleQuoteEscape(value) + "'"
}
