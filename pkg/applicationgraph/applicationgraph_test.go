package applicationgraph

import (
	"reflect"
	"testing"

	"yunka.io/pkg/contract"
)

func testManifest() contract.Manifest {
	return contract.Manifest{Services: []contract.Service{{Name: "DeviceService", FullName: "device.DeviceService", Methods: []contract.Method{{
		Name: "GetDevice", FullName: "device.DeviceService.GetDevice", Request: "device.GetDeviceRequest", Response: "device.GetDeviceResponse",
		HTTP: []contract.HTTPBinding{{Method: "get", Path: "/v1/devices/{id}"}},
	}}}}, Messages: []contract.Message{{Name: "GetDeviceRequest", FullName: "device.GetDeviceRequest"}, {Name: "GetDeviceResponse", FullName: "device.GetDeviceResponse"}}}
}

func TestContractGraphAndImpact(t *testing.T) {
	builder := NewBuilder()
	if err := AddContract(builder, testManifest()); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	serviceID := ID(NodeService, "device.DeviceService")
	operationID := ID(NodeOperation, "device.DeviceService.GetDevice")
	routeID := ID(NodeHTTPRoute, "GET /v1/devices/{id}")
	if _, ok := graph.Node(serviceID); !ok {
		t.Fatalf("service missing")
	}
	if _, ok := graph.Node(routeID); !ok {
		t.Fatalf("route missing")
	}
	report, err := graph.Impact(operationID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Dependencies) != 2 {
		t.Fatalf("dependencies=%d want 2", len(report.Dependencies))
	}
	if len(report.Dependents) != 2 {
		t.Fatalf("dependents=%d want 2", len(report.Dependents))
	}
}

func TestRuntimeGraphDoesNotInventServiceOwnership(t *testing.T) {
	builder := NewBuilder()
	snapshot := RuntimeSnapshot{State: "ready", Modules: []RuntimeModule{{Name: "device", Startable: true}}, Routes: []string{"/internal/health"}, Runtime: RuntimeInventory{RouteCount: 1}}
	if err := AddRuntime(builder, snapshot, "app"); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("edges=%d want 2", len(graph.Edges))
	}
	for _, edge := range graph.Edges {
		if edge.Kind != EdgeContains && edge.Kind != EdgeExposes {
			t.Fatalf("unexpected edge %+v", edge)
		}
	}
}

func TestBuilderMergesEvidenceDeterministically(t *testing.T) {
	builder := NewBuilder()
	id := ID(NodeService, "svc")
	first := Node{ID: id, Kind: NodeService, Name: "svc", Evidence: []Evidence{Observed("runtime", "seen")}}
	second := Node{ID: id, Kind: NodeService, Name: "svc", Evidence: []Evidence{Declared("contract", "declared"), Observed("runtime", "seen")}}
	if err := builder.AddNode(first); err != nil {
		t.Fatal(err)
	}
	if err := builder.AddNode(second); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(graph.Nodes[0].Evidence); got != 2 {
		t.Fatalf("evidence=%d", got)
	}
	if !reflect.DeepEqual(graph.Nodes[0].Evidence, normalizedEvidence(graph.Nodes[0].Evidence)) {
		t.Fatal("evidence not normalized")
	}
}

func TestAddContractDoesNotMutateCallerManifest(t *testing.T) {
	manifest := contract.Manifest{Services: []contract.Service{
		{Name: "z", FullName: "z"},
		{Name: "a", FullName: "a"},
	}}
	builder := NewBuilder()
	if err := AddContract(builder, manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Services[0].FullName != "z" {
		t.Fatal("contract graph mutated caller manifest")
	}
}

func TestBuilderRejectsConflictingAttributesWithoutMutation(t *testing.T) {
	builder := NewBuilder()
	id := ID(NodeService, "svc")
	if err := builder.AddNode(Node{ID: id, Kind: NodeService, Name: "svc", Attributes: map[string]string{"version": "v1"}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.AddNode(Node{ID: id, Kind: NodeService, Name: "svc", Attributes: map[string]string{"version": "v2", "new": "value"}}); err == nil {
		t.Fatal("expected conflict")
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if graph.Nodes[0].Attributes["version"] != "v1" {
		t.Fatal("existing attribute changed after conflict")
	}
	if _, ok := graph.Nodes[0].Attributes["new"]; ok {
		t.Fatal("partial attribute update leaked after conflict")
	}
}

func TestProcessGraphVocabularyIsAdditive(t *testing.T) {
	builder := NewBuilder()
	appID := ID(NodeApplication, "local")
	processID := ID(NodeProcess, "api")
	serviceID := ID(NodeService, "example.ApiService")
	for _, node := range []Node{
		{ID: appID, Kind: NodeApplication, Name: "local"},
		{ID: processID, Kind: NodeProcess, Name: "api"},
		{ID: serviceID, Kind: NodeService, Name: "example.ApiService"},
	} {
		if err := builder.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	if err := builder.AddEdge(Edge{From: appID, To: processID, Kind: EdgeContains, Evidence: []Evidence{Declared("dev.manifest", "process")}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.AddEdge(Edge{From: processID, To: serviceID, Kind: EdgeRuns, Evidence: []Evidence{Declared("dev.manifest", "ownership")}}); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 3 || len(graph.Edges) != 2 {
		t.Fatalf("nodes=%d edges=%d", len(graph.Nodes), len(graph.Edges))
	}
}
