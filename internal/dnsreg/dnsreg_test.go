package dnsreg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AndersSol/zgx/internal/install"
)

func TestServiceFileXML(t *testing.T) {
	got := ServiceFileXML("abcd1234")
	want := `<service-group>
  <name>abcd1234</name>
  <service>
    <type>_hpzgx._tcp</type>
    <port>22</port>
  </service>
</service-group>`
	if got != want {
		t.Fatalf("ServiceFileXML() = %q, vil ha %q", got, want)
	}

	got = ServiceFileXML(`a<b&c>"'`)
	if !strings.Contains(got, "<name>a&lt;b&amp;c&gt;&quot;&apos;</name>") {
		t.Fatalf("ServiceFileXML() escaper ikke XML-tegn: %q", got)
	}
}

func TestCreateServiceFileCommand(t *testing.T) {
	xml := "<service-group><name>it&apos;s</name></service-group>"
	got := CreateServiceFileCommand(xml)

	if !strings.HasPrefix(got, "sudo -S bash -c ") {
		t.Fatalf("CreateServiceFileCommand() = %q, vil ha sudo-prefix", got)
	}
	if !strings.Contains(got, "tee /etc/avahi/services/hpzgx.service") {
		t.Fatalf("CreateServiceFileCommand() mangler service-fil-sti: %q", got)
	}
	if !strings.Contains(got, "'\\''") {
		t.Fatalf("CreateServiceFileCommand() single-quote-escaper ikke apostrof: %q", got)
	}
}

func TestDeviceIdentifierCommand(t *testing.T) {
	want := "ip route show default | awk '/default/ { print $5 }' | head -1 | xargs -I {} cat /sys/class/net/{}/address | tr -d ':' | sha256sum | cut -c1-8"
	if got := DeviceIdentifierCommand(); got != want {
		t.Fatalf("DeviceIdentifierCommand() = %q, vil ha %q", got, want)
	}
}

func TestRegisterFlow(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: "abcd1234\n"},
			RestartAvahiCommand():     {ExitCode: 0},
		},
	}

	result, err := Register(context.Background(), runner, "pw")
	if err != nil {
		t.Fatalf("Register() returnerte feil: %v", err)
	}
	if result.Identifier != "abcd1234" || !result.ServiceFileWritten || !result.AvahiRestarted {
		t.Fatalf("Register() result = %#v", result)
	}

	createCommand := runner.commandContaining("tee /etc/avahi/services/hpzgx.service")
	if createCommand == "" {
		t.Fatalf("Register() kjørte ikke create-kommando; commands=%v", runner.commands)
	}
	if !strings.Contains(createCommand, "<name>abcd1234</name>") {
		t.Fatalf("create-kommando mangler XML med identifier: %q", createCommand)
	}
}

func TestRegisterRestartFailureNonFatal(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: "abcd1234\n"},
			RestartAvahiCommand():     {ExitCode: 1, Stderr: "nope"},
		},
	}

	result, err := Register(context.Background(), runner, "pw")
	if err != nil {
		t.Fatalf("Register() returnerte feil ved restart-feil, vil ha non-fatal: %v", err)
	}
	if result.AvahiRestarted {
		t.Fatalf("Register() AvahiRestarted = true ved restart-feil")
	}
	if result.Note == "" {
		t.Fatalf("Register() Note er tom ved restart-feil: %#v", result)
	}
}

func TestRegisterEmptyIdIsLoudError(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: " \n"},
		},
	}

	_, err := Register(context.Background(), runner, "pw")
	if err == nil {
		t.Fatal("Register() returnerte nil ved tom device-id")
	}
	if len(runner.commands) != 1 {
		t.Fatalf("Register() kjørte create/restart etter tom id: %v", runner.commands)
	}
}

func TestRegisterRejectsMalformedIdentifier(t *testing.T) {
	for _, stdout := range []string{"evil\n<inject>", "GGGG"} {
		t.Run(stdout, func(t *testing.T) {
			runner := &fakeRunner{
				results: map[string]install.CommandResult{
					DeviceIdentifierCommand(): {ExitCode: 0, Stdout: stdout},
				},
			}

			_, err := Register(context.Background(), runner, "pw")
			if err == nil {
				t.Fatal("Register() returnerte nil ved malformed device-id")
			}
			if runner.commandContaining("tee /etc/avahi/services/hpzgx.service") != "" {
				t.Fatalf("Register() kjørte service-filkommando etter malformed id: %v", runner.commands)
			}
			if runner.commandContaining("systemctl restart avahi-daemon") != "" {
				t.Fatalf("Register() kjørte restart etter malformed id: %v", runner.commands)
			}
		})
	}
}

type fakeRunner struct {
	results  map[string]install.CommandResult
	errors   map[string]error
	commands []string
}

func (r *fakeRunner) Run(_ context.Context, command, _ string, _ time.Duration, _ int) (install.CommandResult, error) {
	r.commands = append(r.commands, command)
	if err := r.errors[command]; err != nil {
		return install.CommandResult{}, err
	}
	if result, ok := r.results[command]; ok {
		return result, nil
	}
	if strings.Contains(command, "tee /etc/avahi/services/hpzgx.service") {
		return install.CommandResult{ExitCode: 0}, nil
	}
	return install.CommandResult{}, errors.New("uventet kommando: " + command)
}

func (r *fakeRunner) commandContaining(needle string) string {
	for _, command := range r.commands {
		if strings.Contains(command, needle) {
			return command
		}
	}
	return ""
}
