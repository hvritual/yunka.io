package devruntime

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

type PlanOptions struct {
	Closure bool
}

type Plan struct {
	Processes []Process              `json:"processes"`
	Runtime   *RuntimeConfig         `json:"runtime,omitempty"`
	BaseGraph applicationgraph.Graph `json:"-"`
	Closure   bool                   `json:"-"`
}

func BuildPlan(manifest DevManifest, root string, targets []string, graph applicationgraph.Graph) (Plan, error) {
	return BuildPlanWithOptions(manifest, root, targets, graph, PlanOptions{})
}

func BuildPlanWithOptions(manifest DevManifest, root string, targets []string, graph applicationgraph.Graph, options PlanOptions) (Plan, error) {
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

	runtimeEnabled := manifest.SchemaVersion >= RuntimeClosureSchemaVersion || options.Closure
	closure := options.Closure
	var runtime *RuntimeConfig
	if runtimeEnabled {
		value := normalizeRuntimeConfig(manifest.Runtime)
		closure = closure || value.Closure
		runtime = &value
		var err error
		ordered, err = deriveAssemblyOwnership(root, ordered)
		if err != nil {
			return Plan{}, err
		}
	}
	if closure {
		if len(graph.Nodes) == 0 {
			return Plan{}, errors.New("devruntime: closure mode requires an application graph")
		}
		graphNodes := make(map[string]applicationgraph.Node, len(graph.Nodes))
		for _, node := range graph.Nodes {
			graphNodes[node.ID] = node
		}
		owners := make(map[string]string, len(ordered))
		for _, process := range ordered {
			owned := processOwnedGraphNodes(process)
			if len(owned) == 0 {
				return Plan{}, fmt.Errorf("devruntime: closure mode requires process %q generated assembly ownership or legacy graphNode/graphNodes", process.Name)
			}
			requiresDiagnostics := false
			for _, graphNode := range owned {
				node, ok := graphNodes[graphNode]
				if !ok {
					return Plan{}, fmt.Errorf("devruntime: process %q graph node %q not found", process.Name, graphNode)
				}
				if owner, exists := owners[graphNode]; exists {
					return Plan{}, fmt.Errorf("devruntime: graph node %q is owned by both %q and %q", graphNode, owner, process.Name)
				}
				owners[graphNode] = process.Name
				if node.Kind == applicationgraph.NodeApplication {
					requiresDiagnostics = true
				}
			}
			if requiresDiagnostics {
				if process.Readiness == nil {
					return Plan{}, fmt.Errorf("devruntime: closure mode requires application owner process %q readiness", process.Name)
				}
				if !process.Readiness.DiagnosticsReady {
					return Plan{}, fmt.Errorf("devruntime: closure mode requires application owner process %q diagnosticsReady=true", process.Name)
				}
				if !process.Readiness.CaptureDiagnostics {
					return Plan{}, fmt.Errorf("devruntime: closure mode requires application owner process %q captureDiagnostics=true", process.Name)
				}
			}
		}
	}

	return Plan{Processes: ordered, Runtime: runtime, BaseGraph: graph, Closure: closure}, nil
}

func (plan Plan) Names() []string {
	result := make([]string, 0, len(plan.Processes))
	for _, process := range plan.Processes {
		result = append(result, strings.TrimSpace(process.Name))
	}
	return result
}
