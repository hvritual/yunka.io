package qualification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	devicev1 "example.com/c103qualification/contracts/device/v1"
	inventoryv1 "example.com/c103qualification/contracts/inventory/v1"
	sitev1 "example.com/c103qualification/contracts/site/v1"
	generatedassembly "example.com/c103qualification/internal/assembly"
	deviceapplication "example.com/c103qualification/internal/device/application"
	inventoryapplication "example.com/c103qualification/internal/inventory/application"
	siteapplication "example.com/c103qualification/internal/site/application"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	frameworkgraph "github.com/hvritual/yunka.io/framework/applicationgraph"
	"github.com/hvritual/yunka.io/framework/core"
	"github.com/hvritual/yunka.io/framework/core/identity"
	frameworkdiagnostics "github.com/hvritual/yunka.io/framework/diagnostics"
	"github.com/hvritual/yunka.io/framework/execution"
	"github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/framework/platform"
	"github.com/hvritual/yunka.io/framework/runtimecomponent"
	"github.com/hvritual/yunka.io/gateway/authz"
	graph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/devruntime"
)

type eventLog struct {
	mu     sync.Mutex
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
	mu        sync.Mutex
	commits   int
	rollbacks int
	closes    int
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
	mu    sync.Mutex
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
	mu     sync.Mutex
	frames map[string]execution.Frame
	units  map[string]execution.UnitOfWork
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
	mu    sync.Mutex
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
	if err := service.probe.capture("site", ctx); err != nil {
		return nil, err
	}
	return &sitev1.ValidateTransferTargetResponse{Allowed: true}, nil
}

type inventoryService struct{ probe *runtimeProbe }

func (service *inventoryService) Reserve(ctx context.Context, _ *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if err := service.probe.capture("inventory", ctx); err != nil {
		return nil, err
	}
	return &inventoryv1.ReserveResponse{Reserved: true}, nil
}

type deviceQueryService struct{}

func (*deviceQueryService) GetDevice(_ context.Context, request *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
	return &devicev1.DeviceDTO{Id: request.GetId(), Serial: "assembled-runtime"}, nil
}

type deviceTransferService struct {
	site      deviceapplication.TransferToSiteManagementChildCapability
	inventory deviceapplication.TransferToInventoryCatalogChildCapability
	probe     *runtimeProbe
}

func (service *deviceTransferService) TransferDevice(ctx context.Context, request *devicev1.TransferDeviceRequest) (*devicev1.TransferDeviceResponse, error) {
	if err := service.probe.capture("root", ctx); err != nil {
		return nil, err
	}
	site, err := service.site.ValidateTransferTarget(ctx, &sitev1.ValidateTransferTargetRequest{SiteId: request.GetSiteId()})
	if err != nil {
		return nil, err
	}
	if !site.GetAllowed() {
		return nil, errors.New("site denied")
	}
	inventory, err := service.inventory.Reserve(ctx, &inventoryv1.ReserveRequest{Sku: request.GetSku()})
	if err != nil {
		return nil, err
	}
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
					if !databaseHealthy.Load() {
						return errors.New("primary database not ready")
					}
					return nil
				},
				ShutdownFunc: func(context.Context) error {
					lifecycle.add("platform:primary")
					return nil
				},
			}, nil
		}),
	}})
	if err != nil {
		t.Fatal(err)
	}

	checker := &allowPermissionChecker{}
	rbac, err := authz.NewRBACAuthorizer(checker)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := newCountingAuthorizer(rbac)
	security, err := authz.NewExecutionSecurity(authorizer, nil)
	if err != nil {
		t.Fatal(err)
	}
	transactions := &trackingTransactionFactory{}
	executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions})

	mux := http.NewServeMux()
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{Subject: "fixture", TenantID: "tenant-1", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT, Authenticated: true}
	httpServer := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secured := identity.WithPrincipal(request.Context(), principal)
		mux.ServeHTTP(writer, request.WithContext(secured))
	})}
	httpComponent, err := runtimecomponent.HTTP(runtimecomponent.HTTPOptions{Name: "http-server", Server: httpServer, Listener: httpListener})
	if err != nil {
		t.Fatal(err)
	}

	grpcListener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	grpcComponent, err := runtimecomponent.GRPC(runtimecomponent.GRPCOptions{Name: "grpc-server", Server: grpcServer, Listener: grpcListener})
	if err != nil {
		t.Fatal(err)
	}

	markerFirst := core.RuntimeComponent{
		Name:         "00-marker",
		StartFunc:    func(context.Context) error { return nil },
		ShutdownFunc: func(context.Context) error { lifecycle.add("runtime:00"); return nil },
	}
	markerLast := core.RuntimeComponent{
		Name:         "zz-marker",
		StartFunc:    func(context.Context) error { return nil },
		ShutdownFunc: func(context.Context) error { lifecycle.add("runtime:zz"); return nil },
	}

	result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{
		Platform:          provider,
		AdditionalModules: qualificationCapabilityDescriptors("full-closure"),
		Factories:         factories{probe: probe},
		Executor:          executor,
		Transports:        generatedassembly.TransportBindings{HTTP: mux, RPC: grpcServer},
		RuntimeComponents: []core.RuntimeComponent{httpComponent, grpcComponent, markerLast, markerFirst},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.App == nil || result.App.State() != core.AppStateReady {
		t.Fatalf("bootstrap app state=%v", result.App)
	}
	shutdownComplete := false
	t.Cleanup(func() {
		if shutdownComplete {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = result.App.Shutdown(shutdownCtx)
	})

	// App startup and readiness are separate facts. The required DB capability is
	// deliberately unhealthy after Start, so readiness must remain fail-closed.
	if health := result.App.Health(ctx); health.Ready {
		t.Fatalf("readiness=true while required database is unhealthy: %+v", health)
	}
	databaseHealthy.Store(true)
	if health := result.App.Health(ctx); !health.Ready {
		t.Fatalf("readiness=false after required database became healthy: %+v", health)
	}

	httpClient := &http.Client{Timeout: 2 * time.Second}
	transferURL := "http://" + httpListener.Addr().String() + "/v1/devices:transfer"
	transferBody := []byte("{\"id\":\"device-1\",\"siteId\":\"site-1\",\"sku\":\"secret-payload-value\"}")
	response := doAssembledHTTPRequest(t, httpClient, http.MethodPost, transferURL, transferBody)
	payload, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(payload, []byte("\"transferred\":true")) {
		t.Fatalf("REST transfer status=%d body=%s", response.StatusCode, payload)
	}

	// One protected root enters authz once. Child wrappers join the active
	// execution scope and therefore neither re-authorize nor open nested UoWs.
	if got := authorizer.count(authz.OperationID("device.transfer")); got != 1 {
		t.Fatalf("device.transfer authorization calls=%d", got)
	}
	if got := checker.calls.Load(); got != 1 {
		t.Fatalf("permission checker calls=%d", got)
	}
	if authorizer.count(authz.OperationID("site.validate_transfer_target")) != 0 || authorizer.count(authz.OperationID("inventory.reserve")) != 0 {
		t.Fatal("internal child operation performed an independent authorization decision")
	}
	modes, units := transactions.snapshot()
	if len(modes) != 1 || modes[0] != execution.TransactionLocal || len(units) != 1 {
		t.Fatalf("root transaction starts modes=%v units=%d", modes, len(units))
	}
	rootFrame, rootUnit, ok := probe.snapshot("root")
	if !ok || rootFrame.RootOperationID != "device.transfer" || rootFrame.OperationID != "device.transfer" || rootFrame.Depth != 0 || rootFrame.Transaction != execution.TransactionLocal {
		t.Fatalf("root execution frame=%+v ok=%v", rootFrame, ok)
	}
	for _, name := range []string{"site", "inventory"} {
		childFrame, childUnit, childOK := probe.snapshot(name)
		if !childOK || childFrame.RootOperationID != "device.transfer" || childFrame.Depth != 1 || childFrame.Transaction != execution.TransactionLocal {
			t.Fatalf("%s child frame=%+v ok=%v", name, childFrame, childOK)
		}
		if childUnit != rootUnit {
			t.Fatalf("%s child did not join the root UoW", name)
		}
	}
	if commits, rollbacks, closes := units[0].counts(); commits != 1 || rollbacks != 0 || closes != 1 {
		t.Fatalf("root UoW counts commit=%d rollback=%d close=%d", commits, rollbacks, closes)
	}

	// Real generated grpc-go registration/client path is reachable through the
	// App-owned gRPC runtime component.
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelDial()
	grpcConnection, err := grpc.DialContext(
		dialCtx,
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return grpcListener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer grpcConnection.Close()
	queryClient := devicev1.NewDeviceQueryServiceClient(grpcConnection)
	query, err := queryClient.GetDevice(ctx, &devicev1.GetDeviceRequest{Id: "device-1"})
	if err != nil || query.GetSerial() != "assembled-runtime" {
		t.Fatalf("gRPC query=%+v err=%v", query, err)
	}

	// Internal-only Operations do not become fake REST/RPC surfaces.
	serviceInfo := grpcServer.GetServiceInfo()
	if _, ok := serviceInfo["device.v1.DeviceQueryService"]; !ok {
		t.Fatalf("device query gRPC service missing: %v", serviceInfo)
	}
	if _, ok := serviceInfo["device.v1.DeviceTransferService"]; !ok {
		t.Fatalf("device transfer gRPC service missing: %v", serviceInfo)
	}
	for service := range serviceInfo {
		if strings.HasPrefix(service, "site.") || strings.HasPrefix(service, "inventory.") {
			t.Fatalf("internal service was externally registered: %s", service)
		}
	}
	internal := doAssembledHTTPRequest(t, httpClient, http.MethodPost, "http://"+httpListener.Addr().String()+"/internal/site.validate_transfer_target", []byte("{}"))
	_, _ = io.Copy(io.Discard, internal.Body)
	internal.Body.Close()
	if internal.StatusCode != http.StatusNotFound {
		t.Fatalf("internal HTTP route status=%d", internal.StatusCode)
	}
	inventory := generatedassembly.RuntimeInventory()
	if len(inventory.Routes) != 1 || inventory.Routes[0] != "/v1/devices:transfer" || inventory.RPCServerCount != 1 || inventory.RPCClientConfigured {
		t.Fatalf("runtime inventory=%+v", inventory)
	}

	// A local child can share a local root but must not escalate a read-only root.
	transfer, ok := result.Applications.DeviceTransfer.(*deviceTransferService)
	if !ok {
		t.Fatalf("unexpected transfer implementation %T", result.Applications.DeviceTransfer)
	}
	beforeModes, _ := transactions.snapshot()
	readOnlyCtx, readOnlyRoot, err := execution.BeginRoot(context.Background(), "fixture.read_only", execution.TransactionReadOnly, []string{"inventory.reserve"}, transactions)
	if err != nil {
		t.Fatal(err)
	}
	_, escalationErr := transfer.inventory.Reserve(readOnlyCtx, &inventoryv1.ReserveRequest{Sku: "sku"})
	if !errors.Is(escalationErr, execution.ErrTransactionConflict) {
		t.Fatalf("child transaction escalation err=%v", escalationErr)
	}
	afterModes, _ := transactions.snapshot()
	if len(afterModes) != len(beforeModes)+1 || afterModes[len(afterModes)-1] != execution.TransactionReadOnly {
		t.Fatalf("child opened a nested/escalated transaction: before=%v after=%v", beforeModes, afterModes)
	}
	if err := readOnlyRoot.Rollback(readOnlyCtx); err != nil {
		t.Fatal(err)
	}

	// Diagnostics are secret-free runtime evidence from the same live App.
	coreReport := result.App.Diagnostics(ctx)
	if coreReport.Runtime.RouteCount != 1 || coreReport.Runtime.RPCServerCount != 1 || len(coreReport.Modules) != 4 || len(coreReport.Components) != 4 {
		t.Fatalf("core diagnostics=%+v", coreReport)
	}
	collector, err := frameworkdiagnostics.New(result.App)
	if err != nil {
		t.Fatal(err)
	}
	encodedDiagnostics, err := json.Marshal(collector.Snapshot(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encodedDiagnostics, []byte("secret-payload-value")) || bytes.Contains(encodedDiagnostics, []byte("tenant-1")) {
		t.Fatalf("diagnostics leaked request/identity data: %s", encodedDiagnostics)
	}
	if len(generatedassembly.AssemblyPlanDigest) != 64 {
		t.Fatalf("assembly digest=%q", generatedassembly.AssemblyPlanDigest)
	}

	manifest, err := contract.LoadManifest(filepath.Join("contracts", "generated", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	runtimeGraph, err := frameworkgraph.Compile(ctx, frameworkgraph.Contract(manifest), frameworkgraph.Core(result.App, "device/transfer"))
	if err != nil {
		t.Fatal(err)
	}
	appID := graph.ID(graph.NodeApplication, "device/transfer")
	appNode, ok := runtimeGraph.Node(appID)
	if !ok {
		t.Fatalf("runtime application node %s missing", appID)
	}
	declared, observed := false, false
	for _, evidence := range appNode.Evidence {
		declared = declared || evidence.Type == graph.EvidenceDeclared
		observed = observed || evidence.Type == graph.EvidenceObserved
	}
	if !declared || !observed {
		t.Fatalf("application evidence=%+v", appNode.Evidence)
	}
	for _, nodeID := range []string{
		graph.ID(graph.NodeRuntimeRoute, "/v1/devices:transfer"),
		graph.ID(graph.NodeModule, "device"),
		graph.ID(graph.NodeModule, "inventory"),
		graph.ID(graph.NodeModule, "site"),
		graph.ID(graph.NodeModule, "qualification-cache"),
		graph.ID(graph.NodeRuntimeComponent, "http-server"),
		graph.ID(graph.NodeRuntimeComponent, "grpc-server"),
	} {
		if _, ok := runtimeGraph.Node(nodeID); !ok {
			t.Fatalf("runtime graph node %s missing", nodeID)
		}
	}

	// dev closure consumes explicit graph ownership: missing diagnostics barrier
	// fails, while an explicit barrier plus the live App diagnostics validates.
	devManifest := devruntime.DevManifest{
		SchemaVersion: devruntime.RuntimeClosureSchemaVersion,
		Runtime:       &devruntime.RuntimeConfig{Application: "device/transfer", Closure: true},
		Processes:     []devruntime.Process{{Name: "assembled", Command: []string{"./assembled"}, GraphNode: appID}},
	}
	if _, err := devruntime.BuildPlan(devManifest, t.TempDir(), nil, runtimeGraph); err == nil {
		t.Fatal("dev closure accepted Application ownership without diagnostics readiness")
	}
	devManifest.Processes[0].Readiness = &devruntime.Readiness{URL: "http://127.0.0.1:1/diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true}
	devPlan, err := devruntime.BuildPlan(devManifest, t.TempDir(), nil, runtimeGraph)
	if err != nil {
		t.Fatal(err)
	}
	devReport := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application:   "device/transfer",
		State:         devruntime.RuntimeRunRunning,
		Plan:          devPlan.Names(),
		Processes: []devruntime.ProcessRuntimeReport{{
			Name: "assembled", GraphNode: appID, State: devruntime.ProcessReady, Ready: true,
			Diagnostics: &devruntime.RuntimeCoreSummary{
				State: coreReport.State, HealthState: coreReport.Health.State, Live: coreReport.Health.Live, Ready: coreReport.Health.Ready,
				RouteCount: coreReport.Runtime.RouteCount, RPCClientConfigured: coreReport.Runtime.RPCClientConfigured,
				RPCServerCount: coreReport.Runtime.RPCServerCount, EventBusConfigured: coreReport.Runtime.EventBusConfigured,
			},
		}},
	}
	if err := devruntime.ValidateRuntimeClosure(devPlan, devReport); err != nil {
		t.Fatalf("live dev closure validation: %v", err)
	}

	// Generated files must visibly reuse C9 execution seams and contain no
	// assembly-local execution/authz/lifecycle implementation.
	generatedSource := readGeneratedTree(t, "internal")
	for _, required := range []string{"operation.ExecuteTyped", "operation.ExecuteChildTyped", "kernel.Bootstrap(ctx"} {
		if !strings.Contains(generatedSource, required) {
			t.Fatalf("generated tree missing canonical seam %q", required)
		}
	}
	for _, forbidden := range []string{"BeginRoot(", "NewExecutionSecurity(", "NewExecutor(", "modulecatalog.Default()", "ServiceLocator", "reflect."} {
		if strings.Contains(generatedSource, forbidden) {
			t.Fatalf("generated tree contains C9/runtime bypass %q", forbidden)
		}
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShutdown()
	if err := result.App.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	shutdownComplete = true
	if got := lifecycle.snapshot(); !equalStrings(got, []string{"runtime:zz", "runtime:00", "platform:primary"}) {
		t.Fatalf("tracked reverse shutdown order=%v", got)
	}
	if result.App.State() != core.AppStateStopped {
		t.Fatalf("state after shutdown=%s", result.App.State())
	}

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
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err == nil {
			return response
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("HTTP endpoint did not become reachable: %v", lastErr)
	return nil
}

func readGeneratedTree(t *testing.T, root string) string {
	t.Helper()
	var builder strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builder.Write(data)
		builder.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return builder.String()
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
