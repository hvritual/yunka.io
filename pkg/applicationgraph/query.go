package applicationgraph

import (
	"errors"
	"sort"
	"strings"
)

type Direction string

const (
	DirectionDependencies Direction = "dependencies"
	DirectionDependents   Direction = "dependents"
)

type ImpactEntry struct {
	Node     Node     `json:"node"`
	Depth    int      `json:"depth"`
	Via      EdgeKind `json:"via"`
	ParentID string   `json:"parentId,omitempty"`
}

type ImpactReport struct {
	Target       Node          `json:"target"`
	Dependencies []ImpactEntry `json:"dependencies,omitempty"`
	Dependents   []ImpactEntry `json:"dependents,omitempty"`
}

func (graph Graph) Node(id string) (Node, bool) {
	id = strings.TrimSpace(id)
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func (graph Graph) Find(query string, kind NodeKind) []Node {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]Node, 0)
	for _, node := range graph.Nodes {
		if kind != "" && node.Kind != kind {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(node.ID), query) || strings.Contains(strings.ToLower(node.Name), query) {
			result = append(result, node)
			continue
		}
		for key, value := range node.Attributes {
			if strings.Contains(strings.ToLower(key), query) || strings.Contains(strings.ToLower(value), query) {
				result = append(result, node)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (graph Graph) Impact(id string, maxDepth int) (ImpactReport, error) {
	target, ok := graph.Node(id)
	if !ok {
		return ImpactReport{}, errors.New("applicationgraph: target node not found")
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return ImpactReport{
		Target:       target,
		Dependencies: graph.walk(id, maxDepth, DirectionDependencies),
		Dependents:   graph.walk(id, maxDepth, DirectionDependents),
	}, nil
}

func (graph Graph) walk(start string, maxDepth int, direction Direction) []ImpactEntry {
	nodeByID := make(map[string]Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeByID[node.ID] = node
	}
	type state struct {
		id, parent string
		depth      int
		via        EdgeKind
	}
	queue := []state{{id: start}}
	visited := map[string]bool{start: true}
	result := make([]ImpactEntry, 0)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}
		candidates := make([]state, 0)
		for _, edge := range graph.Edges {
			var next string
			if direction == DirectionDependencies && edge.From == current.id {
				next = edge.To
			}
			if direction == DirectionDependents && edge.To == current.id {
				next = edge.From
			}
			if next == "" || visited[next] {
				continue
			}
			candidates = append(candidates, state{id: next, parent: current.id, depth: current.depth + 1, via: edge.Kind})
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
		for _, candidate := range candidates {
			visited[candidate.id] = true
			if node, ok := nodeByID[candidate.id]; ok {
				result = append(result, ImpactEntry{Node: node, Depth: candidate.depth, Via: candidate.via, ParentID: candidate.parent})
				queue = append(queue, candidate)
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Depth != result[j].Depth {
			return result[i].Depth < result[j].Depth
		}
		return result[i].Node.ID < result[j].Node.ID
	})
	return result
}
