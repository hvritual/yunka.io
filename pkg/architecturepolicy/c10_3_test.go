package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC103RuntimeClosureKeepsCoreAppAsSingleLifecycleOwner(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}

	component := read("framework/core/runtime_component.go")
	bootstrap := read("framework/kernel/bootstrap.go")
	runtimeCodegen := read("pkg/contract/assembly_runtime_codegen.go")
	capabilityCodegen := read("pkg/contract/assembly_capability_codegen.go")
	graphAdapter := read("framework/applicationgraph/compiler.go")
	httpAdapter := read("framework/runtimecomponent/http.go")
	grpcAdapter := read("framework/runtimecomponent/grpc.go")
	devPlan := read("pkg/devruntime/plan.go")
	devClosure := read("pkg/devruntime/closure.go")
	devCommand := read("app/cmd/dev/main.go")
	combined := component + bootstrap + runtimeCodegen + capabilityCodegen + graphAdapter + httpAdapter + grpcAdapter

	for _, forbidden := range []string{
		"reflect.", "plugin.Open", "go/packages", "ServiceLocator", "serviceLocator",
		"map[string]any", "map[string]interface{}", "sync.Pool", "exec.Command(",
	} {
		if strings.Contains(combined, forbidden) {
			t.Errorf("C10.3 runtime closure contains forbidden discovery/registry token %q", forbidden)
		}
	}

	for _, forbidden := range []string{
		"gateway/authz", "execution.New", "NewExecutionScope", "BeginTransaction", "BeginTx(",
		"transaction.New", "idempotency.New", "modulecatalog.Default()",
	} {
		if strings.Contains(runtimeCodegen+capabilityCodegen, forbidden) {
			t.Errorf("generated C10.3 runtime duplicates canonical execution/lifecycle ownership with %q", forbidden)
		}
	}

	for _, forbidden := range []string{".Start(ctx", ".Shutdown(ctx", "func ("} {
		if strings.Contains(runtimeCodegen+capabilityCodegen, forbidden) {
			t.Errorf("generated C10.3 assembly owns lifecycle directly with %q", forbidden)
		}
	}

	for _, required := range []string{
		"type RuntimeComponent struct",
		"StartFunc", "HealthFunc", "ShutdownFunc",
		"type BootstrapResult[Applications any] struct",
		"App          *core.App",
		"func Bootstrap[Applications any]",
		"app.Start(ctx)",
		"func BindAssemblyRuntime",
		"func BindAssemblyCapabilityRuntime",
		"catalog, err := NewCatalog(options.AdditionalModules...)",
		"kernel.Bootstrap(ctx",
		"BuildWithCapabilities: func(capabilities modulecatalog.CapabilitySet)",
		"BuildApplicationsWithCapabilities",
		"RegisterTransports(options.Transports, applications, options.Executor)",
		"func RuntimeInventory() core.RuntimeInventory",
		"snapshot.Components",
		"type HTTPOptions struct", "Server   *http.Server", "Listener net.Listener",
		"type GRPCOptions struct", "Server   *grpcgo.Server",
		"GracefulStop()", "runtime.server.Stop()",
	} {
		if !strings.Contains(combined, required) {
			t.Errorf("C10.3 runtime closure is missing ownership/reuse marker %q", required)
		}
	}

	for _, suspicious := range []string{"Service any", "Value any", "Lookup(", "GetService(", "Services map["} {
		if strings.Contains(component, suspicious) {
			t.Errorf("RuntimeComponent grew into a service lookup surface with %q", suspicious)
		}
	}

	for _, inferred := range []string{"net.Listen(", "ListenAndServe(", "ListenAndServeTLS("} {
		if strings.Contains(httpAdapter+grpcAdapter, inferred) {
			t.Errorf("transport lifecycle adapter inferred listener/deployment topology with %q", inferred)
		}
	}

	if strings.Contains(bootstrap, "type Runtime struct") || strings.Contains(bootstrap, "type ApplicationRuntime struct") {
		t.Error("kernel bootstrap must not introduce a second runtime owner")
	}
	if strings.Contains(bootstrap, "func (result BootstrapResult") || strings.Contains(bootstrap, "func (r BootstrapResult") {
		t.Error("BootstrapResult must remain a data result and not grow lifecycle methods")
	}

	dev := devPlan + devClosure + devCommand
	for _, forbidden := range []string{"os/exec", "exec.Command(", "net.Listen(", "ListenAndServe(", "plugin.Open", "reflect."} {
		if strings.Contains(dev, forbidden) {
			t.Errorf("C10.3 dev closure inferred process/runtime topology with %q", forbidden)
		}
	}
	for _, required := range []string{
		"node.Kind == applicationgraph.NodeApplication",
		"process.Readiness == nil",
		"process.Readiness.DiagnosticsReady",
		"process.Readiness.CaptureDiagnostics",
		"func ValidateRuntimeClosure(plan Plan, report RuntimeReport) error",
		"report.State != RuntimeRunRunning",
		"runtime report plan does not match current closure plan",
		"graph ownership drift",
		"current.Diagnostics == nil",
		"devruntime.ValidateRuntimeClosure(closurePlan, report)",
	} {
		if !strings.Contains(dev, required) {
			t.Errorf("C10.3 dev closure is missing fail-closed marker %q", required)
		}
	}
	if strings.Index(devCommand, "loadPlan(c)") > strings.Index(devCommand, "devruntime.ValidateRuntimeClosure(closurePlan, report)") {
		t.Error("dev status --closure must rebuild the explicit plan before validating runtime closure")
	}
}
