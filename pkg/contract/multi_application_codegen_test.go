package contract

import (
	"strings"
	"testing"
)

func TestRenderApplicationCodeSupportsMultipleApplicationsPerDomain(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{
			{Name: "QueryRequest", FullName: "device.v1.QueryRequest", DTO: &DTODeclaration{Kind: "input"}},
			{Name: "CommandRequest", FullName: "device.v1.CommandRequest", DTO: &DTODeclaration{Kind: "input"}},
			{Name: "MachineDTO", FullName: "device.v1.MachineDTO", DTO: &DTODeclaration{Kind: "output"}},
		},
		Services: []Service{
			{Name: "DeviceQueryService", FullName: "device.v1.DeviceQueryService", Domain: "device", Application: &ApplicationDeclaration{Name: "device_query"}, Methods: []Method{{Name: "Get", FullName: "device.v1.DeviceQueryService.Get", Request: "device.v1.QueryRequest", Response: "device.v1.MachineDTO", HTTP: []HTTPBinding{{Method: "GET", Path: "/v1/devices/query"}}, Operation: &OperationDeclaration{ID: "device.query.get", UseCase: "get_device", Permissions: []string{"device.read"}, PermissionMode: "all"}}}},
			{Name: "DeviceCommandService", FullName: "device.v1.DeviceCommandService", Domain: "device", Application: &ApplicationDeclaration{Name: "device_command"}, Methods: []Method{{Name: "Get", FullName: "device.v1.DeviceCommandService.Get", Request: "device.v1.CommandRequest", Response: "device.v1.MachineDTO", HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices/command", Body: "*"}}, Operation: &OperationDeclaration{ID: "device.command.get", UseCase: "command_device", Permissions: []string{"device.write"}, PermissionMode: "all"}}}},
		},
	}
	files, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 9 {
		t.Fatalf("generated files=%d want=9", len(files))
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}

	queryPort := byPath["device/application/zz_yunka_device_query_application_port_gen.go"]
	commandPort := byPath["device/application/zz_yunka_device_command_application_port_gen.go"]
	if !strings.Contains(queryPort, "type DeviceQueryApplication interface") {
		t.Fatalf("query port:\n%s", queryPort)
	}
	if !strings.Contains(commandPort, "type DeviceCommandApplication interface") {
		t.Fatalf("command port:\n%s", commandPort)
	}

	queryPolicy := byPath["device/policy/zz_yunka_device_query_operation_policy_gen.go"]
	commandPolicy := byPath["device/policy/zz_yunka_device_command_operation_policy_gen.go"]
	if !strings.Contains(queryPolicy, "OperationDeviceQueryGet") || !strings.Contains(queryPolicy, "func DeviceQueryPermissions()") || !strings.Contains(queryPolicy, "func DeviceQueryResolver()") {
		t.Fatalf("query policy:\n%s", queryPolicy)
	}
	if !strings.Contains(commandPolicy, "OperationDeviceCommandGet") || !strings.Contains(commandPolicy, "func DeviceCommandPermissions()") || !strings.Contains(commandPolicy, "func DeviceCommandResolver()") {
		t.Fatalf("command policy:\n%s", commandPolicy)
	}
	for path, source := range map[string]string{"query": queryPolicy, "command": commandPolicy} {
		if strings.Contains(source, "func Permissions()") || strings.Contains(source, "func Resolver()") {
			t.Fatalf("service policy %s leaks canonical domain symbols:\n%s", path, source)
		}
	}
	domainPolicy := byPath["device/policy/zz_yunka_domain_operation_policy_gen.go"]
	for _, want := range []string{"func Permissions()", "func Resolver()", `"device.read"`, `"device.write"`, "deviceQueryPolicies()", "deviceCommandPolicies()"} {
		if !strings.Contains(domainPolicy, want) {
			t.Fatalf("domain policy missing %q:\n%s", want, domainPolicy)
		}
	}

	queryRPC := byPath["device/transport/rpc/zz_yunka_device_query_rpc_adapter_gen.go"]
	commandRPC := byPath["device/transport/rpc/zz_yunka_device_command_rpc_adapter_gen.go"]
	if !strings.Contains(queryRPC, "func RegisterDeviceQuery(") || !strings.Contains(queryRPC, "type DeviceQueryServer struct") {
		t.Fatalf("query rpc:\n%s", queryRPC)
	}
	if !strings.Contains(commandRPC, "func RegisterDeviceCommand(") || !strings.Contains(commandRPC, "type DeviceCommandServer struct") {
		t.Fatalf("command rpc:\n%s", commandRPC)
	}

	queryREST := byPath["device/transport/rest/zz_yunka_device_query_rest_adapter_gen.go"]
	commandREST := byPath["device/transport/rest/zz_yunka_device_command_rest_adapter_gen.go"]
	if !strings.Contains(queryREST, "func RegisterDeviceQuery(") || !strings.Contains(queryREST, "type DeviceQueryHandler struct") || !strings.Contains(queryREST, "writeDeviceQuerySecurityError") {
		t.Fatalf("query rest:\n%s", queryREST)
	}
	if !strings.Contains(commandREST, "func RegisterDeviceCommand(") || !strings.Contains(commandREST, "type DeviceCommandHandler struct") || !strings.Contains(commandREST, "writeDeviceCommandSecurityError") {
		t.Fatalf("command rest:\n%s", commandREST)
	}
}

func TestRenderApplicationCodeRejectsMultiApplicationGeneratedIdentityCollision(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion, Files: []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device"}}}, Services: []Service{
		{Name: "First", FullName: "device.v1.First", Domain: "device", Application: &ApplicationDeclaration{Name: "device-query"}},
		{Name: "Second", FullName: "device.v1.Second", Domain: "device", Application: &ApplicationDeclaration{Name: "device_query"}},
	}}
	_, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err == nil || !strings.Contains(err.Error(), "same generated identity") {
		t.Fatalf("expected generated identity collision, got %v", err)
	}
}
