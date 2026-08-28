package applicationgraph

import (
	"testing"

	"yunka.io/pkg/operationplan"
)

func TestAddOperationPlansAddsDeclaredExecutionEvidence(t *testing.T) {
	set := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{{
		OperationID: "device.get", Domain: "device", Application: "management", UseCase: "get_device",
		RequestType: "device.v1.GetDeviceRequest", ResponseType: "device.v1.DeviceDTO",
		Security: operationplan.Security{TenantRequired: true, Permissions: []string{"device.read"}, PermissionMode: "all"},
		Bindings: operationplan.Bindings{RPC: "/device.v1.DeviceApplication/GetDevice", HTTP: []operationplan.HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}}},
	}}}
	builder := NewBuilder()
	if err := AddOperationPlans(builder, set); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	operationID := ID(NodeOperation, "device.get")
	var operation Node
	for _, node := range graph.Nodes {
		if node.ID == operationID {
			operation = node
			break
		}
	}
	if operation.ID == "" || operation.Attributes["operationPlanSchema"] != "1" || operation.Attributes["operationPlanDigest"] == "" {
		t.Fatalf("operation=%+v", operation)
	}
	foundEvidence := false
	for _, evidence := range operation.Evidence {
		if evidence.Type == EvidenceDeclared && evidence.Source == "operation.plan" {
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatalf("operation evidence=%v", operation.Evidence)
	}
	wantEdges := map[string]bool{
		ID(NodeApplication, "device/management") + "|" + string(EdgeContains) + "|" + operationID: false,
		operationID + "|" + string(EdgeRequires) + "|" + ID(NodePermission, "device.read"): false,
		ID(NodeHTTPRoute, "GET /v1/devices/{id}") + "|" + string(EdgeRoutesTo) + "|" + operationID: false,
	}
	for _, edge := range graph.Edges {
		key := edge.From + "|" + string(edge.Kind) + "|" + edge.To
		if _, ok := wantEdges[key]; ok {
			wantEdges[key] = true
		}
	}
	for key, found := range wantEdges {
		if !found {
			t.Fatalf("missing edge %s", key)
		}
	}
}
