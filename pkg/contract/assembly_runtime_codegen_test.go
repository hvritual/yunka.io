package contract

import (
	"bytes"
	"strings"
	"testing"
)

func TestBindAssemblyRuntimeGeneratesCanonicalKernelBootstrapAndInventory(t *testing.T) {
	manifest, plan := c102AssemblyFixture(t)
	files, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	files, err = BindAssemblyRuntime(manifest, plan, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != AssemblyCodePath {
		t.Fatalf("unexpected runtime-bound assembly files: %#v", files)
	}
	source := string(files[0].Content)
	for _, required := range []string{
		"type RuntimeBindings struct",
		"type RuntimeBinder func(context.Context, *platform.Provider) (RuntimeBindings, error)",
		"type BootstrapOptions struct",
		"Platform *platform.Provider",
		"BindRuntime RuntimeBinder",
		"RuntimeComponents []core.RuntimeComponent",
		"func RuntimeInventory() core.RuntimeInventory",
		`[]string{"/v1/devices:transfer"}`,
		"RPCClientConfigured: false",
		"RPCServerCount:      1",
		"func Bootstrap(ctx context.Context, options BootstrapOptions) (kernel.BootstrapResult[Applications], error)",
		"catalog, err := NewCatalog()",
		"kernel.Bootstrap(ctx, kernel.BootstrapOptions[Applications]",
		"BuildApplications(options.Factories, options.Executor)",
		"runtime, err = options.BindRuntime(ctx, options.Platform)",
		"BuildApplications(runtime.Factories, runtime.Executor)",
		"RegisterTransports(options.Transports, applications, options.Executor)",
		"RegisterTransports(options.Transports, applications, runtime.Executor)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("runtime-bound assembly missing %q:\n%s", required, source)
		}
	}
	for _, forbidden := range []string{"reflect.", "ServiceLocator", "serviceLocator", "modulecatalog.Default()", "func (", ".Start(ctx", ".Shutdown(ctx", "options.Platform.Prepare("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("runtime-bound assembly contains forbidden ownership/discovery surface %q:\n%s", forbidden, source)
		}
	}
	if strings.Count(source, `"/v1/devices:transfer"`) != 1 {
		t.Fatalf("runtime inventory did not contain exactly the explicit HTTP binding:\n%s", source)
	}

	bootstrapIndex := strings.Index(source, "kernel.Bootstrap(ctx")
	buildIndex := strings.Index(source, "Build: func()")
	binderIndex := strings.Index(source, "runtime, err = options.BindRuntime(ctx, options.Platform)")
	if bootstrapIndex < 0 || buildIndex < bootstrapIndex || binderIndex < buildIndex {
		t.Fatalf("runtime binder escaped kernel Build sequencing: bootstrap=%d build=%d binder=%d\n%s", bootstrapIndex, buildIndex, binderIndex, source)
	}
}

func TestBindAssemblyRuntimeIsSourceOrderIndependent(t *testing.T) {
	manifest, plan := c102AssemblyFixture(t)
	base, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := BindAssemblyRuntime(manifest, plan, base)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Services[0], manifest.Services[1] = manifest.Services[1], manifest.Services[0]
	plan.Applications[0], plan.Applications[1] = plan.Applications[1], plan.Applications[0]
	plan.Operations[0], plan.Operations[1] = plan.Operations[1], plan.Operations[0]
	secondBase, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BindAssemblyRuntime(manifest, plan, secondBase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first[0].Content, second[0].Content) {
		t.Fatalf("runtime bootstrap depends on source enumeration order:\nfirst=%s\nsecond=%s", first[0].Content, second[0].Content)
	}
}
