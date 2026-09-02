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

func TestC84GeneratedApplicationRuntime(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C84_RUNTIME") != "1" {
		t.Skip("C8.4 generated runtime fixture is enforced by make dsl-check")
	}
	protoc := strings.TrimSpace(os.Getenv("PROTOC"))
	protocGenGo := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO"))
	protocGenGRPC := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO_GRPC"))
	for label, path := range map[string]string{
		"PROTOC":             protoc,
		"PROTOC_GEN_GO":      protocGenGo,
		"PROTOC_GEN_GO_GRPC": protocGenGRPC,
	} {
		if path == "" {
			t.Fatalf("%s is required by the C8.4 runtime gate", label)
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
option go_package = "example.com/c84fixture/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };
message GetMachineRequest { string id = 1; }
message MachineDTO { string id = 1; string serial = 2; }
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
`)
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/c84fixture

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

	args := []string{
		"-I", contractsRoot,
		"-I", filepath.Join(repositoryRoot, "contracts", "proto"),
	}
	if include := standardProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args,
		"--plugin=protoc-gen-go="+protocGenGo,
		"--plugin=protoc-gen-go-grpc="+protocGenGRPC,
		"--go_out="+root,
		"--go_opt=module=example.com/c84fixture",
		"--go-grpc_out="+root,
		"--go-grpc_opt=module=example.com/c84fixture,require_unimplemented_servers=false",
		"device/v1/device.proto",
	)
	command := exec.Command(protoc, args...)
	command.Dir = contractsRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("protoc fixture generation failed: %v\n%s", err, output)
	}

	compiled, err := Compile(context.Background(), CompileOptions{
		Dir:        contractsRoot,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
		Files:      []string{"device/v1/device.proto"},
		Protoc:     protoc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Manifest.Services) != 1 || len(compiled.Manifest.Services[0].Methods) != 1 {
		t.Fatalf("unexpected typed fixture manifest: %#v", compiled.Manifest.Services)
	}
	compiled.Manifest.Services[0].Methods[0].HTTP = []HTTPBinding{{Method: "GET", Path: "/v1/machines/{id}"}}
	compiled.Manifest.Normalize()
	if diagnostics := Lint(compiled.Manifest); HasErrors(diagnostics) {
		t.Fatalf("fixture lint failed: %#v", diagnostics)
	}
	files, err := RenderApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/c84fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}

	writeC84FixtureFile(t, filepath.Join(root, "runtime_test.go"), c84RuntimeFixtureTest)
	goTest := exec.Command("go", "test", "-mod=mod", "./...")
	goTest.Dir = root
	goTest.Env = append(os.Environ(), "GOWORK=off")
	if output, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("generated C8.4 runtime fixture failed: %v\n%s", err, output)
	}
}

func writeC84FixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const c84RuntimeFixtureTest = `package c84fixture

import (
    "context"
    "net"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"

    grpcgo "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/test/bufconn"
    devicev1 "example.com/c84fixture/contracts/device/v1"
    rest "example.com/c84fixture/internal/device/transport/rest"
    rpc "example.com/c84fixture/internal/device/transport/rpc"
    policy "example.com/c84fixture/internal/device/policy"
    "github.com/hvritual/yunka.io/framework/core/identity"
    "github.com/hvritual/yunka.io/gateway/authz"
    authzgrpc "github.com/hvritual/yunka.io/gateway/rpc/transport/grpc"
)

type checkCall struct {
    tenant string
    roles []string
    permissions []authz.PermissionKey
    mode authz.PermissionMode
}

type checker struct {
    mu sync.Mutex
    allow bool
    calls []checkCall
}

func (checker *checker) HasPermissions(_ context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
    checker.mu.Lock()
    defer checker.mu.Unlock()
    checker.calls = append(checker.calls, checkCall{tenant: tenant, roles: append([]string(nil), roles...), permissions: append([]authz.PermissionKey(nil), permissions...), mode: mode})
    return checker.allow, nil
}

func (checker *checker) setAllow(value bool) {
    checker.mu.Lock()
    checker.allow = value
    checker.mu.Unlock()
}

func (checker *checker) snapshot() []checkCall {
    checker.mu.Lock()
    defer checker.mu.Unlock()
    return append([]checkCall(nil), checker.calls...)
}

type service struct {
    mu sync.Mutex
    principals []identity.Principal
}

func (service *service) GetMachine(ctx context.Context, request *devicev1.GetMachineRequest) (*devicev1.MachineDTO, error) {
    principal, _ := identity.FromContext(ctx)
    service.mu.Lock()
    service.principals = append(service.principals, principal)
    service.mu.Unlock()
    return &devicev1.MachineDTO{Id: request.GetId(), Serial: "shared-application"}, nil
}

func (service *service) count() int {
    service.mu.Lock()
    defer service.mu.Unlock()
    return len(service.principals)
}

func injectPrincipal(principal identity.Principal) grpcgo.UnaryServerInterceptor {
    return func(ctx context.Context, request any, _ *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
        return handler(identity.WithPrincipal(ctx, principal), request)
    }
}

func TestRESTAndGRPCSharePolicyAuthorizerAndApplication(t *testing.T) {
    principal := identity.Principal{Subject: "user-1", TenantID: "tenant-a", UserID: "user-1", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT, Authenticated: true}
    permissionChecker := &checker{allow: true}
    authorizer, err := authz.NewRBACAuthorizer(permissionChecker)
    if err != nil { t.Fatal(err) }
    application := &service{}

    securityRuntime, err := authz.NewOperationRuntime(policy.Resolver(), authorizer, nil)
    if err != nil { t.Fatal(err) }
    authInterceptor, err := authzgrpc.SecuredUnaryServerInterceptor(securityRuntime)
    if err != nil { t.Fatal(err) }
    listener := bufconn.Listen(1024 * 1024)
    server := grpcgo.NewServer(grpcgo.ChainUnaryInterceptor(injectPrincipal(principal), authInterceptor))
    rpc.Register(server, application)
    go func() { _ = server.Serve(listener) }()
    defer server.Stop()

    dialer := func(context.Context, string) (net.Conn, error) { return listener.Dial() }
    conn, err := grpcgo.DialContext(context.Background(), "bufnet", grpcgo.WithContextDialer(dialer), grpcgo.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil { t.Fatal(err) }
    defer conn.Close()
    grpcResponse, err := devicev1.NewDeviceApplicationClient(conn).GetMachine(context.Background(), &devicev1.GetMachineRequest{Id: "grpc-id"})
    if err != nil { t.Fatal(err) }
    if grpcResponse.GetId() != "grpc-id" || grpcResponse.GetSerial() != "shared-application" { t.Fatalf("unexpected gRPC response: %#v", grpcResponse) }

    mux := http.NewServeMux()
    if err := rest.Register(mux, application, securityRuntime); err != nil { t.Fatal(err) }
    restRequest := httptest.NewRequest(http.MethodGet, "/v1/machines/rest-id", nil)
    restRequest = restRequest.WithContext(identity.WithPrincipal(restRequest.Context(), principal))
    recorder := httptest.NewRecorder()
    mux.ServeHTTP(recorder, restRequest)
    if recorder.Code != http.StatusOK { t.Fatalf("REST status=%d body=%s", recorder.Code, recorder.Body.String()) }
    if body := recorder.Body.String(); !strings.Contains(body, "rest-id") || !strings.Contains(body, "shared-application") { t.Fatalf("unexpected REST response: %s", body) }

    if application.count() != 2 { t.Fatalf("shared application calls=%d want=2", application.count()) }
    calls := permissionChecker.snapshot()
    if len(calls) != 2 { t.Fatalf("permission calls=%d want=2", len(calls)) }
    for _, call := range calls {
        if call.tenant != "tenant-a" || len(call.roles) != 1 || call.roles[0] != "operator" || len(call.permissions) != 1 || call.permissions[0] != authz.PermissionKey("device.machine.read") || call.mode != authz.PermissionAll {
            t.Fatalf("transport policy drift: %#v", call)
        }
    }

    permissionChecker.setAllow(false)
    before := application.count()
    _, err = devicev1.NewDeviceApplicationClient(conn).GetMachine(context.Background(), &devicev1.GetMachineRequest{Id: "denied"})
    if status.Code(err) != codes.PermissionDenied { t.Fatalf("gRPC deny=%v code=%s", err, status.Code(err)) }
    deniedRequest := httptest.NewRequest(http.MethodGet, "/v1/machines/denied", nil)
    deniedRequest = deniedRequest.WithContext(identity.WithPrincipal(deniedRequest.Context(), principal))
    deniedRecorder := httptest.NewRecorder()
    mux.ServeHTTP(deniedRecorder, deniedRequest)
    if deniedRecorder.Code != http.StatusForbidden { t.Fatalf("REST deny status=%d body=%s", deniedRecorder.Code, deniedRecorder.Body.String()) }
    if application.count() != before { t.Fatalf("denied transport reached application: before=%d after=%d", before, application.count()) }
}
`
