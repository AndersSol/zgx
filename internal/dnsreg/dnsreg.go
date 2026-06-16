// Package dnsreg registrerer ZGX-enheter i Avahi for stabil mDNS-oppdagelse.
package dnsreg

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/install"
)

const (
	ServiceType     = "_hpzgx._tcp"
	ServicePort     = 22
	ServiceFilePath = "/etc/avahi/services/hpzgx.service"

	commandTimeout = 30 * time.Second
)

var deviceIdentifierPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// DeviceIdentifierCommand returnerer den faste remote-kommandoen som henter
// default-NIC-ens MAC og hasher den til en 8-tegns ID.
func DeviceIdentifierCommand() string {
	return "ip route show default | awk '/default/ { print $5 }' | head -1 | xargs -I {} cat /sys/class/net/{}/address | tr -d ':' | sha256sum | cut -c1-8"
}

// ServiceFileXML bygger avahi service-group-XML for identifier (XML-escaped).
func ServiceFileXML(identifier string) string {
	return fmt.Sprintf(`<service-group>
  <name>%s</name>
  <service>
    <type>%s</type>
    <port>%d</port>
  </service>
</service-group>`, escapeXML(identifier), ServiceType, ServicePort)
}

// CreateServiceFileCommand bygger sudo-kommandoen som skriver avahi service-fila.
func CreateServiceFileCommand(xml string) string {
	inner := "echo " + singleQuote(xml) + " | tee " + ServiceFilePath + " > /dev/null"
	return "sudo -S bash -c " + singleQuote(inner)
}

// RestartAvahiCommand returnerer den faste restart-kommandoen.
func RestartAvahiCommand() string {
	return "sudo -S systemctl restart avahi-daemon"
}

type Result struct {
	Identifier         string
	ServiceFileWritten bool
	AvahiRestarted     bool
	Note               string
}

// Register kjører hele dns-register-flyten over runner.
func Register(ctx context.Context, runner install.Runner, sudoPassword string) (Result, error) {
	if runner == nil {
		return Result{}, fmt.Errorf("dns-register: Runner mangler")
	}

	idResult, err := runner.Run(ctx, DeviceIdentifierCommand(), "", commandTimeout, 0)
	if err != nil {
		return Result{}, fmt.Errorf("dns-register: hent device-id: %w", err)
	}
	if idResult.ExitCode != 0 {
		return Result{}, fmt.Errorf("dns-register: hent device-id feilet med exit %d: %s", idResult.ExitCode, strings.TrimSpace(idResult.Stderr))
	}
	identifier := strings.TrimSpace(idResult.Stdout)
	if identifier == "" {
		return Result{}, fmt.Errorf("dns-register: tom device-id fra kommando %q", DeviceIdentifierCommand())
	}
	if !deviceIdentifierPattern.MatchString(identifier) {
		return Result{}, fmt.Errorf("dns-register: uventet device-id-format: %q", identifier)
	}

	result := Result{Identifier: identifier}
	xml := ServiceFileXML(identifier)
	createResult, err := runner.Run(ctx, CreateServiceFileCommand(xml), sudoPassword, commandTimeout, 0)
	if err != nil {
		return Result{}, fmt.Errorf("dns-register: skriv service-fil: %w", err)
	}
	if createResult.ExitCode != 0 {
		return Result{}, fmt.Errorf("dns-register: skriv service-fil feilet med exit %d: %s", createResult.ExitCode, strings.TrimSpace(createResult.Stderr))
	}
	result.ServiceFileWritten = true

	restartResult, err := runner.Run(ctx, RestartAvahiCommand(), sudoPassword, commandTimeout, 0)
	if err != nil || restartResult.ExitCode != 0 {
		result.AvahiRestarted = false
		result.Note = "Avahi kunne ikke restartes; service-fila er skrevet og aktiveres ved neste omstart."
		return result, nil
	}

	result.AvahiRestarted = true
	return result, nil
}

func escapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
