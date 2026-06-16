package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Device struct {
	Alias    string `json:"alias"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Port     int    `json:"port"`
	Identity string `json:"identity,omitempty"`
}

type Config struct {
	Devices []Device `json:"devices"`
}

// DefaultPath returnerer config-stien: $XDG_CONFIG_HOME/zgx/config.json,
// ellers ~/.config/zgx/config.json.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finn hjemmemappe: %w", err)
		}
		base = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(base, "zgx", "config.json"), nil
}

// Load leser config fra path. Fraværende fil returnerer tom Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("les config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("les config-json: %w", err)
	}
	cfg.sortDevices()
	return &cfg, nil
}

// Save skriver config til path med privat mappe og fil.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.sortDevices()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("opprett config-mappe: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("sett config-mappe-permissions: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serialiser config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("skriv config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("sett config-fil-permissions: %w", err)
	}
	return nil
}

// Get finner en enhet på alias.
func (c *Config) Get(alias string) (Device, bool) {
	for _, device := range c.Devices {
		if device.Alias == alias {
			return device, true
		}
	}
	return Device{}, false
}

// Upsert legger til eller oppdaterer en enhet på alias.
func (c *Config) Upsert(d Device) {
	for i := range c.Devices {
		if c.Devices[i].Alias == d.Alias {
			c.Devices[i] = d
			c.sortDevices()
			return
		}
	}
	c.Devices = append(c.Devices, d)
	c.sortDevices()
}

// Remove fjerner en enhet på alias; returnerer om noe ble fjernet.
func (c *Config) Remove(alias string) bool {
	for i := range c.Devices {
		if c.Devices[i].Alias == alias {
			c.Devices = append(c.Devices[:i], c.Devices[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) sortDevices() {
	sort.Slice(c.Devices, func(i, j int) bool {
		return c.Devices[i].Alias < c.Devices[j].Alias
	})
}
