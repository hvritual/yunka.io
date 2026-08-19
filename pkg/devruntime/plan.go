package devruntime

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

type Plan struct {
	Processes []Process `json:"processes"`
}

func BuildPlan(manifest DevManifest, root string, targets []string, graph applicationgraph.Graph) (Plan, error) {
	if err := manifest.Validate(root, graph); err != nil {
		return Plan{}, err
	}
	byName := make(map[string]Process, len(manifest.Processes))
	for _, process := range manifest.Processes {
		process = normalizeProcess(process)
		byName[process.Name] = process
	}
	targets = stableStrings(targets)
	if len(targets) == 0 {
		for name := range byName {
			targets = append(targets, name)
		}
		sort.Strings(targets)
	}
	for _, target := range targets {
		if _, ok := byName[target]; !ok {
			return Plan{}, fmt.Errorf("devruntime: target %q not found", target)
		}
	}

	state := make(map[string]uint8)
	ordered := make([]Process, 0)
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("devruntime: dependency cycle at %q", name)
		case 2:
			return nil
		}
		state[name] = 1
		current := byName[name]
		deps := append([]string(nil), current.DependsOn...)
		sort.Strings(deps)
		for _, dependency := range deps {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, current)
		return nil
	}
	for _, target := range targets {
		if err := visit(target); err != nil {
			return Plan{}, err
		}
	}
	if len(ordered) == 0 {
		return Plan{}, errors.New("devruntime: empty plan")
	}
	return Plan{Processes: ordered}, nil
}

func (plan Plan) Names() []string {
	result := make([]string, 0, len(plan.Processes))
	for _, process := range plan.Processes {
		result = append(result, strings.TrimSpace(process.Name))
	}
	return result
}
