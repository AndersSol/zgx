package cmd

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var pipeToShellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\|\s*(sudo\s+)?(ba)?sh\b`),
	regexp.MustCompile(`(?i)\b(curl|wget)\b.*(&&|;)\s*(sudo\s+)?(ba)?sh\b`),
}

// PlanItem beskriver én rå kommando som skal vises før kjøring.
type PlanItem struct {
	ID, Command string
}

// PipesToShell rapporterer om en kommando rører pipe-to-shell (curl|sh-mønster)
// - verdt å fremheve for brukeren (laster ned og kjører ekstern kode).
func PipesToShell(command string) bool {
	for _, pattern := range pipeToShellPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

// RenderPlan bygger en lesbar plan over hva som vil kjøre: per app id + den
// rå kommandoen, med en markør på pipe-to-shell-kommandoer.
func RenderPlan(title string, items []PlanItem) string {
	var b strings.Builder
	fmt.Fprintln(&b, title)
	for _, item := range items {
		marker := " "
		if PipesToShell(item.Command) {
			marker = "⚠"
		}
		fmt.Fprintf(&b, "%s %s\n  %s\n", marker, item.ID, item.Command)
	}
	return b.String()
}

// Confirm leser en ja/nei-bekreftelse fra in og skriver prompt til out.
// Returnerer true kun ved eksakt "ja" (trimmet, case-insensitivt).
func Confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}

	return strings.EqualFold(strings.TrimSpace(answer), "ja"), nil
}
