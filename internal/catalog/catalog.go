// Package catalog eier den kuraterte app-katalogen uten CLI- eller UI-kobling.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
)

// App beskriver én installerbar app i katalogen.
type App struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Icon               string   `json:"icon"`
	Description        string   `json:"description"`
	Features           []string `json:"features"`
	Category           string   `json:"category"`
	InstallCommand     string   `json:"installCommand"`
	VerifyCommand      string   `json:"verifyCommand"`
	UninstallCommand   *string  `json:"uninstallCommand,omitempty"`
	RequiresVirtualEnv bool     `json:"requiresVirtualEnv,omitempty"`
	Dependencies       []string `json:"dependencies,omitempty"`
}

// Category grupperer apper slik kilden gjør.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Apps        []App  `json:"apps"`
}

var _ embed.FS

//go:embed catalog.json
var catalogJSON []byte

// Load leser embedded katalog og feiler høyt ved ugyldig data.
func Load() ([]Category, error) {
	var cats []Category
	if err := json.Unmarshal(catalogJSON, &cats); err != nil {
		return nil, fmt.Errorf("katalog: parse av embedded catalog.json feilet: %w", err)
	}

	return cats, nil
}

// AllApps flater ut katalogen for oppslag og validering.
func AllApps(cats []Category) []App {
	apps := make([]App, 0)
	for _, cat := range cats {
		apps = append(apps, cat.Apps...)
	}

	return apps
}

// ByID finner en app uten å anta sortering i katalogen.
func ByID(cats []Category, id string) (App, bool) {
	for _, app := range AllApps(cats) {
		if app.ID == id {
			return app, true
		}
	}

	return App{}, false
}

// CategoryByID finner en kategori uten å eksponere intern lagring.
func CategoryByID(cats []Category, id string) (Category, bool) {
	for _, cat := range cats {
		if cat.ID == id {
			return cat, true
		}
	}

	return Category{}, false
}
