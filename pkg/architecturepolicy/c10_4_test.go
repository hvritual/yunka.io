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
	kernelBootstrap := read("framework/kernel/bootstrap.go")
	platformProvider := read("framework/platform/provider.go")
	combined := runtimeCodegen + kernelBootstrap

	for _, required := range []string{
		"type RuntimeBinder func(context.Context, *platform.Provider) (RuntimeBindings, error)",
		"BindRuntime RuntimeBinder",
		"kernel.Bootstrap(ctx",
		"Build: func() (Applications, error)",
		"runtime, err = options.BindRuntime(ctx, options.Platform)",
		"BuildApplications(runtime.Factories, runtime.Executor)",
		"RegisterTransports(options.Transports, applications, runtime.Executor)",
	} {
		if !strings.Contains(runtimeCodegen, required) {
			t.Errorf("C10.4 runtime binding is missing explicit marker %q", required)
		}
	}

	bootstrapIndex := strings.Index(runtimeCodegen, "kernel.Bootstrap(ctx")
	buildIndex := strings.Index(runtimeCodegen, "Build: func() (Applications, error)")
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
		if strings.Contains(runtimeCodegen, forbidden) {
			t.Errorf("C10.4 generated runtime binder duplicates ownership/discovery with %q", forbidden)
		}
	}

	if !strings.Contains(kernelBootstrap, "app, err := New(options.Kernel)") || !strings.Contains(kernelBootstrap, "applications, err := options.Build()") {
		t.Fatal("kernel Bootstrap no longer proves kernel.New before generated Build callback")
	}
	if strings.Index(kernelBootstrap, "app, err := New(options.Kernel)") > strings.Index(kernelBootstrap, "applications, err := options.Build()") {
		t.Fatal("kernel Bootstrap must prepare/build the App before the generated runtime binder executes")
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
