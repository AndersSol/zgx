package discovery

import (
	"context"
	"errors"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brutella/dnssd"
)

const (
	sshService   = "_ssh._tcp"
	hpzgxService = "_hpzgx._tcp"
)

var zgxHostnamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^zgx-[A-Za-z0-9]{6}$`),
	regexp.MustCompile(`^zgx-[A-Za-z0-9]{4}$`),
	regexp.MustCompile(`^spark-[A-Za-z0-9]{4}$`),
}

// Device is a DNS-SD discovered ZGX device.
type Device struct {
	Name       string            `json:"name"`
	Hostname   string            `json:"hostname"`
	Addresses  []string          `json:"addresses"`
	Port       int               `json:"port"`
	Protocol   string            `json:"protocol"`
	TXTRecords map[string]string `json:"txtRecords,omitempty"`
}

// IsZGXHostname reports whether hostname matches a known ZGX factory/default pattern.
func IsZGXHostname(hostname string) bool {
	for _, pattern := range zgxHostnamePatterns {
		if pattern.MatchString(hostname) {
			return true
		}
	}
	return false
}

// HostnameFromHost extracts the bare hostname from a DNS-SD host value.
func HostnameFromHost(host string) string {
	trimmed := strings.TrimSuffix(host, ".")
	trimmed = strings.TrimSuffix(trimmed, ".local")
	if idx := strings.IndexByte(trimmed, '.'); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// MergeZGX merges SSH and HPZGX DNS-SD results using the source semantics from ZGX Toolkit.
func MergeZGX(ssh, hpzgx []Device) []Device {
	merged := make(map[string]Device)

	for _, device := range ssh {
		if IsZGXHostname(device.Hostname) {
			merged[device.Hostname] = device
		}
	}

	for _, device := range hpzgx {
		merged[device.Hostname] = device
	}

	hostnames := make([]string, 0, len(merged))
	for hostname := range merged {
		hostnames = append(hostnames, hostname)
	}
	sort.Strings(hostnames)

	devices := make([]Device, 0, len(hostnames))
	for _, hostname := range hostnames {
		devices = append(devices, merged[hostname])
	}
	return devices
}

// Discover browses for ZGX devices using mDNS/DNS-SD until ctx is done.
func Discover(ctx context.Context) ([]Device, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu    sync.Mutex
		ssh   []Device
		hpzgx []Device
	)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		devices, err := browse(ctx, sshService)
		mu.Lock()
		ssh = append(ssh, devices...)
		mu.Unlock()
		if isFatalDiscoveryError(err) {
			cancel()
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		devices, err := browse(ctx, hpzgxService)
		mu.Lock()
		hpzgx = append(hpzgx, devices...)
		mu.Unlock()
		if isFatalDiscoveryError(err) {
			cancel()
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return MergeZGX(ssh, hpzgx), nil
}

// DiscoverTimeout browses for ZGX devices for timeout and returns the devices found.
func DiscoverTimeout(timeout time.Duration) ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Discover(ctx)
}

func browse(ctx context.Context, service string) ([]Device, error) {
	var (
		mu      sync.Mutex
		devices []Device
	)

	add := func(entry dnssd.BrowseEntry) {
		mu.Lock()
		defer mu.Unlock()
		devices = append(devices, deviceFromBrowseEntry(entry))
	}

	err := dnssd.LookupType(ctx, service, add, func(dnssd.BrowseEntry) {})

	mu.Lock()
	defer mu.Unlock()
	return append([]Device(nil), devices...), err
}

func deviceFromBrowseEntry(entry dnssd.BrowseEntry) Device {
	return Device{
		Name:       entry.Name,
		Hostname:   HostnameFromHost(entry.Host),
		Addresses:  ipv4Strings(entry.IPs),
		Port:       entry.Port,
		Protocol:   "tcp",
		TXTRecords: copyTXTRecords(entry.Text),
	}
}

func ipv4Strings(ips []net.IP) []string {
	addresses := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip.To4() == nil {
			continue
		}
		addresses = append(addresses, ip.String())
	}
	return addresses
}

func copyTXTRecords(records map[string]string) map[string]string {
	if len(records) == 0 {
		return nil
	}

	copied := make(map[string]string, len(records))
	for key, value := range records {
		copied[key] = value
	}
	return copied
}

func isFatalDiscoveryError(err error) bool {
	return err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)
}
