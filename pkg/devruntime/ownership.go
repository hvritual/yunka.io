package devruntime

import (
	"fmt"
	"strings"
)

// validateProcessGraphOwnership rejects ambiguous or lossy ownership declarations.
// Legacy graphNode remains valid. New graphNodes is explicit plural ownership and
// must not be mixed with graphNode on the same process.
func validateProcessGraphOwnership(process Process) error {
	if strings.TrimSpace(process.GraphNode) != "" && len(process.GraphNodes) != 0 {
		return fmt.Errorf("graphNode and graphNodes are mutually exclusive")
	}
	seen := make(map[string]struct{}, len(process.GraphNodes))
	for index, node := range process.GraphNodes {
		node = strings.TrimSpace(node)
		if node == "" {
			return fmt.Errorf("graphNodes[%d] must not be empty", index)
		}
		if _, duplicate := seen[node]; duplicate {
			return fmt.Errorf("duplicate graph node %q", node)
		}
		seen[node] = struct{}{}
	}
	return nil
}

func processOwnedGraphNodes(process Process) []string {
	if len(process.GraphNodes) != 0 {
		return append([]string(nil), process.GraphNodes...)
	}
	if node := strings.TrimSpace(process.GraphNode); node != "" {
		return []string{node}
	}
	return nil
}

func reportOwnedGraphNodes(process ProcessRuntimeReport) []string {
	if len(process.GraphNodes) != 0 {
		return append([]string(nil), process.GraphNodes...)
	}
	if node := strings.TrimSpace(process.GraphNode); node != "" {
		return []string{node}
	}
	return nil
}

func equalOwnedGraphNodes(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index]) != strings.TrimSpace(right[index]) {
			return false
		}
	}
	return true
}
