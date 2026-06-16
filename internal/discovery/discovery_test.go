package discovery

import (
	"reflect"
	"testing"
)

func TestIsZGXHostname(t *testing.T) {
	positives := []string{
		"zgx-abc123",
		"zgx-ABCDEF",
		"zgx-ABCD",
		"spark-wxyz",
		"spark-1234",
	}
	for _, hostname := range positives {
		if !IsZGXHostname(hostname) {
			t.Errorf("IsZGXHostname(%q) = false, want true", hostname)
		}
	}

	negatives := []string{
		"zgx-abc",
		"zgx-abcde",
		"zgx-toolong7",
		"spark-toolong5",
		"spark-abc",
		"foobar",
		"",
		"zgx-",
		"ZGX-abcdef",
	}
	for _, hostname := range negatives {
		if IsZGXHostname(hostname) {
			t.Errorf("IsZGXHostname(%q) = true, want false", hostname)
		}
	}
}

func TestHostnameFromHost(t *testing.T) {
	tests := map[string]string{
		"zgx-abc123.local.": "zgx-abc123",
		"zgx-abc123.local":  "zgx-abc123",
		"foo":               "foo",
	}

	for input, want := range tests {
		if got := HostnameFromHost(input); got != want {
			t.Errorf("HostnameFromHost(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMergeZGX(t *testing.T) {
	ssh := []Device{
		{
			Name:      "ssh-zgx",
			Hostname:  "zgx-abc123",
			Addresses: []string{"10.0.0.10"},
			Port:      22,
			Protocol:  "tcp",
		},
		{
			Name:      "ssh-laptop",
			Hostname:  "laptop",
			Addresses: []string{"10.0.0.11"},
			Port:      22,
			Protocol:  "tcp",
		},
	}
	hpzgx := []Device{
		{
			Name:       "hpzgx-zgx",
			Hostname:   "zgx-abc123",
			Addresses:  []string{"1.2.3.4"},
			Port:       22,
			Protocol:   "tcp",
			TXTRecords: map[string]string{"source": "hpzgx"},
		},
		{
			Name:      "hpzgx-server",
			Hostname:  "server9",
			Addresses: []string{"10.0.0.12"},
			Port:      8022,
			Protocol:  "tcp",
		},
	}

	got := MergeZGX(ssh, hpzgx)
	want := []Device{
		{
			Name:      "hpzgx-server",
			Hostname:  "server9",
			Addresses: []string{"10.0.0.12"},
			Port:      8022,
			Protocol:  "tcp",
		},
		{
			Name:       "hpzgx-zgx",
			Hostname:   "zgx-abc123",
			Addresses:  []string{"1.2.3.4"},
			Port:       22,
			Protocol:   "tcp",
			TXTRecords: map[string]string{"source": "hpzgx"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeZGX() = %#v, want %#v", got, want)
	}
}
