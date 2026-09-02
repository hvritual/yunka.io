package devruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/assemblyplan"
)

const DefaultRuntimeAssemblyPlan = "contracts/generated/assembly-plan.json"

// deriveAssemblyOwnership makes the generated AssemblyPlan authoritative for
// process -> Application ownership whenever the topology is unambiguous.
//
// A single process owns the root target. In a multi-process manifest, a process
// owns the AssemblyPlan target whose name equals the process name. Legacy
// graphNode/graphNodes remain accepted as a compatibility assertion; when a
// generated target exists, disagreement is rejected instead of silently
// creating a second source of truth.
func deriveAssemblyOwnership(root string, processes []Process) ([]Process, error) {
	planPath := filepath.Join(root, filepath.FromSlash(DefaultRuntimeAssemblyPlan))
	plan, err := assemblyplan.Load(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return processes, nil
		}
		return nil, fmt.Errorf("devruntime: generated assembly ownership: %w", err)
	}

	targets := make(map[string][]string, len(plan.Targets))
	for _, target := range plan.Targets {
		name := strings.TrimSpace(target.Name)
		if name == "" {
			continue
		}
		targets[name] = applicationNodeIDs(target.Applications)
	}

	result := append([]Process(nil), processes...)
	for index := range result {
		process := &result[index]
		var derived []string
		if len(result) == 1 {
			derived = targets[assemblyplan.RootTarget]
			if len(derived) == 0 {
				derived = targets[process.Name]
			}
		} else {
			derived = targets[process.Name]
		}
		if len(derived) == 0 {
			continue
		}

		legacy := processOwnedGraphNodes(*process)
		if len(legacy) != 0 {
			legacy = stableStrings(legacy)
			if !equalOwnedGraphNodes(legacy, derived) {
				return nil, fmt.Errorf("devruntime: process %q legacy graph ownership disagrees with generated assembly plan: legacy=%v generated=%v; remove graphNode/graphNodes", process.Name, legacy, derived)
			}
		}
		process.GraphNode = ""
		process.GraphNodes = append([]string(nil), derived...)
	}
	return result, nil
}

func applicationNodeIDs(applications []string) []string {
	seen := make(map[string]struct{}, len(applications))
	result := make([]string, 0, len(applications))
	for _, application := range applications {
		application = strings.TrimSpace(application)
		if application == "" {
			continue
		}
		id := applicationgraph.ID(applicationgraph.NodeApplication, application)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
