package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC104RuntimeBindingIsExplicitAndKeepsExistingOwners(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}

	runtimeCodegen := read("pkg/contract/assembly_runtime_codegen.go")
	capabilityCodegen := read("pkg/contract/assembly_capability_codegen.go")
	kernelBootstrap := read("framework/kernel/bootstrap.go")
	platformProvider := read("framework/platform/provider.go")
	combined := runtimeCodegen + capabilityCodegen + kernelBootstrap

	for _, required := range []string{
		"type RuntimeBinder func(context.Context, *platform.Provider) (RuntimeBindings, error)",
		"type RuntimeCapabilityBinder func(context.Context, *platform.Provider, modulecatalog.CapabilitySet) (RuntimeBindings, error)",
		"BindRuntime RuntimeBinder",
		"BindRuntimeWithCapabilities RuntimeCapabilityBinder",
		"kernel.Bootstrap(ctx",
		"BuildWithCapabilities: func(capabilities modulecatalog.CapabilitySet) (Applications, error)",
		"runtime, err = options.BindRuntime(ctx, options.Platform)",
		"BuildApplicationsWithCapabilities",
		"RegisterTransports(options.Transports, applications, runtime.Executor)",
	} {
		if !strings.Contains(combined, required) {
			t.Errorf("C10.4 runtime binding is missing explicit marker %q", required)
		}
	}

	bootstrapIndex := strings.Index(runtimeCodegen, "kernel.Bootstrap(ctx")
	buildIndex := strings.Index(runtimeCodegen, "BuildWithCapabilities: func(capabilities modulecatalog.CapabilitySet) (Applications, error)")
	binderIndex := strings.Index(runtimeCodegen, "runtime, err = options.BindRuntime(ctx, options.Platform)")
	if bootstrapIndex < 0 || buildIndex < bootstrapIndex || binderIndex < buildIndex {
		t.Fatalf("runtime binder must remain inside kernel Bootstrap Build sequencing: bootstrap=%d build=%d binder=%d", bootstrapIndex, buildIndex, binderIndex)
	}

	for _, forbidden := range []string{
		"options.Platform.Prepare(", "platform.New(", "modulecatalog.Default()",
		"gateway/authz", "execution.New", "BeginTransaction", "BeginTx(",
		"reflect.", "plugin.Open", "go/packages", "ServiceLocator", "serviceLocator",
		"map[string]any", "map[string]interface{}", "sync.Pool",
	} {
		if strings.Contains(runtimeCodegen+capabilityCodegen, forbidden) {
			t.Errorf("C10.4 generated runtime binder duplicates ownership/discovery with %q", forbidden)
		}
	}

	newIndex := strings.Index(kernelBootstrap, "app, capabilities, err := newWithCapabilities(options.Kernel)")
	capabilityBuildIndex := strings.Index(kernelBootstrap, "applications, err = options.BuildWithCapabilities(capabilities)")
	legacyBuildIndex := strings.Index(kernelBootstrap, "applications, err = options.Build()")
	if newIndex < 0 || capabilityBuildIndex < newIndex || legacyBuildIndex < newIndex {
		t.Fatalf("kernel Bootstrap must prepare the App/capability snapshot before either typed construction callback: new=%d capabilityBuild=%d legacyBuild=%d", newIndex, capabilityBuildIndex, legacyBuildIndex)
	}
	if !strings.Contains(kernelBootstrap, "if (options.Build == nil) == (options.BuildWithCapabilities == nil)") {
		t.Fatal("kernel Bootstrap must fail closed unless exactly one build callback is configured")
	}
	if !strings.Contains(kernelBootstrap, "if err := options.Register(applications); err != nil") || strings.Index(kernelBootstrap, "if err := options.Register(applications); err != nil") < capabilityBuildIndex {
		t.Fatal("kernel Bootstrap must register transports only after typed Application construction")
	}
	if !strings.Contains(kernelBootstrap, "if err := app.Start(ctx); err != nil") || strings.Index(kernelBootstrap, "if err := app.Start(ctx); err != nil") < capabilityBuildIndex {
		t.Fatal("kernel Bootstrap must start the App only after typed Application construction")
	}

	for _, required := range []string{
		"func (provider *Provider) Prepare(",
		"provider.state = providerStatePrepared",
		"func (provider *Provider) ForModule(",
	} {
		if !strings.Contains(platformProvider, required) {
			t.Errorf("Platform Provider lost prepared capability contract %q", required)
		}
	}

	if strings.Contains(combined, "type Runtime struct") || strings.Contains(combined, "type ApplicationRuntime struct") {
		t.Fatal("C10.4 runtime binding must not introduce a second runtime owner")
	}
}
