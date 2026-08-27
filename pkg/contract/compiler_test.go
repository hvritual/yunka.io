package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testProtoc(t *testing.T) string {
	t.Helper()
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		if path, err := exec.LookPath("protoc"); err == nil {
			protoc = path
		}
	}
	if protoc == "" {
		t.Skip("protoc is not available")
	}
	return protoc
}

func TestCompileDirectoryAndDirectiveHTTP(t *testing.T) {
	protoc := testProtoc(t)
	dir := t.TempDir()
	proto := `syntax = "proto3";
package demo.v1;
option go_package = "example/demo/v1;demov1";
message EchoRequest { string id = 1; repeated string tags = 2; }
message EchoResponse { string value = 1; }
service EchoService {
  // @yunka.http GET /v1/echo/{id}
  // @yunka.auth required
  rpc Echo(EchoRequest) returns (EchoResponse);
}
`
	if err := os.WriteFile(filepath.Join(dir, "echo.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(context.Background(), CompileOptions{Dir: dir, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.SchemaVersion != ManifestVersion {
		t.Fatalf("schemaVersion=%d want=%d", result.Manifest.SchemaVersion, ManifestVersion)
	}
	if len(result.Manifest.Services) != 1 || len(result.Manifest.Services[0].Methods) != 1 {
		t.Fatalf("unexpected services: %#v", result.Manifest.Services)
	}
	method := result.Manifest.Services[0].Methods[0]
	if len(method.HTTP) != 1 || method.HTTP[0].Method != "GET" || method.HTTP[0].Path != "/v1/echo/{id}" {
		t.Fatalf("unexpected http binding: %#v", method.HTTP)
	}
	if method.Directives["auth"] != "required" {
		t.Fatalf("unexpected directives: %#v", method.Directives)
	}
	if result.DescriptorSHA == "" {
		t.Fatal("descriptor digest is empty")
	}
	if diagnostics := Lint(result.Manifest); HasErrors(diagnostics) {
		t.Fatalf("lint failed: %#v", diagnostics)
	}
}

func TestCompileTypedPBDSL(t *testing.T) {
	protoc := testProtoc(t)
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	proto := `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };
message Filter { string site_id = 1; }
message GetMachineRequest {
  string id = 1;
  Filter filter = 2;
}
message MachineDTO {
  option (yunka.dsl.v1.dto) = { kind: DTO_OUTPUT };
  string id = 1;
  string serial = 2;
}
service DeviceApplication {
  option (yunka.dsl.v1.application) = { name: "device_management" };
  rpc GetMachine(GetMachineRequest) returns (MachineDTO) {
    option (yunka.dsl.v1.operation) = {
      id: "device.machine.get"
      use_case: "get_machine"
      permissions: "device.machine.read"
      permission_mode: PERMISSION_ALL
      tenant_required: true
      authentication: AUTHENTICATION_JWT
    };
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "device.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(context.Background(), CompileOptions{
		Dir:        dir,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
		Files:      []string{"device.proto"},
		Protoc:     protoc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Files) != 1 || result.Manifest.Files[0].Domain == nil || result.Manifest.Files[0].Domain.Name != "device" {
		t.Fatalf("typed domain missing: %#v", result.Manifest.Files)
	}
	byMessage := map[string]Message{}
	for _, message := range result.Manifest.Messages {
		byMessage[message.FullName] = message
		if strings.HasPrefix(message.FullName, "yunka.dsl") {
			t.Fatalf("DSL support message leaked into business manifest: %s", message.FullName)
		}
	}
	if got := byMessage["device.v1.GetMachineRequest"].DTO; got == nil || got.Kind != "input" {
		t.Fatalf("request DTO should be inferred as input: %#v", got)
	}
	if got := byMessage["device.v1.Filter"].DTO; got == nil || got.Kind != "input" {
		t.Fatalf("reachable nested DTO should be inferred as input: %#v", got)
	}
	if got := byMessage["device.v1.MachineDTO"].DTO; got == nil || got.Kind != "output" {
		t.Fatalf("explicit output DTO missing: %#v", got)
	}
	service := result.Manifest.Services[0]
	if service.Domain != "device" || service.Application == nil || service.Application.Name != "device_management" {
		t.Fatalf("typed application missing: %#v", service)
	}
	method := service.Methods[0]
	if method.Operation == nil || method.Operation.ID != "device.machine.get" || method.Operation.UseCase != "get_machine" {
		t.Fatalf("typed operation missing: %#v", method)
	}
	if method.Authorization == nil || method.Authorization.OperationID != "device.machine.get" || len(method.Authorization.Permissions) != 1 || method.Authorization.Permissions[0] != "device.machine.read" || method.Authorization.PermissionMode != "all" || !method.Authorization.TenantRequired {
		t.Fatalf("typed authorization normalization failed: %#v", method.Authorization)
	}
	if diagnostics := Lint(result.Manifest); HasErrors(diagnostics) {
		t.Fatalf("typed DSL lint failed: %#v", diagnostics)
	}
}

func TestCompileTypedPBDSLRejectsShadowPolicyConflict(t *testing.T) {
	protoc := testProtoc(t)
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	proto := `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" };
message Request { string id = 1; }
message Response { string id = 1; }
service DeviceApplication {
  option (yunka.dsl.v1.application) = { name: "device_management" };
  // @yunka.operation device.machine.get
  // @yunka.permission device.machine.admin
  // @yunka.permission_mode all
  // @yunka.tenant_required true
  // @yunka.authentication jwt
  rpc Get(Request) returns (Response) {
    option (yunka.dsl.v1.operation) = {
      id: "device.machine.get"
      use_case: "get_machine"
      permissions: "device.machine.read"
      tenant_required: true
      authentication: AUTHENTICATION_JWT
    };
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "device.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Compile(context.Background(), CompileOptions{
		Dir:        dir,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
		Files:      []string{"device.proto"},
		Protoc:     protoc,
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts with legacy @yunka authorization directives") {
		t.Fatalf("expected shadow policy conflict, got %v", err)
	}
}

func TestLintRejectsShadowHTTPConflict(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Messages: []Message{{Name: "Request", FullName: "demo.Request"}, {Name: "Response", FullName: "demo.Response"}},
		Services: []Service{{
			Name: "Demo", FullName: "demo.Demo",
			Methods: []Method{{
				Name: "Get", FullName: "demo.Demo.Get", Request: "demo.Request", Response: "demo.Response",
				HTTP:       []HTTPBinding{{Method: "GET", Path: "/v1/demo"}},
				Directives: map[string]string{"http": "POST /v1/demo"},
			}},
		}},
	}
	diagnostics := Lint(manifest)
	if !HasErrors(diagnostics) {
		t.Fatalf("expected HTTP shadow conflict: %#v", diagnostics)
	}
	found := false
	for _, diagnostic := range diagnostics {
		found = found || strings.Contains(diagnostic.Message, "conflicts with legacy @yunka.http")
	}
	if !found {
		t.Fatalf("missing HTTP conflict diagnostic: %#v", diagnostics)
	}
}
