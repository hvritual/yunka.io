package contract

import (
	"strings"
	"testing"
)

func TestRenderApplicationCodeProducesSharedPortAdaptersAndPolicy(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{
			{Name: "GetMachineRequest", FullName: "device.v1.GetMachineRequest", DTO: &DTODeclaration{Kind: "input"}, Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "MachineDTO", FullName: "device.v1.MachineDTO", DTO: &DTODeclaration{Kind: "output"}, Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
		},
		Services: []Service{{
			Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "device_management"},
			Methods: []Method{{
				Name: "GetMachine", FullName: "device.v1.DeviceApplication.GetMachine", Request: "device.v1.GetMachineRequest", Response: "device.v1.MachineDTO",
				HTTP:          []HTTPBinding{{Method: "GET", Path: "/v1/machines/{id}"}},
				Operation:     &OperationDeclaration{ID: "device.machine.get", UseCase: "get_machine", Permissions: []string{"device.machine.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
				Authorization: &AuthorizationPolicy{OperationID: "device.machine.get", Permissions: []string{"device.machine.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
			}},
		}},
	}
	files, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("generated files=%d want=4", len(files))
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
		if !strings.HasPrefix(string(file.Content), GeneratedApplicationMarker) {
			t.Fatalf("generated marker missing from %s", file.Path)
		}
	}
	port := byPath["device/application/zz_yunka_device_management_application_port_gen.go"]
	if !strings.Contains(port, "type DeviceApplication interface") || !strings.Contains(port, "GetMachine(context.Context, *devicev1.GetMachineRequest) (*devicev1.MachineDTO, error)") {
		t.Fatalf("unexpected application port:\n%s", port)
	}
	policy := byPath["device/policy/zz_yunka_device_management_operation_policy_gen.go"]
	if !strings.Contains(policy, `OperationGetMachine authz.OperationID = "device.machine.get"`) || !strings.Contains(policy, `"device.machine.read"`) || !strings.Contains(policy, `"/device.v1.DeviceApplication/GetMachine"`) || !strings.Contains(policy, "func Permissions() []authz.PermissionKey") {
		t.Fatalf("unexpected policy:\n%s", policy)
	}
	rpc := byPath["device/transport/rpc/zz_yunka_device_management_rpc_adapter_gen.go"]
	if !strings.Contains(rpc, "return server.application.GetMachine(ctx, request)") {
		t.Fatalf("RPC does not delegate to shared port:\n%s", rpc)
	}
	rest := byPath["device/transport/rest/zz_yunka_device_management_rest_adapter_gen.go"]
	if !strings.Contains(rest, `handler.runtime.Prepare(request.Context(), "/device.v1.DeviceApplication/GetMachine", wire)`) || !strings.Contains(rest, "handler.application.GetMachine(secured, wire)") || !strings.Contains(rest, "runtime     authz.OperationRuntime") {
		t.Fatalf("REST does not use shared C8.5 operation runtime before application:\n%s", rest)
	}
	if strings.Contains(rest, "handler.authorizer") || strings.Contains(rest, "ResolvePolicy") {
		t.Fatalf("REST retained transport-local authorization logic:\n%s", rest)
	}
	if strings.Contains(rest, `strconv "strconv"`) {
		t.Fatalf("string-only REST adapter imported strconv:\n%s", rest)
	}
}

func TestRenderApplicationCodeRejectsBusinessSemanticBodyMapping(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device"}}},
		Messages:      []Message{{Name: "Request", FullName: "device.v1.Request"}, {Name: "Response", FullName: "device.v1.Response"}},
		Services:      []Service{{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "device"}, Methods: []Method{{Name: "Update", FullName: "device.v1.DeviceApplication.Update", Request: "device.v1.Request", Response: "device.v1.Response", HTTP: []HTTPBinding{{Method: "PATCH", Path: "/v1/device", Body: "payload"}}, Operation: &OperationDeclaration{ID: "device.update", UseCase: "update", Permissions: []string{"device.write"}, PermissionMode: "all"}}}}},
	}
	if _, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"}); err == nil || !strings.Contains(err.Error(), "handwritten mapping") {
		t.Fatalf("expected explicit handwritten mapping rejection, got %v", err)
	}
}

func TestRenderRESTAppliesPathAfterWholeBody(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device"}}},
		Messages: []Message{
			{Name: "UpdateMachineRequest", FullName: "device.v1.UpdateMachineRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}, {Name: "serial", JSONName: "serial", Number: 2, Kind: "scalar", Type: "string"}}},
			{Name: "MachineDTO", FullName: "device.v1.MachineDTO"},
		},
		Services: []Service{{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "device"}, Methods: []Method{{
			Name: "UpdateMachine", FullName: "device.v1.DeviceApplication.UpdateMachine", Request: "device.v1.UpdateMachineRequest", Response: "device.v1.MachineDTO",
			HTTP:          []HTTPBinding{{Method: "PUT", Path: "/v1/machines/{id}", Body: "*"}},
			Operation:     &OperationDeclaration{ID: "device.machine.update", UseCase: "update_machine", Permissions: []string{"device.machine.write"}, PermissionMode: "all"},
			Authorization: &AuthorizationPolicy{OperationID: "device.machine.update", Permissions: []string{"device.machine.write"}, PermissionMode: "all"},
		}}}},
	}
	files, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	var rest string
	for _, file := range files {
		if strings.Contains(file.Path, "/transport/rest/") {
			rest = string(file.Content)
			break
		}
	}
	bodyIndex := strings.Index(rest, "protojson.Unmarshal(body, wire)")
	pathIndex := strings.Index(rest, `wire.Id = request.PathValue("id")`)
	if bodyIndex < 0 || pathIndex < 0 {
		t.Fatalf("expected body and path assignment in REST adapter:\n%s", rest)
	}
	if pathIndex <= bodyIndex {
		t.Fatalf("path binding must override whole-body values: bodyIndex=%d pathIndex=%d\n%s", bodyIndex, pathIndex, rest)
	}
}

func TestRenderRESTImportsStrconvForParsedScalarOnly(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device"}}},
		Messages: []Message{
			{Name: "GetMachineRequest", FullName: "device.v1.GetMachineRequest", Fields: []Field{{Name: "slot", JSONName: "slot", Number: 1, Kind: "scalar", Type: "int32"}}},
			{Name: "MachineDTO", FullName: "device.v1.MachineDTO"},
		},
		Services: []Service{{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "device"}, Methods: []Method{{
			Name: "GetMachine", FullName: "device.v1.DeviceApplication.GetMachine", Request: "device.v1.GetMachineRequest", Response: "device.v1.MachineDTO",
			HTTP:          []HTTPBinding{{Method: "GET", Path: "/v1/machines/{slot}"}},
			Operation:     &OperationDeclaration{ID: "device.machine.get", UseCase: "get_machine", Permissions: []string{"device.machine.read"}, PermissionMode: "all"},
			Authorization: &AuthorizationPolicy{OperationID: "device.machine.get", Permissions: []string{"device.machine.read"}, PermissionMode: "all"},
		}}}},
	}
	files, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	var rest string
	for _, file := range files {
		if strings.Contains(file.Path, "/transport/rest/") {
			rest = string(file.Content)
			break
		}
	}
	if !strings.Contains(rest, `strconv "strconv"`) || !strings.Contains(rest, "strconv.ParseInt") {
		t.Fatalf("numeric REST adapter must import and use strconv:\n%s", rest)
	}
}
