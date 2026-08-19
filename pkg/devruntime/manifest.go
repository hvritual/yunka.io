package devruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

const DevSchemaVersion = 1

type DevManifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Processes     []Process `json:"processes"`
}

type Process struct {
	Name       string   `json:"name"`
	Command    []string `json:"command"`
	WorkingDir string   `json:"workingDir,omitempty"`
	DependsOn  []string `json:"dependsOn,omitempty"`
	GraphNode  string   `json:"graphNode,omitempty"`
	InheritEnv []string `json:"inheritEnv,omitempty"`
}

func LoadDevManifest(path string) (DevManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DevManifest{}, err
	}
	var manifest DevManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return DevManifest{}, err
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = DevSchemaVersion
	}
	if manifest.SchemaVersion != DevSchemaVersion {
		return DevManifest{}, errors.New("devruntime: unsupported manifest schema version")
	}
	return manifest, nil
}

func (manifest DevManifest) Validate(root string, graph applicationgraph.Graph) error {
	names := make(map[string]Process, len(manifest.Processes))
	graphIndex := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		graphIndex[node.ID] = struct{}{}
	}
	for _, process := range manifest.Processes {
		name := strings.TrimSpace(process.Name)
		if name == "" {
			return errors.New("devruntime: process name is required")
		}
		if _, exists := names[name]; exists {
			return fmt.Errorf("devruntime: duplicate process %q", name)
		}
		if len(process.Command) == 0 || strings.TrimSpace(process.Command[0]) == "" {
			return fmt.Errorf("devruntime: process %q command is required", name)
		}
		if _, err := resolveWorkingDir(root, process.WorkingDir); err != nil {
			return fmt.Errorf("devruntime: process %q: %w", name, err)
		}
		if process.GraphNode != "" && len(graphIndex) > 0 {
			if _, ok := graphIndex[process.GraphNode]; !ok {
				return fmt.Errorf("devruntime: process %q graph node %q not found", name, process.GraphNode)
			}
		}
		names[name] = process
	}
	for _, process := range manifest.Processes {
		seen := make(map[string]struct{})
		for _, dependency := range process.DependsOn {
			dependency = strings.TrimSpace(dependency)
			if dependency == "" {
				continue
			}
			if dependency == process.Name {
				return fmt.Errorf("devruntime: process %q depends on itself", process.Name)
			}
			if _, ok := names[dependency]; !ok {
				return fmt.Errorf("devruntime: process %q dependency %q not found", process.Name, dependency)
			}
			if _, duplicate := seen[dependency]; duplicate {
				return fmt.Errorf("devruntime: process %q duplicate dependency %q", process.Name, dependency)
			}
			seen[dependency] = struct{}{}
		}
	}
	return nil
}

func normalizeProcess(process Process) Process {
	process.Name = strings.TrimSpace(process.Name)
	process.WorkingDir = strings.TrimSpace(process.WorkingDir)
	process.GraphNode = strings.TrimSpace(process.GraphNode)
	if len(process.Command) > 0 {
		process.Command[0] = strings.TrimSpace(process.Command[0])
	}
	process.DependsOn = stableStrings(process.DependsOn)
	process.InheritEnv = stableStrings(process.InheritEnv)
	return process
}

func resolveWorkingDir(root, value string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal := root
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		rootReal = resolved
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return root, nil
	}
	if filepath.IsAbs(value) {
		return "", errors.New("workingDir must be relative to project root")
	}
	candidate := filepath.Clean(filepath.Join(root, value))
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("workingDir escapes project root")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
		realRelative, relErr := filepath.Rel(rootReal, resolved)
		if relErr != nil {
			return "", relErr
		}
		if realRelative == ".." || strings.HasPrefix(realRelative, ".."+string(filepath.Separator)) {
			return "", errors.New("workingDir resolves outside project root")
		}
	}
	return candidate, nil
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
