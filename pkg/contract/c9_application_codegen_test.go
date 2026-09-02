package contract

import (
	"strings"
	"testing"
)

func TestRenderC9ApplicationCodeEmitsOnlyExecutorTransports(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{
			{Name: "GetDeviceRequest", FullName: "device.v1.GetDeviceRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "DeviceDTO", FullName: "device.v1.DeviceDTO"},
		},
		Services: []Service{{
			Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "management"},
			Methods: []Method{{
				Name: "GetDevice", FullName: "device.v1.DeviceApplication.GetDevice", Request: "device.v1.GetDeviceRequest", Response: "device.v1.DeviceDTO",
				HTTP:      []HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}},
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

func TestRenderC9ApplicationCodeGeneratesChildOperationCapability(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{
			{Name: "site.proto", Package: "site.v1", GoPackage: "example.com/biz/contracts/site/v1;sitev1", Domain: &DomainDeclaration{Name: "site", Version: "v1"}},
			{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}},
		},
		Messages: []Message{{Name: "ValidateRequest", FullName: "site.v1.ValidateRequest"}, {Name: "ValidateResponse", FullName: "site.v1.ValidateResponse"}, {Name: "TransferRequest", FullName: "device.v1.TransferRequest"}, {Name: "TransferResponse", FullName: "device.v1.TransferResponse"}},
		Services: []Service{
			{Name: "SiteApplication", FullName: "site.v1.SiteApplication", Domain: "site", Application: &ApplicationDeclaration{Name: "validation"}, Methods: []Method{{Name: "Validate", FullName: "site.v1.SiteApplication.Validate", Request: "site.v1.ValidateRequest", Response: "site.v1.ValidateResponse", Operation: &OperationDeclaration{ID: "site.validate", UseCase: "validate_site", Permissions: []string{"site.read"}, PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}}}}},
			{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/validation"}}, Methods: []Method{{Name: "Transfer", FullName: "device.v1.DeviceApplication.Transfer", Request: "device.v1.TransferRequest", Response: "device.v1.TransferResponse", Operation: &OperationDeclaration{ID: "device.transfer", UseCase: "transfer_device", Permissions: []string{"device.update", "site.read"}, PermissionMode: "all", RequiresOperations: []string{"site.validate"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}}},
		},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}
	capability := byPath["device/application/zz_yunka_transfer_capability_ports_gen.go"]
	if !strings.Contains(capability, "operation.ExecuteChildTyped") || !strings.Contains(capability, "sitepolicy.OperationPlanValidate()") || !strings.Contains(capability, "NewTransferToSiteValidationChildCapability") {
		t.Fatalf("C9 child capability is not executor-backed:\n%s", capability)
	}
	if strings.Contains(capability, "Resolve(") || strings.Contains(capability, "map[string]any") {
		t.Fatalf("service locator leaked into child capability:\n%s", capability)
	}
}

func TestRenderC9TransportsEstablishIdempotencyContext(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion,
		Files:    []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{{Name: "CreateRequest", FullName: "device.v1.CreateRequest"}, {Name: "CreateResponse", FullName: "device.v1.CreateResponse"}},
		Services: []Service{{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "management"}, Methods: []Method{{Name: "Create", FullName: "device.v1.DeviceApplication.Create", Request: "device.v1.CreateRequest", Response: "device.v1.CreateResponse", HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices", Body: "*"}}, Operation: &OperationDeclaration{ID: "device.create", UseCase: "create_device", PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}}}},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}
	rest := byPath["device/transport/rest/zz_yunka_management_operation_executor_gen.go"]
	rpc := byPath["device/transport/rpc/zz_yunka_management_operation_executor_gen.go"]
	if !strings.Contains(rest, `request.Header.Get("Idempotency-Key")`) || !strings.Contains(rest, "execution.WithIdempotencyKey") {
		t.Fatalf("REST idempotency context missing:\n%s", rest)
	}
	if !strings.Contains(rpc, `metadata.Get("idempotency-key")`) || !strings.Contains(rpc, "execution.WithIdempotencyKey") {
		t.Fatalf("gRPC idempotency context missing:\n%s", rpc)
	}
}

func TestRenderC9InternalOperationIsCapabilityOnlyNotTransport(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{
			{Name: "site.proto", Package: "site.v1", GoPackage: "example.com/biz/contracts/site/v1;sitev1", Domain: &DomainDeclaration{Name: "site"}},
			{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device"}},
		},
		Messages: []Message{
			{Name: "ValidateRequest", FullName: "site.v1.ValidateRequest"},
			{Name: "SiteDTO", FullName: "site.v1.SiteDTO"},
			{Name: "TransferRequest", FullName: "device.v1.TransferRequest"},
			{Name: "TransferResponse", FullName: "device.v1.TransferResponse"},
		},
		Services: []Service{
			{
				Name: "SiteApplication", FullName: "site.v1.SiteApplication", Domain: "site",
				Application: &ApplicationDeclaration{
					Name: "management",
					Operations: []OperationDeclaration{{
						ID: "site.validate", UseCase: "validate_site", Permissions: []string{"site.read"}, PermissionMode: "all",
						RequestType: "site.v1.ValidateRequest", ResponseType: "site.v1.SiteDTO", ApplicationMethod: "Validate",
						Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"},
					}},
				},
			},
			{
				Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device",
				Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/management"}},
				Methods: []Method{{
					Name: "Transfer", FullName: "device.v1.DeviceApplication.Transfer", Request: "device.v1.TransferRequest", Response: "device.v1.TransferResponse",
					Operation: &OperationDeclaration{ID: "device.transfer", UseCase: "transfer_device", Permissions: []string{"device.update", "site.read"}, PermissionMode: "all", RequiresOperations: []string{"site.validate"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}},
				}},
			},
		},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}
	sitePort := byPath["site/application/zz_yunka_management_application_port_gen.go"]
	if !strings.Contains(sitePort, "Validate(context.Context") {
		t.Fatalf("internal application method missing:\n%s", sitePort)
	}
	capability := byPath["device/application/zz_yunka_transfer_capability_ports_gen.go"]
	if !strings.Contains(capability, "Validate(context.Context") || !strings.Contains(capability, "operation.ExecuteChildTyped") || !strings.Contains(capability, "sitepolicy.OperationPlanValidate()") {
		t.Fatalf("internal child capability missing:\n%s", capability)
	}
	plan := byPath["site/policy/zz_yunka_management_operation_plan_gen.go"]
	if !strings.Contains(plan, `OperationID: "site.validate"`) || !strings.Contains(plan, `RPC: ""`) {
		t.Fatalf("internal operation plan missing or transport-bound:\n%s", plan)
	}
	for path, source := range byPath {
		if strings.Contains(path, "site/transport/") {
			t.Fatalf("internal-only application must not generate transport adapter: %s\n%s", path, source)
		}
	}
}
