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

func TestC9GeneratedUnifiedExecutionRuntime(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C9_RUNTIME") != "1" {
		t.Skip("C9 generated runtime fixture is enforced by make dsl-check")
	}
	protoc := strings.TrimSpace(os.Getenv("PROTOC"))
	protocGenGo := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO"))
	protocGenGRPC := strings.TrimSpace(os.Getenv("PROTOC_GEN_GO_GRPC"))
	for label, path := range map[string]string{"PROTOC": protoc, "PROTOC_GEN_GO": protocGenGo, "PROTOC_GEN_GO_GRPC": protocGenGRPC} {
		if path == "" {
			t.Fatalf("%s is required by the C9 runtime gate", label)
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
	writeC84FixtureFile(t, filepath.Join(contractsRoot, "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c9fixture/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };
message GetDeviceRequest { string id = 1; }
message DeviceDTO { string id = 1; string serial = 2; }
service DeviceApplication {
  option (yunka.dsl.v1.application) = { name: "management" };
  rpc GetDevice(GetDeviceRequest) returns (DeviceDTO) {
    option (yunka.dsl.v1.operation) = {
      id: "device.get"
      use_case: "get_device"
      permissions: "device.read"
      permission_mode: PERMISSION_ALL
      tenant_required: true
      authentication: AUTHENTICATION_JWT
    };
  }
}
`)
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/c9fixture

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	yunka.io/framework v0.0.0
	yunka.io/gateway v0.0.0
	yunka.io/pkg v0.0.0
)

replace yunka.io/framework => %s
replace yunka.io/gateway => %s
replace yunka.io/pkg => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))
	args := []string{"-I", contractsRoot, "-I", filepath.Join(repositoryRoot, "contracts", "proto")}
	if include := standardProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args,
		"--plugin=protoc-gen-go="+protocGenGo,
		"--plugin=protoc-gen-go-grpc="+protocGenGRPC,
		"--go_out="+root,
		"--go_opt=module=example.com/c9fixture",
		"--go-grpc_out="+root,
		"--go-grpc_opt=module=example.com/c9fixture,require_unimplemented_servers=false",
		"device/v1/device.proto",
	)
	command := exec.Command(protoc, args...)
	command.Dir = contractsRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("protoc fixture generation failed: %v\n%s", err, output)
	}
	compiled, err := Compile(context.Background(), CompileOptions{Dir: contractsRoot, ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")}, Files: []string{"device/v1/device.proto"}, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	compiled.Manifest.Services[0].Methods[0].HTTP = []HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}}
	compiled.Manifest.Normalize()
	if diagnostics := Lint(compiled.Manifest); HasErrors(diagnostics) {
		t.Fatalf("fixture lint failed: %#v", diagnostics)
	}
	files, err := RenderC9ApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/c9fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}
	writeC84FixtureFile(t, filepath.Join(root, "runtime_test.go"), c9RuntimeFixtureTest)
	goTest := exec.Command("go", "test", "-mod=mod", "./...")
	goTest.Dir = root
	goTest.Env = append(os.Environ(), "GOWORK=off")
	if output, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("generated C9 runtime fixture failed: %v\n%s", err, output)
	}
}

const c9RuntimeFixtureTest = `package c9fixture

import (
    "context"
    "net"
    "net/http"
    "net/http/httptest"
    "sync"
    "testing"

    grpcgo "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/test/bufconn"
    devicev1 "example.com/c9fixture/contracts/device/v1"
    rest "example.com/c9fixture/internal/device/transport/rest"
    rpc "example.com/c9fixture/internal/device/transport/rpc"
    "yunka.io/framework/core/identity"
    "yunka.io/framework/core/runtimecontext"
    "yunka.io/framework/operation"
    "yunka.io/gateway/authz"
)

type checker struct {
    mu sync.Mutex
    allow bool
    calls int
}

func (checker *checker) HasPermissions(_ context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
    checker.mu.Lock()
    defer checker.mu.Unlock()
    checker.calls++
    return checker.allow && tenant == "tenant-a" && len(roles) == 1 && roles[0] == "operator" && len(permissions) == 1 && permissions[0] == "device.read" && mode == authz.PermissionAll, nil
}

func (checker *checker) setAllow(value bool) { checker.mu.Lock(); checker.allow = value; checker.mu.Unlock() }
func (checker *checker) count() int { checker.mu.Lock(); defer checker.mu.Unlock(); return checker.calls }

type observer struct {
    mu sync.Mutex
    events []operation.Event
}
func (observer *observer) Observe(_ context.Context, event operation.Event) { observer.mu.Lock(); observer.events = append(observer.events, event); observer.mu.Unlock() }
func (observer *observer) securityStarts() int { observer.mu.Lock(); defer observer.mu.Unlock(); count := 0; for _, event := range observer.events { if event.OperationID == "device.get" && event.Phase == operation.PhaseSecurity && event.Outcome == operation.OutcomeStarted { count++ } }; return count }

type service struct {
    mu sync.Mutex
    calls int
}
func (service *service) GetDevice(ctx context.Context, request *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
    authorized, err := authz.RequireAuthorizedOperation(ctx, "device.get")
    if err != nil { return nil, err }
    metadata, ok := runtimecontext.MetadataFrom(ctx)
    if !ok || metadata.Operation != "device.get" { return nil, status.Error(codes.Internal, "missing stable operation metadata") }
    if !authorized.Decision.Allowed { return nil, status.Error(codes.Internal, "missing authorization") }
    service.mu.Lock(); service.calls++; service.mu.Unlock()
    return &devicev1.DeviceDTO{Id: request.GetId(), Serial: "shared-executor"}, nil
}
func (service *service) count() int { service.mu.Lock(); defer service.mu.Unlock(); return service.calls }

func injectPrincipal(principal identity.Principal) grpcgo.UnaryServerInterceptor {
    return func(ctx context.Context, request any, _ *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
        return handler(identity.WithPrincipal(ctx, principal), request)
    }
}

func TestRESTAndGRPCShareOneExecutorPath(t *testing.T) {
    principal := identity.Principal{Subject: "user-1", TenantID: "tenant-a", UserID: "user-1", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT, Authenticated: true}
    permissionChecker := &checker{allow: true}
    authorizer, err := authz.NewRBACAuthorizer(permissionChecker)
    if err != nil { t.Fatal(err) }
    security, err := authz.NewExecutionSecurity(authorizer, nil)
    if err != nil { t.Fatal(err) }
    executionObserver := &observer{}
    executor := operation.NewExecutor(security, executionObserver)
    application := &service{}

    listener := bufconn.Listen(1024 * 1024)
    server := grpcgo.NewServer(grpcgo.ChainUnaryInterceptor(injectPrincipal(principal)))
    if err := rpc.RegisterOperationExecutor(server, application, executor); err != nil { t.Fatal(err) }
    go func() { _ = server.Serve(listener) }()
    defer server.Stop()

    dialer := func(context.Context, string) (net.Conn, error) { return listener.Dial() }
    conn, err := grpcgo.DialContext(context.Background(), "bufnet", grpcgo.WithContextDialer(dialer), grpcgo.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil { t.Fatal(err) }
    defer conn.Close()
    grpcResponse, err := devicev1.NewDeviceApplicationClient(conn).GetDevice(context.Background(), &devicev1.GetDeviceRequest{Id: "grpc-id"})
    if err != nil { t.Fatal(err) }
    if grpcResponse.GetSerial() != "shared-executor" { t.Fatalf("gRPC response=%v", grpcResponse) }

    mux := http.NewServeMux()
    if err := rest.RegisterOperationExecutor(mux, application, executor); err != nil { t.Fatal(err) }
    request := httptest.NewRequest(http.MethodGet, "/v1/devices/rest-id", nil)
    request = request.WithContext(identity.WithPrincipal(request.Context(), principal))
    recorder := httptest.NewRecorder()
    mux.ServeHTTP(recorder, request)
    if recorder.Code != http.StatusOK { t.Fatalf("REST status=%d body=%s", recorder.Code, recorder.Body.String()) }

    if application.count() != 2 || permissionChecker.count() != 2 || executionObserver.securityStarts() != 2 {
        t.Fatalf("application=%d authorization=%d securityStarts=%d", application.count(), permissionChecker.count(), executionObserver.securityStarts())
    }

    permissionChecker.setAllow(false)
    _, err = devicev1.NewDeviceApplicationClient(conn).GetDevice(context.Background(), &devicev1.GetDeviceRequest{Id: "denied"})
    if status.Code(err) != codes.PermissionDenied { t.Fatalf("gRPC denied code=%s err=%v", status.Code(err), err) }
    deniedRequest := httptest.NewRequest(http.MethodGet, "/v1/devices/denied", nil)
    deniedRequest = deniedRequest.WithContext(identity.WithPrincipal(deniedRequest.Context(), principal))
    deniedRecorder := httptest.NewRecorder()
    mux.ServeHTTP(deniedRecorder, deniedRequest)
    if deniedRecorder.Code != http.StatusForbidden { t.Fatalf("REST denied status=%d body=%s", deniedRecorder.Code, deniedRecorder.Body.String()) }
    if application.count() != 2 { t.Fatalf("denied request reached application calls=%d", application.count()) }
}
`
