package change

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
	"yunka.io/app/cmd/projectflow"
)

func TestBuildFromFactsPlansExistingOperationWithoutSemanticGuessing(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	mustWrite(t, filepath.Join(root, "contracts", "proto", "device.proto"), "syntax = \"proto3\";\n")

	inputs := testInputs(root, []string{"contracts/proto/device.proto"})
	plan, err := buildFromFacts(inputs, testGraph(), "device.machine.get", IntentBoth, 3)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != SchemaVersion || plan.Operation.ID != "operation:device.machine.get" || plan.Intent != IntentBoth {
		t.Fatalf("plan identity=%#v", plan)
	}
	if len(plan.EditableTargets) != 1 || plan.EditableTargets[0].Path != "contracts/proto/device.proto" || plan.EditableTargets[0].Owner != "developer-contract" {
		t.Fatalf("editable targets=%#v", plan.EditableTargets)
	}
	if len(plan.UnresolvedTargets) != 1 || plan.UnresolvedTargets[0].Kind != "implementation" {
		t.Fatalf("unresolved targets=%#v", plan.UnresolvedTargets)
	}
	if plan.UnresolvedTargets[0].Scope != "internal/device/application" {
		t.Fatalf("implementation scope=%q", plan.UnresolvedTargets[0].Scope)
	}
	if len(plan.Affected.Dependencies) == 0 || len(plan.Affected.Dependents) == 0 {
		t.Fatalf("impact=%#v", plan.Affected)
	}
	for _, expected := range []string{"permissionMode", "tenantRequired", "transaction", "idempotency"} {
		if !hasRisk(plan.Risks, expected) {
			t.Fatalf("missing canonical risk %s: %#v", expected, plan.Risks)
		}
	}
	if !hasGate(plan.Gates, "yunka ownership check") || !hasGate(plan.Gates, "yunka generate") || !hasGate(plan.Gates, "yunka check --format agent-json") || !hasGate(plan.Gates, "go test ./...") || !hasGate(plan.Gates, "yunka dev") {
		t.Fatalf("gates=%#v", plan.Gates)
	}
	first, err := Render(plan, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(plan, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("change plan JSON is not deterministic")
	}
	var decoded Plan
	if err := json.Unmarshal([]byte(first), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Operation.Attributes["operationId"] != "device.machine.get" {
		t.Fatalf("operation attributes=%#v", decoded.Operation.Attributes)
	}
}

func TestBuildFromFactsFailsClosedWhenContractSourceIsAmbiguous(t *testing.T) {
	root := t.TempDir()
	inputs := testInputs(root, []string{"contracts/proto/a.proto", "contracts/proto/b.proto"})
	plan, err := buildFromFacts(inputs, testGraph(), "operation:device.machine.get", IntentContract, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.EditableTargets) != 0 || len(plan.UnresolvedTargets) != 1 {
		t.Fatalf("targets editable=%#v unresolved=%#v", plan.EditableTargets, plan.UnresolvedTargets)
	}
	unresolved := plan.UnresolvedTargets[0]
	if unresolved.Kind != "contract-source" || len(unresolved.Candidates) != 2 {
		t.Fatalf("unresolved=%#v", unresolved)
	}
	if !strings.Contains(unresolved.Reason, "do not identify which protobuf source") {
		t.Fatalf("reason=%q", unresolved.Reason)
	}
	if hasGate(plan.Gates, "yunka ownership check") {
		t.Fatalf("ambiguous source must not produce an executable ownership target gate: %#v", plan.Gates)
	}
}

func TestBuildFromFactsRejectsUndeclaredOperationAndInvalidIntent(t *testing.T) {
	inputs := testInputs(t.TempDir(), nil)
	_, err := buildFromFacts(inputs, testGraph(), "device.machine.suspend", IntentBoth, 3)
	if err == nil {
		t.Fatal("expected undeclared operation failure")
	}
	item := Diagnose(err)
	if item.Code != diagnostic.CodeChangeOperation || !strings.Contains(item.Detail, "does not infer plans for undeclared operations") {
		t.Fatalf("diagnostic=%#v", item)
	}

	_, err = Build(inputs.Project.Root, "device.machine.get", "semantic-magic", 3)
	if err == nil || Diagnose(err).Code != diagnostic.CodeChangeIntent {
		t.Fatalf("invalid intent err=%v diagnostic=%#v", err, Diagnose(err))
	}
}

func TestBuildFailsWithStableEvidenceDiagnosticWhenCanonicalArtifactsAreMissing(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Build(root, "device.machine.get", IntentBoth, 3)
	if err == nil {
		t.Fatal("expected missing canonical evidence failure")
	}
	item := Diagnose(err)
	if item.Code != diagnostic.CodeChangeEvidence || item.Location == nil || item.Location.Path != "contracts/generated/manifest.json" {
		t.Fatalf("diagnostic=%#v", item)
	}
}

func testInputs(root string, sources []string) projectflow.OwnershipInputs {
	return projectflow.OwnershipInputs{
		Project: projectflow.ProjectDescriptor{
			Root:               root,
			GoModule:           "example.com/demo",
			ContractSourceKind: "proto-root",
			ContractSource:     "contracts/proto",
			ContractGenerated:  "contracts/generated",
			GeneratedGoRoot:    "internal",
		},
		ContractSourceFiles: append([]string(nil), sources...),
	}
}

func testGraph() applicationgraph.Graph {
	operationID := applicationgraph.ID(applicationgraph.NodeOperation, "device.machine.get")
	applicationID := applicationgraph.ID(applicationgraph.NodeApplication, "device/device_management")
	permissionID := applicationgraph.ID(applicationgraph.NodePermission, "device.machine.read")
	routeID := applicationgraph.ID(applicationgraph.NodeHTTPRoute, "GET /v1/machines/{id}")
	serviceID := applicationgraph.ID(applicationgraph.NodeService, "device.v1.DeviceApplication")
	return applicationgraph.Graph{
		SchemaVersion: applicationgraph.SchemaVersion,
		Nodes: []applicationgraph.Node{
			{ID: applicationID, Kind: applicationgraph.NodeApplication, Name: "device/device_management"},
			{ID: operationID, Kind: applicationgraph.NodeOperation, Name: "device.machine.get", Attributes: map[string]string{
				"operationId": "device.machine.get", "domain": "device", "application": "device_management", "permissionMode": "all", "tenantRequired": "true", "public": "false", "transaction": "required", "idempotency": "keyed",
			}},
			{ID: permissionID, Kind: applicationgraph.NodePermission, Name: "device.machine.read"},
			{ID: routeID, Kind: applicationgraph.NodeHTTPRoute, Name: "GET /v1/machines/{id}"},
			{ID: serviceID, Kind: applicationgraph.NodeService, Name: "device.v1.DeviceApplication"},
		},
		Edges: []applicationgraph.Edge{
			{From: applicationID, To: operationID, Kind: applicationgraph.EdgeContains},
			{From: operationID, To: permissionID, Kind: applicationgraph.EdgeRequires},
			{From: routeID, To: operationID, Kind: applicationgraph.EdgeRoutesTo},
			{From: serviceID, To: operationID, Kind: applicationgraph.EdgeExposes},
		},
	}
}

func hasRisk(risks []Risk, kind string) bool {
	for _, risk := range risks {
		if risk.Kind == kind {
			return true
		}
	}
	return false
}

func hasGate(gates []Gate, prefix string) bool {
	for _, gate := range gates {
		if strings.HasPrefix(gate.Command, prefix) {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAG062ChangePlanResolvesExistingSealedHandlerAndGovernanceGates(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	mustWrite(t, filepath.Join(root, ".yunka", "source-policy.json"), "{}\n")
	layout, err := projectflow.DescribeImplementationLayout(testInputs(root, nil).Project, "device", "device_management", "GetMachine")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{layout.Build, layout.TypePolicy, layout.Handler} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(path)), "package placeholder\n")
	}
	graph := testGraph()
	for i := range graph.Nodes {
		if graph.Nodes[i].Kind == applicationgraph.NodeOperation {
			graph.Nodes[i].Attributes["applicationMethod"] = "GetMachine"
		}
	}
	plan, err := buildFromFacts(testInputs(root, nil), graph, "device.machine.get", IntentImplementation, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.UnresolvedTargets) != 0 || len(plan.EditableTargets) != 1 || plan.EditableTargets[0].Path != layout.Handler {
		t.Fatalf("targets editable=%#v unresolved=%#v", plan.EditableTargets, plan.UnresolvedTargets)
	}
	if !hasGate(plan.Gates, "yunka audit source --root . --format agent-json") || !hasGate(plan.Gates, "yunka audit types --root . --policy") {
		t.Fatalf("governance gates=%#v", plan.Gates)
	}
}

func TestAG062ChangePlanFailsClosedOnPartialSealedStarter(t *testing.T) {
	root := t.TempDir()
	inputs := testInputs(root, nil)
	layout, err := projectflow.DescribeImplementationLayout(inputs.Project, "device", "device_management", "GetMachine")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, filepath.FromSlash(layout.Build)), "package owner\n")
	graph := testGraph()
	for i := range graph.Nodes {
		if graph.Nodes[i].Kind == applicationgraph.NodeOperation {
			graph.Nodes[i].Attributes["applicationMethod"] = "GetMachine"
		}
	}
	plan, err := buildFromFacts(inputs, graph, "device.machine.get", IntentImplementation, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.EditableTargets) != 0 || len(plan.UnresolvedTargets) != 1 || !strings.Contains(plan.UnresolvedTargets[0].Reason, "partial") {
		t.Fatalf("partial starter targets=%#v %#v", plan.EditableTargets, plan.UnresolvedTargets)
	}
}

func TestAG062IntentBothPreservesSealedImplementationTarget(t *testing.T) {
	targets := []EditableTarget{
		{Path: "contracts/proto/device.proto", Owner: "developer-contract"},
		{Path: "internal/device/application/device_management/internal/usecase/getmachine_handler.go", Owner: "developer-implementation"},
	}
	got := preserveImplementationTargets(targets, []string{"contracts/proto/device.proto"})
	if len(got) != 1 || got[0].Path != targets[1].Path || got[0].Owner != "developer-implementation" {
		t.Fatalf("implementation targets=%#v", got)
	}
}
