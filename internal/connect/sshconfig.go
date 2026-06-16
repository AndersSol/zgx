package connect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostBlock bygger en ~/.ssh/config Host-blokk for en ZGX-enhet, omsluttet av
// zgx-marker-kommentarer slik at MergeHostConfig kan finne og erstatte den
// idempotent uten å røre brukerens øvrige config.
func HostBlock(alias, hostName, user string, port int, identityFile string) string {
	return fmt.Sprintf(`# >>> zgx managed: %s >>>
Host %s
    HostName %s
    User %s
    Port %d
    IdentityFile %s
    IdentitiesOnly yes
# <<< zgx managed: %s <<<
`, alias, alias, hostName, user, port, identityFile, alias)
}

// MergeHostConfig fletter en Host-blokk for `alias` inn i eksisterende
// config-innhold. IDEMPOTENT: finnes en zgx-managed blokk for samme alias
// allerede, ERSTATT den; ellers APPEND. Brukerens andre linjer bevares uendret.
func MergeHostConfig(existing, alias, block string) string {
	startMarker := fmt.Sprintf("# >>> zgx managed: %s >>>", alias)
	endMarker := fmt.Sprintf("# <<< zgx managed: %s <<<", alias)

	start := strings.Index(existing, startMarker)
	if start >= 0 {
		endRelative := strings.Index(existing[start:], endMarker)
		if endRelative >= 0 {
			end := start + endRelative + len(endMarker)
			if end < len(existing) && existing[end] == '\r' {
				end++
			}
			if end < len(existing) && existing[end] == '\n' {
				end++
			}
			return existing[:start] + normalizeBlock(block) + existing[end:]
		}
	}

	if existing == "" {
		return normalizeBlock(block)
	}

	separator := ""
	if strings.HasSuffix(existing, "\n\n") || strings.HasSuffix(existing, "\r\n\r\n") {
		separator = ""
	} else if strings.HasSuffix(existing, "\n") || strings.HasSuffix(existing, "\r\n") {
		separator = "\n"
	} else {
		separator = "\n\n"
	}
	return existing + separator + normalizeBlock(block)
}

// WriteHostConfig leser configPath (tom hvis fraværende), fletter inn blokken,
// og skriver tilbake med perms 0600. Oppretter mappa (0700) ved behov.
func WriteHostConfig(configPath, alias, hostName, user string, port int, identityFile string) error {
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "alias", value: alias},
		{field: "hostName", value: hostName},
		{field: "user", value: user},
		{field: "identityFile", value: identityFile},
	} {
		if err := validateConfigValue(item.field, item.value); err != nil {
			return err
		}
	}

	dir := filepath.Dir(configPath)
	if err := ensureSecureDir(dir); err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read ssh config %q: %w", configPath, err)
	}

	merged := MergeHostConfig(string(existingBytes), alias, HostBlock(alias, hostName, user, port, identityFile))
	if err := os.WriteFile(configPath, []byte(merged), 0o600); err != nil {
		return fmt.Errorf("write ssh config %q: %w", configPath, err)
	}
	if err := os.Chmod(configPath, 0o600); err != nil {
		return fmt.Errorf("chmod ssh config %q: %w", configPath, err)
	}
	return nil
}

func validateConfigValue(field, value string) error {
	if value == "" {
		return fmt.Errorf("ugyldig ssh config-verdi for %s: tom", field)
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("ugyldig ssh config-verdi for %s: innledende/avsluttende whitespace", field)
	}
	for _, r := range value {
		if r == '\n' || r == '\r' || (r < 0x20 && r != ' ') {
			return fmt.Errorf("ugyldig ssh config-verdi for %s: kontrolltegn", field)
		}
	}
	return nil
}

func normalizeBlock(block string) string {
	return strings.TrimRight(block, "\r\n") + "\n"
}
