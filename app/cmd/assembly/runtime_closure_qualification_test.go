package assembly

import (
	"os"
	"path/filepath"
	"testing"
)

func TestC103QualificationFullAssembledRuntimeClosure(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C103_QUALIFICATION") != "1" {
		t.Skip("C10.3 assembled runtime fixture is enforced by the repository qualification gate")
	}

	protoc := requireQualificationTool(t, "PROTOC")
	protocGenGo := requireQualificationTool(t, "PROTOC_GEN_GO")
	protocGenGRPC := requireQualificationTool(t, "PROTOC_GEN_GO_GRPC")
	repositoryRoot := qualificationRepositoryRoot(t)
	appRoot := filepath.Join(repositoryRoot, "app")
	fixtureRoot := t.TempDir()
	protoRoot := filepath.Join(fixtureRoot, "contracts", "proto")
	moduleRoot := filepath.Join(fixtureRoot, "modules")
	contractOut := filepath.Join(fixtureRoot, "contracts", "generated")
	codeOut := filepath.Join(fixtureRoot, "internal")
	codeImport := c102QualificationModule + "/internal"

	// Reuse the already-qualified C10.2 consumer module layout, then replace its
	// contract and consumer test with the stronger C10.3 runtime pressure. This
	// keeps one compiler/generator path while leaving the C10.2 fixture itself
	// untouched.
	writeQualificationFixture(t, fixtureRoot, repositoryRoot)
	writeC103ContractFixture(t, fixtureRoot)
	writeQualificationFile(t, filepath.Join(fixtureRoot, "qualification_test.go"), c103RuntimeConsumerTestSource())
	generateC103QualificationModules(t, appRoot, moduleRoot)
	generateQualificationPB(t, protoc, protocGenGo, protocGenGRPC, repositoryRoot, fixtureRoot, protoRoot)

	files := []string{
		"device/v1/device.proto",
		"site/v1/site.proto",
		"inventory/v1/inventory.proto",
	}
	runQualificationYunka(t, appRoot, contractGenerateArgs(protoc, repositoryRoot, protoRoot, contractOut, codeOut, codeImport, files)...)
	runQualificationYunka(t, appRoot, assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)
	assertQualificationAssemblyPlan(t, filepath.Join(contractOut, "assembly-plan.json"))
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)

	// This is the actual assembled-runtime gate: the generated consumer starts
	// the generated Bootstrap and real HTTP/gRPC runtime components and proves
	// all C10.3 runtime-closure invariants behaviorally.
	runQualificationCommand(t, fixtureRoot, qualificationConsumerEnv(), "go", "test", "-timeout=5m", "-count=1", "-mod=mod", "./...")

	// Runtime execution must not mutate the generated contract/assembly facts.
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)
}

func generateC103QualificationModules(t *testing.T, appRoot, moduleRoot string) {
	t.Helper()
	runQualificationYunka(t, appRoot, "module", "new", "--name", "site", "--root", moduleRoot, "--no-config", "--no-logger")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "inventory", "--root", moduleRoot, "--no-config", "--no-logger", "--depends-on", "site")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "device", "--root", moduleRoot, "--no-config", "--no-logger", "--database", "primary", "--depends-on", "site", "--depends-on", "inventory")
}

func writeC103ContractFixture(t *testing.T, root string) {
	t.Helper()
	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "site", "v1", "site.proto"), `syntax = "proto3";
package site.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/site/v1;sitev1";
option (yunka.dsl.v1.domain) = { name: "site" version: "v1" };

message ValidateTransferTargetRequest { string site_id = 1; }
message ValidateTransferTargetResponse { bool allowed = 1; }

service SiteApplication {
  option (yunka.dsl.v1.application) = {
    name: "management"
    operations: {
      id: "site.validate_transfer_target"
      use_case: "validate_transfer_target"
      permissions: "site.validate"
      request_type: "site.v1.ValidateTransferTargetRequest"
      response_type: "site.v1.ValidateTransferTargetResponse"
      application_method: "ValidateTransferTarget"
      execution: { transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)

	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "inventory", "v1", "inventory.proto"), `syntax = "proto3";
package inventory.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/inventory/v1;inventoryv1";
option (yunka.dsl.v1.domain) = { name: "inventory" version: "v1" };

message ReserveRequest { string sku = 1; }
message ReserveResponse { bool reserved = 1; }

service InventoryApplication {
  option (yunka.dsl.v1.application) = {
    name: "catalog"
    operations: {
      id: "inventory.reserve"
      use_case: "reserve_inventory"
      permissions: "inventory.reserve"
      request_type: "inventory.v1.ReserveRequest"
      response_type: "inventory.v1.ReserveResponse"
      application_method: "Reserve"
      execution: { transaction: TRANSACTION_LOCAL idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)

	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };

message GetDeviceRequest { string id = 1; }
message DeviceDTO { string id = 1; string serial = 2; }
message TransferDeviceRequest { string id = 1; string site_id = 2; string sku = 3; }
message TransferDeviceResponse { bool transferred = 1; }

service DeviceQueryService {
  option (yunka.dsl.v1.application) = { name: "query" };
  rpc GetDevice(GetDeviceRequest) returns (DeviceDTO) {
    option (yunka.dsl.v1.operation) = {
      id: "device.get"
      use_case: "get_device"
      public: true
      execution: { transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE }
    };
  }
}

service DeviceTransferService {
  option (yunka.dsl.v1.application) = {
    name: "transfer"
    requires: "site/management"
    requires: "inventory/catalog"
  };
  // @yunka.http POST /v1/devices:transfer body=*
  rpc TransferDevice(TransferDeviceRequest) returns (TransferDeviceResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "device.transfer"
      use_case: "transfer_device"
      permissions: "device.transfer"
      permissions: "site.validate"
      permissions: "inventory.reserve"
      tenant_required: true
      authentication: AUTHENTICATION_JWT
      requires_operations: "site.validate_transfer_target"
      requires_operations: "inventory.reserve"
      composition: COMPOSITION_LOCAL
      execution: { transaction: TRANSACTION_LOCAL idempotency: IDEMPOTENCY_NONE }
    };
  }
}
`)
}

func c103RuntimeConsumerTestSource() string {
	return `package qualification

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net"
    "net/http"
    "path/filepath"
    "strings"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    devicev1 "example.com/c102qualification/contracts/device/v1"
    inventoryv1 "example.com/c102qualification/contracts/inventory/v1"
    sitev1 "example.com/c102qualification/contracts/site/v1"
    generatedassembly "example.com/c102qualification/internal/assembly"
    deviceapplication "example.com/c102qualification/internal/device/application"
    inventoryapplication "example.com/c102qualification/internal/inventory/application"
    siteapplication "example.com/c102qualification/internal/site/application"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/test/bufconn"
    "gorm.io/gorm"
    frameworkgraph "yunka.io/framework/applicationgraph"
    "yunka.io/framework/core"
    "yunka.io/framework/core/identity"
    frameworkdiagnostics "yunka.io/framework/diagnostics"
    "yunka.io/framework/execution"
    "yunka.io/framework/operation"
    "yunka.io/framework/platform"
    "yunka.io/framework/runtimecomponent"
    "yunka.io/gateway/authz"
    graph "yunka.io/pkg/applicationgraph"
    "yunka.io/pkg/contract"
    "yunka.io/pkg/devruntime"
)

type eventLog struct {
    mu sync.Mutex
    values []string
}

func (log *eventLog) add(value string) {
    log.mu.Lock()
    log.values = append(log.values, value)
    log.mu.Unlock()
}

func (log *eventLog) snapshot() []string {
    log.mu.Lock()
    defer log.mu.Unlock()
    return append([]string(nil), log.values...)
}

type trackingUnitOfWork struct {
    mu sync.Mutex
    commits int
    rollbacks int
    closes int
}

func (unit *trackingUnitOfWork) Commit(context.Context) error {
    unit.mu.Lock()
    unit.commits++
    unit.mu.Unlock()
    return nil
}
func (unit *trackingUnitOfWork) Rollback(context.Context) error {
    unit.mu.Lock()
    unit.rollbacks++
    unit.mu.Unlock()
    return nil
}
func (unit *trackingUnitOfWork) Close() error {
    unit.mu.Lock()
    unit.closes++
    unit.mu.Unlock()
    return nil
}
func (unit *trackingUnitOfWork) counts() (int, int, int) {
    unit.mu.Lock()
    defer unit.mu.Unlock()
    return unit.commits, unit.rollbacks, unit.closes
}

type trackingTransactionFactory struct {
    mu sync.Mutex
    modes []execution.TransactionMode
    units []*trackingUnitOfWork
}

func (factory *trackingTransactionFactory) Begin(_ context.Context, mode execution.TransactionMode) (execution.UnitOfWork, error) {
    unit := &trackingUnitOfWork{}
    factory.mu.Lock()
    factory.modes = append(factory.modes, mode)
    factory.units = append(factory.units, unit)
    factory.mu.Unlock()
    return unit, nil
}
func (factory *trackingTransactionFactory) snapshot() ([]execution.TransactionMode, []*trackingUnitOfWork) {
    factory.mu.Lock()
    defer factory.mu.Unlock()
    return append([]execution.TransactionMode(nil), factory.modes...), append([]*trackingUnitOfWork(nil), factory.units...)
}

type runtimeProbe struct {
    mu sync.Mutex
    frames map[string]execution.Frame
    units map[string]execution.UnitOfWork
}

func newRuntimeProbe() *runtimeProbe {
    return &runtimeProbe{frames: map[string]execution.Frame{}, units: map[string]execution.UnitOfWork{}}
}
func (probe *runtimeProbe) capture(name string, ctx context.Context) error {
    frame, ok := execution.Current(ctx)
    if !ok {
        return fmt.Errorf("%s: execution frame missing", name)
    }
    unit, ok := execution.UnitOfWorkFrom(ctx)
    if !ok {
        return fmt.Errorf("%s: root unit of work missing", name)
    }
    probe.mu.Lock()
    probe.frames[name] = frame
    probe.units[name] = unit
    probe.mu.Unlock()
    return nil
}
func (probe *runtimeProbe) snapshot(name string) (execution.Frame, execution.UnitOfWork, bool) {
    probe.mu.Lock()
    defer probe.mu.Unlock()
    frame, frameOK := probe.frames[name]
    unit, unitOK := probe.units[name]
    return frame, unit, frameOK && unitOK
}

type allowPermissionChecker struct{ calls atomic.Int32 }
func (checker *allowPermissionChecker) HasPermissions(context.Context, string, []string, []authz.PermissionKey, authz.PermissionMode) (bool, error) {
    checker.calls.Add(1)
    return true, nil
}

type countingAuthorizer struct {
    inner authz.Authorizer
    mu sync.Mutex
    calls map[authz.OperationID]int
}
func newCountingAuthorizer(inner authz.Authorizer) *countingAuthorizer {
    return &countingAuthorizer{inner: inner, calls: map[authz.OperationID]int{}}
}
func (authorizer *countingAuthorizer) Authorize(ctx context.Context, principal identity.Principal, policy authz.Policy) (authz.Decision, error) {
    authorizer.mu.Lock()
    authorizer.calls[policy.Operation]++
    authorizer.mu.Unlock()
    return authorizer.inner.Authorize(ctx, principal, policy)
}
func (authorizer *countingAuthorizer) count(operation authz.OperationID) int {
    authorizer.mu.Lock()
    defer authorizer.mu.Unlock()
    return authorizer.calls[operation]
}

type siteService struct{ probe *runtimeProbe }
func (service *siteService) ValidateTransferTarget(ctx context.Context, _ *sitev1.ValidateTransferTargetRequest) (*sitev1.ValidateTransferTargetResponse, error) {
    if err := service.probe.capture("site", ctx); err != nil { return nil, err }
    return &sitev1.ValidateTransferTargetResponse{Allowed: true}, nil
}

type inventoryService struct{ probe *runtimeProbe }
func (service *inventoryService) Reserve(ctx context.Context, _ *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
    if err := service.probe.capture("inventory", ctx); err != nil { return nil, err }
    return &inventoryv1.ReserveResponse{Reserved: true}, nil
}

type deviceQueryService struct{}
func (*deviceQueryService) GetDevice(_ context.Context, request *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
    return &devicev1.DeviceDTO{Id: request.GetId(), Serial: "assembled-runtime"}, nil
}

type deviceTransferService struct {
    site deviceapplication.SiteManagementChildCapability
    inventory deviceapplication.InventoryCatalogChildCapability
    probe *runtimeProbe
}
func (service *deviceTransferService) TransferDevice(ctx context.Context, request *devicev1.TransferDeviceRequest) (*devicev1.TransferDeviceResponse, error) {
    if err := service.probe.capture("root", ctx); err != nil { return nil, err }
    site, err := service.site.ValidateTransferTarget(ctx, &sitev1.ValidateTransferTargetRequest{SiteId: request.GetSiteId()})
    if err != nil { return nil, err }
    if !site.GetAllowed() { return nil, errors.New("site denied") }
    inventory, err := service.inventory.Reserve(ctx, &inventoryv1.ReserveRequest{Sku: request.GetSku()})
    if err != nil { return nil, err }
    return &devicev1.TransferDeviceResponse{Transferred: inventory.GetReserved()}, nil
}

type factories struct{ probe *runtimeProbe }
func (factory factories) BuildSiteManagement(generatedassembly.SiteManagementDependencies) (siteapplication.SiteApplication, error) {
    return &siteService{probe: factory.probe}, nil
}
func (factory factories) BuildInventoryCatalog(generatedassembly.InventoryCatalogDependencies) (inventoryapplication.InventoryApplication, error) {
    return &inventoryService{probe: factory.probe}, nil
}
func (factory factories) BuildDeviceQuery(generatedassembly.DeviceQueryDependencies) (deviceapplication.QueryApplication, error) {
    return &deviceQueryService{}, nil
}
func (factory factories) BuildDeviceTransfer(dependencies generatedassembly.DeviceTransferDependencies) (deviceapplication.TransferApplication, error) {
    return &deviceTransferService{site: dependencies.SiteManagement, inventory: dependencies.InventoryCatalog, probe: factory.probe}, nil
}
var _ generatedassembly.ApplicationFactories = factories{}

func TestFullAssembledRuntimeClosure(t *testing.T) {
    ctx := context.Background()
    probe := newRuntimeProbe()
    lifecycle := &eventLog{}
    var databaseHealthy atomic.Bool

    provider, err := platform.New(platform.Options{Databases: map[string]platform.DatabaseFactory{
        "primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
            return platform.DatabaseResource{
                Database: &gorm.DB{},
                HealthFunc: func(context.Context) error {
                    if !databaseHealthy.Load() { return errors.New("primary database not ready") }
                    return nil
                },
                ShutdownFunc: func(context.Context) error { lifecycle.add("platform:primary"); return nil },
            }, nil
        }),
    }})
    if err != nil { t.Fatal(err) }

    checker := &allowPermissionChecker{}
    rbac, err := authz.NewRBACAuthorizer(checker)
    if err != nil { t.Fatal(err) }
    authorizer := newCountingAuthorizer(rbac)
    security, err := authz.NewExecutionSecurity(authorizer, nil)
    if err != nil { t.Fatal(err) }
    transactions := &trackingTransactionFactory{}
    executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions})

    mux := http.NewServeMux()
    httpListener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil { t.Fatal(err) }
    principal := identity.Principal{Subject: "fixture", TenantID: "tenant-1", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT, Authenticated: true}
    httpServer := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        secured := identity.WithPrincipal(request.Context(), principal)
        mux.ServeHTTP(writer, request.WithContext(secured))
    })}
    httpComponent, err := runtimecomponent.HTTP(runtimecomponent.HTTPOptions{Name: "http-server", Server: httpServer, Listener: httpListener})
    if err != nil { t.Fatal(err) }

    grpcListener := bufconn.Listen(1024 * 1024)
    grpcServer := grpc.NewServer()
    grpcComponent, err := runtimecomponent.GRPC(runtimecomponent.GRPCOptions{Name: "grpc-server", Server: grpcServer, Listener: grpcListener})
    if err != nil { t.Fatal(err) }

    markerFirst := core.RuntimeComponent{Name: "00-marker", StartFunc: func(context.Context) error { return nil }, ShutdownFunc: func(context.Context) error { lifecycle.add("runtime:00"); return nil }}
    markerLast := core.RuntimeComponent{Name: "zz-marker", StartFunc: func(context.Context) error { return nil }, ShutdownFunc: func(context.Context) error { lifecycle.add("runtime:zz"); return nil }}

    result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{
        Platform: provider,
        Factories: factories{probe: probe},
        Executor: executor,
        Transports: generatedassembly.TransportBindings{HTTP: mux, RPC: grpcServer},
        RuntimeComponents: []core.RuntimeComponent{httpComponent, grpcComponent, markerLast, markerFirst},
    })
    if err != nil { t.Fatal(err) }
    if result.App == nil || result.App.State() != core.AppStateReady { t.Fatalf("bootstrap result=%+v", result) }
    shutdownComplete := false
    t.Cleanup(func() {
        if !shutdownComplete {
            shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer cancel()
            _ = result.App.Shutdown(shutdownCtx)
        }
    })

    // The lifecycle may be started while a required capability is unhealthy,
    // but readiness must remain fail-closed until that capability reports healthy.
    if health := result.App.Health(ctx); health.Ready { t.Fatalf("readiness=true while database is unhealthy: %+v", health) }
    databaseHealthy.Store(true)
    if health := result.App.Health(ctx); !health.Ready { t.Fatalf("readiness=false after database became healthy: %+v", health) }

    httpClient := &http.Client{Timeout: 2 * time.Second}
    transferURL := "http://" + httpListener.Addr().String() + "/v1/devices:transfer"
    transferBody := []byte("{\"id\":\"device-1\",\"siteId\":\"site-1\",\"sku\":\"secret-payload-value\"}")
    response := doAssembledHTTPRequest(t, httpClient, http.MethodPost, transferURL, transferBody)
    payload, err := io.ReadAll(response.Body)
    response.Body.Close()
    if err != nil { t.Fatal(err) }
    if response.StatusCode != http.StatusOK || !bytes.Contains(payload, []byte("\"transferred\":true")) {
        t.Fatalf("REST transfer status=%d body=%s", response.StatusCode, payload)
    }

    // A protected root request owns exactly one authz decision and one local UoW;
    // generated child capabilities must not re-enter security or open nested UoWs.
    if got := authorizer.count(authz.OperationID("device.transfer")); got != 1 { t.Fatalf("device.transfer authorization calls=%d", got) }
    if got := checker.calls.Load(); got != 1 { t.Fatalf("permission checker calls=%d", got) }
    if authorizer.count(authz.OperationID("site.validate_transfer_target")) != 0 || authorizer.count(authz.OperationID("inventory.reserve")) != 0 {
        t.Fatalf("child operations were independently authorized: %+v", authorizer.calls)
    }
    modes, units := transactions.snapshot()
    if len(modes) != 1 || modes[0] != execution.TransactionLocal || len(units) != 1 {
        t.Fatalf("root transaction starts modes=%v units=%d", modes, len(units))
    }
    rootFrame, rootUnit, ok := probe.snapshot("root")
    if !ok || rootFrame.RootOperationID != "device.transfer" || rootFrame.OperationID != "device.transfer" || rootFrame.Depth != 0 || rootFrame.Transaction != execution.TransactionLocal {
        t.Fatalf("root frame=%+v ok=%v", rootFrame, ok)
    }
    for _, name := range []string{"site", "inventory"} {
        childFrame, childUnit, ok := probe.snapshot(name)
        if !ok || childFrame.RootOperationID != "device.transfer" || childFrame.Depth != 1 || childFrame.Transaction != execution.TransactionLocal {
            t.Fatalf("%s child frame=%+v ok=%v", name, childFrame, ok)
        }
        if childUnit != rootUnit { t.Fatalf("%s child did not join root UoW", name) }
    }
    if commits, rollbacks, closes := units[0].counts(); commits != 1 || rollbacks != 0 || closes != 1 {
        t.Fatalf("root UoW counts commit=%d rollback=%d close=%d", commits, rollbacks, closes)
    }

    // Real generated grpc-go registration/client path is externally reachable.
    dialCtx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancelDial()
    grpcConnection, err := grpc.DialContext(dialCtx, "bufnet",
        grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return grpcListener.Dial() }),
        grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
    if err != nil { t.Fatal(err) }
    defer grpcConnection.Close()
    queryClient := devicev1.NewDeviceQueryServiceClient(grpcConnection)
    query, err := queryClient.GetDevice(ctx, &devicev1.GetDeviceRequest{Id: "device-1"})
    if err != nil || query.GetSerial() != "assembled-runtime" { t.Fatalf("gRPC query=%+v err=%v", query, err) }

    // Internal Operations remain internal: no generated gRPC service registration,
    // no HTTP route, and no invented RuntimeInventory route.
    serviceInfo := grpcServer.GetServiceInfo()
    if _, ok := serviceInfo["device.v1.DeviceQueryService"]; !ok { t.Fatalf("device query gRPC service missing: %v", serviceInfo) }
    if _, ok := serviceInfo["device.v1.DeviceTransferService"]; !ok { t.Fatalf("device transfer gRPC service missing: %v", serviceInfo) }
    for service := range serviceInfo {
        if strings.HasPrefix(service, "site.") || strings.HasPrefix(service, "inventory.") { t.Fatalf("internal service was externally registered: %s", service) }
    }
    internal := doAssembledHTTPRequest(t, httpClient, http.MethodPost, "http://"+httpListener.Addr().String()+"/internal/site.validate_transfer_target", []byte("{}"))
    _, _ = io.Copy(io.Discard, internal.Body)
    internal.Body.Close()
    if internal.StatusCode != http.StatusNotFound { t.Fatalf("internal HTTP route status=%d", internal.StatusCode) }
    inventory := generatedassembly.RuntimeInventory()
    if len(inventory.Routes) != 1 || inventory.Routes[0] != "/v1/devices:transfer" || inventory.RPCServerCount != 1 || inventory.RPCClientConfigured {
        t.Fatalf("runtime inventory=%+v", inventory)
    }

    // A local child declaration may join an existing local root but may not
    // escalate a read-only root into a local transaction. The generated child
    // capability must surface the canonical execution conflict without Begin().
    transfer, ok := result.Applications.DeviceTransfer.(*deviceTransferService)
    if !ok { t.Fatalf("unexpected transfer implementation %T", result.Applications.DeviceTransfer) }
    beforeModes, _ := transactions.snapshot()
    readOnlyCtx, readOnlyRoot, err := execution.BeginRoot(context.Background(), "fixture.read_only", execution.TransactionReadOnly, []string{"inventory.reserve"}, transactions)
    if err != nil { t.Fatal(err) }
    _, escalationErr := transfer.inventory.Reserve(readOnlyCtx, &inventoryv1.ReserveRequest{Sku: "sku"})
    if !errors.Is(escalationErr, execution.ErrTransactionConflict) { t.Fatalf("escalation err=%v", escalationErr) }
    afterModes, _ := transactions.snapshot()
    if len(afterModes) != len(beforeModes)+1 || afterModes[len(afterModes)-1] != execution.TransactionReadOnly {
        t.Fatalf("child opened nested/escalated transaction: before=%v after=%v", beforeModes, afterModes)
    }
    if err := readOnlyRoot.Rollback(readOnlyCtx); err != nil { t.Fatal(err) }

    // Diagnostics and graph evidence are built from the same live App and do not
    // retain request payloads or caller secrets.
    coreReport := result.App.Diagnostics(ctx)
    if coreReport.Runtime.RouteCount != 1 || coreReport.Runtime.RPCServerCount != 1 || len(coreReport.Modules) != 3 || len(coreReport.Components) != 4 {
        t.Fatalf("core diagnostics=%+v", coreReport)
    }
    collector, err := frameworkdiagnostics.New(result.App)
    if err != nil { t.Fatal(err) }
    diagnosticsReport := collector.Snapshot(ctx)
    encodedDiagnostics, err := json.Marshal(diagnosticsReport)
    if err != nil { t.Fatal(err) }
    if bytes.Contains(encodedDiagnostics, []byte("secret-payload-value")) || bytes.Contains(encodedDiagnostics, []byte("tenant-1")) {
        t.Fatalf("diagnostics leaked payload/identity: %s", encodedDiagnostics)
    }
    if len(generatedassembly.AssemblyPlanDigest) != 64 { t.Fatalf("assembly digest=%q", generatedassembly.AssemblyPlanDigest) }

    manifest, err := contract.LoadManifest(filepath.Join("contracts", "generated", "manifest.json"))
    if err != nil { t.Fatal(err) }
    runtimeGraph, err := frameworkgraph.Compile(ctx, frameworkgraph.Contract(manifest), frameworkgraph.Core(result.App, "device/transfer"))
    if err != nil { t.Fatal(err) }
    appID := graph.ID(graph.NodeApplication, "device/transfer")
    appNode, ok := runtimeGraph.Node(appID)
    if !ok { t.Fatalf("runtime application node %s missing", appID) }
    declared, observed := false, false
    for _, evidence := range appNode.Evidence {
        declared = declared || evidence.Type == graph.EvidenceDeclared
        observed = observed || evidence.Type == graph.EvidenceObserved
    }
    if !declared || !observed { t.Fatalf("application evidence=%+v", appNode.Evidence) }
    for _, nodeID := range []string{
        graph.ID(graph.NodeRuntimeRoute, "/v1/devices:transfer"),
        graph.ID(graph.NodeModule, "device"),
        graph.ID(graph.NodeModule, "inventory"),
        graph.ID(graph.NodeModule, "site"),
        graph.ID(graph.NodeRuntimeComponent, "http-server"),
        graph.ID(graph.NodeRuntimeComponent, "grpc-server"),
    } {
        if _, ok := runtimeGraph.Node(nodeID); !ok { t.Fatalf("runtime graph node %s missing", nodeID) }
    }

    // The dev closure layer consumes explicit graph ownership. Missing the
    // diagnostics barrier fails; adding the explicit barrier succeeds and the
    // live App diagnostics validate as a current runtime report.
    devManifest := devruntime.DevManifest{
        SchemaVersion: devruntime.RuntimeClosureSchemaVersion,
        Runtime: &devruntime.RuntimeConfig{Application: "device/transfer", Closure: true},
        Processes: []devruntime.Process{{Name: "assembled", Command: []string{"./assembled"}, GraphNode: appID}},
    }
    if _, err := devruntime.BuildPlan(devManifest, t.TempDir(), nil, runtimeGraph); err == nil {
        t.Fatal("dev closure accepted Application ownership without diagnostics readiness")
    }
    devManifest.Processes[0].Readiness = &devruntime.Readiness{URL: "http://127.0.0.1:1/diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true}
    devPlan, err := devruntime.BuildPlan(devManifest, t.TempDir(), nil, runtimeGraph)
    if err != nil { t.Fatal(err) }
    devReport := devruntime.RuntimeReport{
        SchemaVersion: devruntime.RuntimeReportSchemaVersion,
        Application: "device/transfer",
        State: devruntime.RuntimeRunRunning,
        Plan: devPlan.Names(),
        Processes: []devruntime.ProcessRuntimeReport{{
            Name: "assembled", GraphNode: appID, State: devruntime.ProcessReady, Ready: true,
            Diagnostics: &devruntime.RuntimeCoreSummary{
                State: coreReport.State, HealthState: coreReport.Health.State, Live: coreReport.Health.Live, Ready: coreReport.Health.Ready,
                RouteCount: coreReport.Runtime.RouteCount, RPCClientConfigured: coreReport.Runtime.RPCClientConfigured,
                RPCServerCount: coreReport.Runtime.RPCServerCount, EventBusConfigured: coreReport.Runtime.EventBusConfigured,
            },
        }},
    }
    if err := devruntime.ValidateRuntimeClosure(devPlan, devReport); err != nil { t.Fatalf("live dev closure validation: %v", err) }

    // Generated code must visibly reuse the canonical C9 execution seams rather
    // than bypassing them with an assembly-local execution runtime.
    generatedSource := readGeneratedTree(t, "internal")
    for _, required := range []string{"operation.ExecuteTyped", "operation.ExecuteChildTyped", "kernel.Bootstrap(ctx"} {
        if !strings.Contains(generatedSource, required) { t.Fatalf("generated tree missing canonical seam %q", required) }
    }
    for _, forbidden := range []string{"BeginRoot(", "NewExecutionSecurity(", "NewExecutor(", "modulecatalog.Default()", "ServiceLocator", "reflect."} {
        if strings.Contains(generatedSource, forbidden) { t.Fatalf("generated tree contains execution/lifecycle bypass %q", forbidden) }
    }

    shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancelShutdown()
    if err := result.App.Shutdown(shutdownCtx); err != nil { t.Fatal(err) }
    shutdownComplete = true
    if got := lifecycle.snapshot(); !equalStrings(got, []string{"runtime:zz", "runtime:00", "platform:primary"}) {
        t.Fatalf("tracked reverse shutdown order=%v", got)
    }
    if result.App.State() != core.AppStateStopped { t.Fatalf("state after shutdown=%s", result.App.State()) }

    stoppedClient := &http.Client{Timeout: 200 * time.Millisecond}
    if _, err := stoppedClient.Post(transferURL, "application/json", bytes.NewReader(transferBody)); err == nil {
        t.Fatal("HTTP remained reachable after App shutdown")
    }
    stoppedCtx, cancelStopped := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancelStopped()
    if _, err := queryClient.GetDevice(stoppedCtx, &devicev1.GetDeviceRequest{Id: "after-stop"}); err == nil {
        t.Fatal("gRPC remained reachable after App shutdown")
    }
}

func doAssembledHTTPRequest(t *testing.T, client *http.Client, method, url string, body []byte) *http.Response {
    t.Helper()
    deadline := time.Now().Add(2 * time.Second)
    var lastErr error
    for time.Now().Before(deadline) {
        request, err := http.NewRequest(method, url, bytes.NewReader(body))
        if err != nil { t.Fatal(err) }
        request.Header.Set("Content-Type", "application/json")
        response, err := client.Do(request)
        if err == nil { return response }
        lastErr = err
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatalf("HTTP endpoint did not become reachable: %v", lastErr)
    return nil
}

func readGeneratedTree(t *testing.T, root string) string {
    t.Helper()
    var builder strings.Builder
    err := filepath.Walk(root, func(path string, info interface{ IsDir() bool }, walkErr error) error {
        if walkErr != nil { return walkErr }
        if info.IsDir() || !strings.HasSuffix(path, ".go") { return nil }
        data, err := io.ReadAll(mustOpen(t, path))
        if err != nil { return err }
        builder.Write(data)
        builder.WriteByte('\n')
        return nil
    })
    if err != nil { t.Fatal(err) }
    return builder.String()
}

func mustOpen(t *testing.T, path string) *openFile {
    t.Helper()
    file, err := openPath(path)
    if err != nil { t.Fatal(err) }
    return file
}

type openFile = fileAlias

func equalStrings(left, right []string) bool {
    if len(left) != len(right) { return false }
    for index := range left { if left[index] != right[index] { return false } }
    return true
}
`
}
