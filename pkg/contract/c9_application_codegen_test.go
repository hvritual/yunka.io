package contract

import (
	"strings"
	"testing"
)

func TestRenderC9ApplicationCodeEmitsOnlyExecutorTransports(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{
			{Name: "GetDeviceRequest", FullName: "device.v1.GetDeviceRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "DeviceDTO", FullName: "device.v1.DeviceDTO"},
		},
		Services: []Service{{
			Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "management"},
			Methods: []Method{{
				Name: "GetDevice", FullName: "device.v1.DeviceApplication.GetDevice", Request: "device.v1.GetDeviceRequest", Response: "device.v1.DeviceDTO",
				HTTP: []HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}},
				Operation: &OperationDeclaration{ID: "device.get", UseCase: "get_device", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
			}},
		}},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := make(map[string]string, len(files))
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
		if strings.HasSuffix(file.Path, "_rest_adapter_gen.go") || strings.HasSuffix(file.Path, "_rpc_adapter_gen.go") {
			t.Fatalf("legacy C8 transport leaked into canonical C9 generation: %s", file.Path)
		}
	}
	if _, ok := byPath["device/application/zz_yunka_management_application_port_gen.go"]; !ok {
		t.Fatal("application port compatibility artifact missing")
	}
	if _, ok := byPath["device/policy/zz_yunka_management_operation_policy_gen.go"]; !ok {
		t.Fatal("static policy compatibility artifact missing")
	}
	plan := byPath["device/policy/zz_yunka_management_operation_plan_gen.go"]
	if !strings.Contains(plan, `OperationID: "device.get"`) || !strings.Contains(plan, `RPC: "/device.v1.DeviceApplication/GetDevice"`) {
		t.Fatalf("compiled operation plan missing canonical identity/binding:\n%s", plan)
	}
	rpc := byPath["device/transport/rpc/zz_yunka_management_operation_executor_gen.go"]
	if !strings.Contains(rpc, "operation.ExecuteTyped") || strings.Contains(rpc, "runtime.Prepare") {
		t.Fatalf("RPC is not executor-backed:\n%s", rpc)
	}
	rest := byPath["device/transport/rest/zz_yunka_management_operation_executor_gen.go"]
	if !strings.Contains(rest, "operation.ExecuteTyped") || strings.Contains(rest, "runtime.Prepare") {
		t.Fatalf("REST is not executor-backed:\n%s", rest)
	}
}
