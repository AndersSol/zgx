package pairing

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AndersSol/zgx/internal/install"
)

const lshwFixture = `[
  {"product":"MT2892 Family [ConnectX-6 Dx]","vendor":"Mellanox Technologies","logicalname":"enp1s0f0"},
  {"product":"Ethernet Controller","vendor":"Intel Corporation","logicalname":"enp2s0"},
  {"product":"MT2894 Family [ConnectX-6 Lx]","vendor":"Mellanox Technologies","logicalname":"eth0"},
  {"product":"MT2894 Family [ConnectX-6 Lx]","vendor":"Mellanox Technologies"}
]`

func TestParseConnectXNICs(t *testing.T) {
	nics, err := ParseConnectXNICs([]byte(lshwFixture))
	if err != nil {
		t.Fatalf("ParseConnectXNICs returnerte feil: %v", err)
	}

	want := []NIC{{LinuxDeviceName: "enp1s0f0"}}
	if !slices.Equal(nics, want) {
		t.Fatalf("ParseConnectXNICs = %#v, vil ha %#v", nics, want)
	}
}

func TestParseConnectXNICsSkipsArrayLogicalName(t *testing.T) {
	fixture := `[{"product":"ConnectX","vendor":"Mellanox","logicalname":["enp1s0f0","enp1s0f1"]}]`
	nics, err := ParseConnectXNICs([]byte(fixture))
	if err != nil {
		t.Fatalf("ParseConnectXNICs returnerte feil: %v", err)
	}
	if len(nics) != 0 {
		t.Fatalf("ParseConnectXNICs med array logicalname = %#v, vil ha tom liste", nics)
	}
}

func TestParseIPv4(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "10.0.0.5/24\n", want: "10.0.0.5"},
		{input: "", want: ""},
		{input: "10.0.0.5/24\n192.168.1.10/24\n", want: "10.0.0.5"},
	}

	for _, tt := range tests {
		if got := ParseIPv4(tt.input); got != tt.want {
			t.Errorf("ParseIPv4(%q) = %q, vil ha %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildNetplan(t *testing.T) {
	config, err := BuildNetplan([]NIC{
		{LinuxDeviceName: "enp1s0f0"},
		{LinuxDeviceName: "enp2s0f1"},
	})
	if err != nil {
		t.Fatalf("BuildNetplan returnerte feil: %v", err)
	}

	want := strings.Join([]string{
		"network:",
		"  version: 2",
		"  ethernets:",
		"    enp1s0f0:",
		"      link-local: [ ipv4 ]",
		"    enp2s0f1:",
		"      link-local: [ ipv4 ]",
	}, "\n")
	if config != want {
		t.Fatalf("BuildNetplan = %q, vil ha %q", config, want)
	}
}

func TestBuildNetplanRejectsInjection(t *testing.T) {
	tests := []string{
		"enp0s1\n    evil: [x]",
		"enp0s1; rm -rf",
	}

	for _, name := range tests {
		if _, err := BuildNetplan([]NIC{{LinuxDeviceName: name}}); err == nil {
			t.Fatalf("BuildNetplan(%q) returnerte nil error, vil ha injection-guard-feil", name)
		}
	}
}

func TestWriteNetplanCommand(t *testing.T) {
	config := "network:\n  ethernets:\n    enp1's0:\n      link-local: [ ipv4 ]"
	command := WriteNetplanCommand(config)

	for _, want := range []string{"sudo -S sh -c", NetplanPath, "chmod 600 " + NetplanPath, "enp1", "s0"} {
		if !strings.Contains(command, want) {
			t.Fatalf("WriteNetplanCommand mangler %q i %q", want, command)
		}
	}
	if !strings.Contains(command, "'\\''") {
		t.Fatalf("WriteNetplanCommand single-quote-escaper ikke payload: %q", command)
	}
	if !strings.HasPrefix(command, "sudo -S sh -c") {
		t.Fatalf("WriteNetplanCommand starter ikke med sudo -S sh -c: %q", command)
	}
}

func TestWriteNetplanCommandSingleQuoteOuter(t *testing.T) {
	config := "network:\n  ethernets:\n    enp1s0:\n      match: \"$(whoami)`id`\""
	command := WriteNetplanCommand(config)

	if !strings.HasPrefix(command, "sudo -S sh -c '") {
		t.Fatalf("WriteNetplanCommand() = %q, vil ha single-quote etter sh -c", command)
	}
	if strings.HasPrefix(command, "sudo -S sh -c \"") {
		t.Fatalf("WriteNetplanCommand() bruker double-quote rundt sh -c: %q", command)
	}
	for _, want := range []string{NetplanPath, "chmod 600 " + NetplanPath} {
		if !strings.Contains(command, want) {
			t.Fatalf("WriteNetplanCommand mangler %q i %q", want, command)
		}
	}

	innerArg := strings.TrimPrefix(command, "sudo -S sh -c ")
	if !strings.HasPrefix(innerArg, "'") || !strings.HasSuffix(innerArg, "'") {
		t.Fatalf("inner command er ikke single-quote-omsluttet: %q", command)
	}
	if !strings.Contains(command, "$(whoami)") {
		t.Fatalf("testpayload mangler fra kommandoen: %q", command)
	}
}

func TestPairFlow(t *testing.T) {
	runner := &fakePairRunner{
		results: map[string]install.CommandResult{
			LshwCommand:                  {ExitCode: 0, Stdout: lshwFixture},
			IPCommand("enp1s0f0"):        {ExitCode: 0, Stdout: "10.0.0.5/24\n"},
			ApplyNetplanCommand():        {ExitCode: 0},
			RemoveNetplanCommand():       {ExitCode: 0},
			"unused command placeholder": {ExitCode: 0},
		},
	}

	nics, err := Pair(context.Background(), runner, "pw")
	if err != nil {
		t.Fatalf("Pair returnerte feil: %v", err)
	}
	want := []NIC{{LinuxDeviceName: "enp1s0f0", IPv4Address: "10.0.0.5"}}
	if !slices.Equal(nics, want) {
		t.Fatalf("Pair nics = %#v, vil ha %#v", nics, want)
	}

	writeCommand := runner.firstCommandContaining(NetplanPath)
	if writeCommand == "" {
		t.Fatalf("Pair kjørte ikke write-kommando; calls=%#v", runner.calls)
	}
	if !strings.Contains(writeCommand, "enp1s0f0") || !strings.Contains(writeCommand, "link-local: [ ipv4 ]") {
		t.Fatalf("write-kommando mangler netplan-YAML: %q", writeCommand)
	}
	if !runner.called(ApplyNetplanCommand()) {
		t.Fatalf("Pair kjørte ikke apply; calls=%#v", runner.calls)
	}
}

func TestPairNoNICsIsLoudError(t *testing.T) {
	runner := &fakePairRunner{
		results: map[string]install.CommandResult{
			LshwCommand: {ExitCode: 0, Stdout: `[{"product":"Ethernet Controller","vendor":"Intel","logicalname":"enp2s0"}]`},
		},
	}

	_, err := Pair(context.Background(), runner, "pw")
	if err == nil {
		t.Fatal("Pair uten ConnectX-NIC-er returnerte nil error")
	}
	if runner.firstCommandContaining(NetplanPath) != "" || runner.called(ApplyNetplanCommand()) {
		t.Fatalf("Pair kjørte write/apply ved tom NIC-liste; calls=%#v", runner.calls)
	}
}

type fakePairCall struct {
	command      string
	sudoPassword string
	timeout      time.Duration
	retries      int
}

type fakePairRunner struct {
	results map[string]install.CommandResult
	calls   []fakePairCall
}

func (r *fakePairRunner) Run(_ context.Context, command, sudoPassword string, timeout time.Duration, retries int) (install.CommandResult, error) {
	r.calls = append(r.calls, fakePairCall{
		command:      command,
		sudoPassword: sudoPassword,
		timeout:      timeout,
		retries:      retries,
	})
	if result, ok := r.results[command]; ok {
		return result, nil
	}
	if strings.HasPrefix(command, "sudo -S sh -c") {
		return install.CommandResult{ExitCode: 0}, nil
	}
	return install.CommandResult{ExitCode: 127, Stderr: "unexpected command"}, nil
}

func (r *fakePairRunner) firstCommandContaining(needle string) string {
	for _, call := range r.calls {
		if strings.Contains(call.command, needle) {
			return call.command
		}
	}
	return ""
}

func (r *fakePairRunner) called(command string) bool {
	for _, call := range r.calls {
		if call.command == command {
			return true
		}
	}
	return false
}
