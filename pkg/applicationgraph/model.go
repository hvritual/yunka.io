package applicationgraph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const SchemaVersion = 1

type NodeKind string

const (
	NodeApplication     NodeKind = "application"
	NodeDomain          NodeKind = "domain"
	NodePermission      NodeKind = "permission"
	NodeProcess         NodeKind = "process"
	NodeModule          NodeKind = "module"
	NodeService         NodeKind = "service"
	NodeOperation       NodeKind = "operation"
	NodeMessage         NodeKind = "message"
	NodeEnum            NodeKind = "enum"
	NodeHTTPRoute       NodeKind = "http_route"
	NodeRuntimeRoute    NodeKind = "runtime_route"
	NodeSelectorService NodeKind = "selector_service"
	NodeInstance        NodeKind = "instance"
	NodePolicy          NodeKind = "resilience_policy"
	NodeEventTopic      NodeKind = "event_topic"
)

type EdgeKind string

const (
	EdgeContains   EdgeKind = "contains"
	EdgeExposes    EdgeKind = "exposes"
	EdgeAccepts    EdgeKind = "accepts"
	EdgeReturns    EdgeKind = "returns"
	EdgeRoutesTo   EdgeKind = "routes_to"
	EdgeRequires   EdgeKind = "requires"
	EdgeSelects    EdgeKind = "selects"
	EdgeGovernedBy EdgeKind = "governed_by"
	EdgePublishes  EdgeKind = "publishes"
	EdgeConsumes   EdgeKind = "consumes"
	EdgeDependsOn  EdgeKind = "depends_on"
	EdgeRuns       EdgeKind = "runs"
)

type EvidenceType string

const (
	EvidenceDeclared EvidenceType = "declared"
	EvidenceObserved EvidenceType = "observed"
	EvidenceInferred EvidenceType = "inferred"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type Evidence struct {
	Type       EvidenceType `json:"type"`
	Source     string       `json:"source"`
	Confidence Confidence   `json:"confidence"`
	Detail     string       `json:"detail,omitempty"`
}

type Node struct {
	ID         string            `json:"id"`
	Kind       NodeKind          `json:"kind"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Evidence   []Evidence        `json:"evidence,omitempty"`
}

type Edge struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Kind       EdgeKind          `json:"kind"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Evidence   []Evidence        `json:"evidence,omitempty"`
}

type Graph struct {
	SchemaVersion int    `json:"schemaVersion"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

func ID(kind NodeKind, name string) string {
	return string(kind) + ":" + strings.TrimSpace(name)
}

func Declared(source, detail string) Evidence {
	return Evidence{Type: EvidenceDeclared, Source: strings.TrimSpace(source), Confidence: ConfidenceHigh, Detail: strings.TrimSpace(detail)}
}

func Observed(source, detail string) Evidence {
	return Evidence{Type: EvidenceObserved, Source: strings.TrimSpace(source), Confidence: ConfidenceHigh, Detail: strings.TrimSpace(detail)}
}

func Inferred(source, detail string, confidence Confidence) Evidence {
	if confidence == "" {
		confidence = ConfidenceLow
	}
	return Evidence{Type: EvidenceInferred, Source: strings.TrimSpace(source), Confidence: confidence, Detail: strings.TrimSpace(detail)}
}

type Builder struct {
	nodes map[string]Node
	edges map[string]Edge
}

func NewBuilder() *Builder {
	return &Builder{nodes: make(map[string]Node), edges: make(map[string]Edge)}
}

func (builder *Builder) AddNode(node Node) error {
	if builder == nil {
		return errors.New("applicationgraph: nil builder")
	}
	node.ID = strings.TrimSpace(node.ID)
	node.Name = strings.TrimSpace(node.Name)
	if node.ID == "" || node.Kind == "" || node.Name == "" {
		return errors.New("applicationgraph: node id, kind and name are required")
	}
	current, exists := builder.nodes[node.ID]
	if exists && current.Kind != node.Kind {
		return fmt.Errorf("applicationgraph: node %q kind conflict: %s != %s", node.ID, current.Kind, node.Kind)
	}
	if !exists {
		current = Node{ID: node.ID, Kind: node.Kind, Name: node.Name}
	}
	if current.Name == "" {
		current.Name = node.Name
	} else if current.Name != node.Name {
		return fmt.Errorf("applicationgraph: node %q name conflict: %q != %q", node.ID, current.Name, node.Name)
	}
	attributes, err := mergeAttributes(current.Attributes, node.Attributes)
	if err != nil {
		return fmt.Errorf("applicationgraph: node %q: %w", node.ID, err)
	}
	current.Attributes = attributes
	current.Evidence = mergeEvidence(current.Evidence, node.Evidence)
	builder.nodes[node.ID] = current
	return nil
}

func (builder *Builder) HasNode(id string) bool {
	if builder == nil {
		return false
	}
	_, ok := builder.nodes[strings.TrimSpace(id)]
	return ok
}

func (builder *Builder) AddEdge(edge Edge) error {
	if builder == nil {
		return errors.New("applicationgraph: nil builder")
	}
	edge.From = strings.TrimSpace(edge.From)
	edge.To = strings.TrimSpace(edge.To)
	if edge.From == "" || edge.To == "" || edge.Kind == "" {
		return errors.New("applicationgraph: edge from, to and kind are required")
	}
	key := edge.From + "\x00" + string(edge.Kind) + "\x00" + edge.To
	current := builder.edges[key]
	current.From, current.To, current.Kind = edge.From, edge.To, edge.Kind
	attributes, err := mergeAttributes(current.Attributes, edge.Attributes)
	if err != nil {
		return fmt.Errorf("applicationgraph: edge %s --%s--> %s: %w", edge.From, edge.Kind, edge.To, err)
	}
	current.Attributes = attributes
	current.Evidence = mergeEvidence(current.Evidence, edge.Evidence)
	builder.edges[key] = current
	return nil
}

func (builder *Builder) Build() (Graph, error) {
	if builder == nil {
		return Graph{}, errors.New("applicationgraph: nil builder")
	}
	graph := Graph{SchemaVersion: SchemaVersion}
	for _, node := range builder.nodes {
		node.Evidence = normalizedEvidence(node.Evidence)
		graph.Nodes = append(graph.Nodes, node)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	for _, edge := range builder.edges {
		if !builder.HasNode(edge.From) || !builder.HasNode(edge.To) {
			return Graph{}, fmt.Errorf("applicationgraph: edge %s --%s--> %s references missing node", edge.From, edge.Kind, edge.To)
		}
		edge.Evidence = normalizedEvidence(edge.Evidence)
		graph.Edges = append(graph.Edges, edge)
	}
	sort.Slice(graph.Edges, func(i, j int) bool {
		left, right := graph.Edges[i], graph.Edges[j]
		if left.From != right.From {
			return left.From < right.From
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.To < right.To
	})
	return graph, nil
}

func mergeAttributes(current, incoming map[string]string) (map[string]string, error) {
	if len(incoming) == 0 {
		return current, nil
	}
	result := make(map[string]string, len(current)+len(incoming))
	for key, value := range current {
		result[key] = value
	}
	for key, value := range incoming {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if existing, ok := result[key]; ok && existing != value {
			return current, fmt.Errorf("attribute %q conflict: %q != %q", key, existing, value)
		}
		result[key] = value
	}
	return result, nil
}

func mergeEvidence(current, incoming []Evidence) []Evidence {
	return normalizedEvidence(append(append([]Evidence(nil), current...), incoming...))
}

func normalizedEvidence(values []Evidence) []Evidence {
	seen := make(map[string]struct{}, len(values))
	result := make([]Evidence, 0, len(values))
	for _, evidence := range values {
		evidence.Source = strings.TrimSpace(evidence.Source)
		evidence.Detail = strings.TrimSpace(evidence.Detail)
		if evidence.Type == "" || evidence.Source == "" {
			continue
		}
		if evidence.Confidence == "" {
			evidence.Confidence = ConfidenceLow
		}
		key := string(evidence.Type) + "\x00" + evidence.Source + "\x00" + string(evidence.Confidence) + "\x00" + evidence.Detail
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, evidence)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		if result[i].Source != result[j].Source {
			return result[i].Source < result[j].Source
		}
		if result[i].Confidence != result[j].Confidence {
			return result[i].Confidence < result[j].Confidence
		}
		return result[i].Detail < result[j].Detail
	})
	return result
}

func (graph Graph) Validate() error {
	if graph.SchemaVersion != SchemaVersion {
		return fmt.Errorf("applicationgraph: unsupported schema version %d", graph.SchemaVersion)
	}
	nodes := make(map[string]Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if strings.TrimSpace(node.ID) == "" || node.Kind == "" || strings.TrimSpace(node.Name) == "" {
			return errors.New("applicationgraph: invalid node")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("applicationgraph: duplicate node %q", node.ID)
		}
		nodes[node.ID] = node
	}
	edges := make(map[string]struct{}, len(graph.Edges))
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.From]; !ok {
			return fmt.Errorf("applicationgraph: edge source %q not found", edge.From)
		}
		if _, ok := nodes[edge.To]; !ok {
			return fmt.Errorf("applicationgraph: edge target %q not found", edge.To)
		}
		key := edge.From + "\x00" + string(edge.Kind) + "\x00" + edge.To
		if _, exists := edges[key]; exists {
			return fmt.Errorf("applicationgraph: duplicate edge %s --%s--> %s", edge.From, edge.Kind, edge.To)
		}
		edges[key] = struct{}{}
	}
	return nil
}
