package contract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestC86GeneratedMultiApplicationRuntime(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C86_RUNTIME") != "1" {
		t.Skip("C8.6 multi-application runtime fixture is enforced by make dsl-check")
	}
	protoc := strings.TrimSpace(os.Getenv("PROTOC"))
	protocGenGo := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO"))
	protocGenGRPC := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO_GRPC"))
	for label, path := range map[string]string{"PROTOC": protoc, "PROTOC_GEN_GO": protocGenGo, "PROTOC_GEN_GO_GRPC": protocGenGRPC} {
		if path == "" {
			t.Fatalf("%s is required by the C8.6 runtime gate", label)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s=%s: %v", label, path, err)
		}
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	contractsRoot := filepath.Join(root, "contracts")
	protoPath := filepath.Join(contractsRoot, "device", "v1", "device.proto")
	writeC84FixtureFile(t, protoPath, `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c86fixture/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };
message QueryRequest { string id = 1; }
message CommandRequest { string id = 1; }
message MachineDTO { string id = 1; }
service DeviceQueryService {
  option (yunka.dsl.v1.application) = { name: "device_query" };
  rpc Get(QueryRequest) returns (MachineDTO) {
    option (yunka.dsl.v1.operation) = { id: "device.query.get" use_case: "get_device" permissions: "device.read" permission_mode: PERMISSION_ALL tenant_required: true authentication: AUTHENTICATION_JWT };
  }
}
service DeviceCommandService {
  option (yunka.dsl.v1.application) = { name: "device_command" };
  rpc Get(CommandRequest) returns (MachineDTO) {
    option (yunka.dsl.v1.operation) = { id: "device.command.get" use_case: "command_device" permissions: "device.write" permission_mode: PERMISSION_ALL tenant_required: true authentication: AUTHENTICATION_JWT };
  }
}
`)
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/c86fixture

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	github.com/hvritual/yunka.io/framework v0.0.0
	github.com/hvritual/yunka.io/gateway v0.0.0
	github.com/hvritual/yunka.io/pkg v0.0.0
)

replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/gateway => %s
replace github.com/hvritual/yunka.io/pkg => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))
	args := []string{"-I", contractsRoot, "-I", filepath.Join(repositoryRoot, "contracts", "proto")}
	if include := standardProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args, "--plugin=protoc-gen-go="+protocGenGo, "--plugin=protoc-gen-go-grpc="+protocGenGRPC, "--go_out="+root, "--go_opt=module=example.com/c86fixture", "--go-grpc_out="+root, "--go-grpc_opt=module=example.com/c86fixture,require_unimplemented_servers=false", "device/v1/device.proto")
	command := exec.Command(protoc, args...)
	command.Dir = contractsRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("protoc fixture generation failed: %v\n%s", err, output)
	}
	compiled, err := Compile(context.Background(), CompileOptions{Dir: contractsRoot, ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")}, Files: []string{"device/v1/device.proto"}, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Manifest.Services) != 2 {
		t.Fatalf("services=%d want=2", len(compiled.Manifest.Services))
	}
	for i := range compiled.Manifest.Services {
		switch compiled.Manifest.Services[i].Name {
		case "DeviceQueryService":
			compiled.Manifest.Services[i].Methods[0].HTTP = []HTTPBinding{{Method: "GET", Path: "/v1/devices/query/{id}"}}
		case "DeviceCommandService":
			compiled.Manifest.Services[i].Methods[0].HTTP = []HTTPBinding{{Method: "POST", Path: "/v1/devices/command", Body: "*"}}
		}
	}
	compiled.Manifest.Normalize()
	if diagnostics := Lint(compiled.Manifest); HasErrors(diagnostics) {
		t.Fatalf("fixture lint failed: %#v", diagnostics)
	}
	files, err := RenderApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/c86fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}
	writeC84FixtureFile(t, filepath.Join(root, "runtime_test.go"), c86RuntimeFixtureTest)
	goTest := exec.Command("go", "test", "-mod=mod", "./...")
	goTest.Dir = root
	goTest.Env = append(os.Environ(), "GOWORK=off")
	if output, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("generated C8.6 runtime fixture failed: %v\n%s", err, output)
	}
}

const c86RuntimeFixtureTest = `package c86fixture

import (
    "context"
    "net/http"
    "testing"

    devicev1 "example.com/c86fixture/contracts/device/v1"
    application "example.com/c86fixture/internal/device/application"
    policy "example.com/c86fixture/internal/device/policy"
    rest "example.com/c86fixture/internal/device/transport/rest"
    rpc "example.com/c86fixture/internal/device/transport/rpc"
    grpcgo "google.golang.org/grpc"
    "github.com/hvritual/yunka.io/gateway/authz"
)

type checker struct{}
func (checker) HasPermissions(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) { return true, nil }
type queryService struct{}
func (*queryService) Get(context.Context, *devicev1.QueryRequest) (*devicev1.MachineDTO, error) { return &devicev1.MachineDTO{Id: "query"}, nil }
type commandService struct{}
func (*commandService) Get(context.Context, *devicev1.CommandRequest) (*devicev1.MachineDTO, error) { return &devicev1.MachineDTO{Id: "command"}, nil }
var _ application.DeviceQueryApplication = (*queryService)(nil)
var _ application.DeviceCommandApplication = (*commandService)(nil)

func TestMultiApplicationDomainWiring(t *testing.T) {
    authorizer, err := authz.NewRBACAuthorizer(checker{}); if err != nil { t.Fatal(err) }
    runtime, err := authz.NewOperationRuntime(policy.Resolver(), authorizer, nil); if err != nil { t.Fatal(err) }
    if got := len(policy.Permissions()); got != 2 { t.Fatalf("permissions=%d want=2", got) }
    resolver := policy.Resolver()
    for _, method := range []string{"/device.v1.DeviceQueryService/Get", "/device.v1.DeviceCommandService/Get"} {
        if _, ok := resolver.ResolvePolicy(context.Background(), method); !ok { t.Fatalf("missing policy %s", method) }
    }
    grpcServer := grpcgo.NewServer()
    rpc.RegisterDeviceQuery(grpcServer, &queryService{})
    rpc.RegisterDeviceCommand(grpcServer, &commandService{})
    mux := http.NewServeMux()
    if err := rest.RegisterDeviceQuery(mux, &queryService{}, runtime); err != nil { t.Fatal(err) }
    if err := rest.RegisterDeviceCommand(mux, &commandService{}, runtime); err != nil { t.Fatal(err) }
}
`
