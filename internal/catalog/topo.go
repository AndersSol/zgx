package catalog

import (
	"fmt"
	"strings"
)

type visitState uint8

const (
	unvisited visitState = iota
	visiting
	visited
)

// InstallOrder returnerer apps i installerings-rekkefølge: hver avhengighet før
// appen som krever den. Port av appInstallationService.sortAppsByDependencies
// DFS post-order over PRESIST det innsendte settet -- deps utenfor settet
// løses ikke (kalleren bestemmer subsettet). Tillegg utover kilden: en
// avhengighetssykel gir LOUD feil, ikke en stille vilkårlig rekkefølge.
func InstallOrder(apps []App) ([]App, error) {
	byID := make(map[string]App, len(apps))
	for _, app := range apps {
		if _, ok := byID[app.ID]; !ok {
			byID[app.ID] = app
		}
	}

	sorted := make([]App, 0, len(apps))
	states := make(map[string]visitState, len(apps))
	stack := make([]string, 0, len(apps))
	stackPos := make(map[string]int, len(apps))

	var visit func(App) error
	visit = func(app App) error {
		switch states[app.ID] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("topo: avhengighetssykel oppdaget: %s", cyclePath(stack, stackPos, app.ID))
		}

		states[app.ID] = visiting
		stackPos[app.ID] = len(stack)
		stack = append(stack, app.ID)

		for _, depID := range app.Dependencies {
			depApp, ok := byID[depID]
			if !ok {
				continue
			}
			if err := visit(depApp); err != nil {
				return err
			}
		}

		stack = stack[:len(stack)-1]
		delete(stackPos, app.ID)
		states[app.ID] = visited
		sorted = append(sorted, app)

		return nil
	}

	for _, app := range apps {
		if err := visit(app); err != nil {
			return nil, err
		}
	}

	return sorted, nil
}

func cyclePath(stack []string, stackPos map[string]int, id string) string {
	pos, ok := stackPos[id]
	if !ok {
		return id
	}

	cycle := append([]string{}, stack[pos:]...)
	cycle = append(cycle, id)

	return strings.Join(cycle, " -> ")
}
