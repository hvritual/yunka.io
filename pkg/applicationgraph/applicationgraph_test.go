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

func TestTypedContractGraphUsesDeclaredBusinessIdentities(t *testing.T) {
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files:         []contract.File{{Name: "device.proto", Domain: &contract.DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []contract.Message{
			{Name: "GetMachineRequest", FullName: "device.v1.GetMachineRequest", DTO: &contract.DTODeclaration{Kind: "input"}},
			{Name: "MachineDTO", FullName: "device.v1.MachineDTO", DTO: &contract.DTODeclaration{Kind: "output"}},
		},
		Services: []contract.Service{{
			Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &contract.ApplicationDeclaration{Name: "device_management"},
			Methods: []contract.Method{{
				Name: "GetMachine", FullName: "device.v1.DeviceApplication.GetMachine", Request: "device.v1.GetMachineRequest", Response: "device.v1.MachineDTO",
				HTTP:      []contract.HTTPBinding{{Method: "GET", Path: "/v1/machines/{id}"}},
				Operation: &contract.OperationDeclaration{ID: "device.machine.get", UseCase: "get_machine", Permissions: []string{"device.machine.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
			}},
		}},
	}
	builder := NewBuilder()
	if err := AddContract(builder, manifest); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	domainID := ID(NodeDomain, "device")
	applicationID := ID(NodeApplication, "device/device_management")
	serviceID := ID(NodeService, "device.v1.DeviceApplication")
	operationID := ID(NodeOperation, "device.machine.get")
	requestID := ID(NodeMessage, "device.v1.GetMachineRequest")
	responseID := ID(NodeMessage, "device.v1.MachineDTO")
	permissionID := ID(NodePermission, "device.machine.read")
	routeID := ID(NodeHTTPRoute, "GET /v1/machines/{id}")
	for _, id := range []string{domainID, applicationID, serviceID, operationID, requestID, responseID, permissionID, routeID} {
		node, ok := graph.Node(id)
		if !ok {
			t.Fatalf("typed graph node missing: %s", id)
		}
		for _, evidence := range node.Evidence {
			if evidence.Type != EvidenceDeclared || evidence.Confidence != ConfidenceHigh {
				t.Fatalf("typed PB node must be declared/high: %s %+v", id, evidence)
			}
		}
	}
	wantEdges := map[string]bool{
		domainID + "|" + string(EdgeContains) + "|" + applicationID:    false,
		applicationID + "|" + string(EdgeContains) + "|" + operationID: false,
		serviceID + "|" + string(EdgeExposes) + "|" + operationID:      false,
		operationID + "|" + string(EdgeAccepts) + "|" + requestID:      false,
		operationID + "|" + string(EdgeReturns) + "|" + responseID:     false,
		operationID + "|" + string(EdgeRequires) + "|" + permissionID:  false,
		routeID + "|" + string(EdgeRoutesTo) + "|" + operationID:       false,
	}
	for _, edge := range graph.Edges {
		key := edge.From + "|" + string(edge.Kind) + "|" + edge.To
		if _, ok := wantEdges[key]; ok {
			wantEdges[key] = true
		}
	}
	for key, found := range wantEdges {
		if !found {
			t.Fatalf("typed graph edge missing: %s\n%+v", key, graph.Edges)
		}
	}
	request, _ := graph.Node(requestID)
	response, _ := graph.Node(responseID)
	if request.Attributes["dtoKind"] != "input" || response.Attributes["dtoKind"] != "output" {
		t.Fatalf("DTO classifications missing from graph: request=%+v response=%+v", request.Attributes, response.Attributes)
	}
	operation, _ := graph.Node(operationID)
	if operation.Attributes["rpcMethod"] != "device.v1.DeviceApplication.GetMachine" || operation.Attributes["useCase"] != "get_machine" {
		t.Fatalf("operation declaration metadata missing: %+v", operation.Attributes)
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

func TestAddContractDoesNotMutateTypedPointers(t *testing.T) {
	manifest := contract.Manifest{
		Files:    []contract.File{{Name: "device.proto", Domain: &contract.DomainDeclaration{Name: " device "}}},
		Messages: []contract.Message{{Name: "Request", FullName: "device.Request", DTO: &contract.DTODeclaration{Kind: "input"}}},
		Services: []contract.Service{{Name: "Device", FullName: "device.Device", Domain: " device ", Application: &contract.ApplicationDeclaration{Name: " device_app "}, Methods: []contract.Method{{Name: "Get", FullName: "device.Device.Get", Operation: &contract.OperationDeclaration{ID: " device.get ", UseCase: " get ", Permissions: []string{" device.read "}}}}}},
	}
	builder := NewBuilder()
	if err := AddContract(builder, manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Files[0].Domain.Name != " device " || manifest.Services[0].Application.Name != " device_app " || manifest.Services[0].Methods[0].Operation.ID != " device.get " {
		t.Fatal("contract graph mutated typed declaration pointers")
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

func TestInternalApplicationOperationHasNoTransportExposure(t *testing.T) {
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files:         []contract.File{{Name: "site.proto", Domain: &contract.DomainDeclaration{Name: "site"}}},
		Messages: []contract.Message{
			{Name: "ValidateRequest", FullName: "site.v1.ValidateRequest", DTO: &contract.DTODeclaration{Kind: "input"}},
			{Name: "SiteDTO", FullName: "site.v1.SiteDTO", DTO: &contract.DTODeclaration{Kind: "output"}},
		},
		Services: []contract.Service{{
			Name: "SiteApplication", FullName: "site.v1.SiteApplication", Domain: "site",
			Application: &contract.ApplicationDeclaration{
				Name: "management",
				Operations: []contract.OperationDeclaration{{
					ID: "site.validate", UseCase: "validate_site", Permissions: []string{"site.read"}, PermissionMode: "all",
					RequestType: "site.v1.ValidateRequest", ResponseType: "site.v1.SiteDTO", ApplicationMethod: "Validate",
				}},
			},
		}},
	}
	builder := NewBuilder()
	if err := AddContract(builder, manifest); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	applicationID := ID(NodeApplication, "site/management")
	operationID := ID(NodeOperation, "site.validate")
	node, ok := graph.Node(operationID)
	if !ok || node.Attributes["applicationMethod"] != "Validate" {
		t.Fatalf("internal operation node missing: %+v", node)
	}
	contained := false
	for _, edge := range graph.Edges {
		if edge.From == applicationID && edge.To == operationID && edge.Kind == EdgeContains {
			contained = true
		}
		if edge.To == operationID && edge.Kind == EdgeExposes {
			t.Fatalf("internal operation unexpectedly exposed by transport: %+v", edge)
		}
	}
	if !contained {
		t.Fatalf("application does not contain internal operation: %+v", graph.Edges)
	}
}
