package contract

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"yunka.io/pkg/assemblyplan"
)

// BindAssemblyRuntime augments the structural assembly file only after module
// bindings have been qualified. The resulting Bootstrap can therefore use the
// generated NewCatalog without introducing a second writable module source.
func BindAssemblyRuntime(manifest Manifest, plan assemblyplan.Plan, files []GeneratedAssemblyFile) ([]GeneratedAssemblyFile, error) {
	manifest.Normalize()
	plan = assemblyplan.Normalize(plan)
	if err := assemblyplan.Validate(plan); err != nil {
		return nil, fmt.Errorf("contract assembly runtime codegen: assembly plan: %w", err)
	}
	if err := verifyAssemblyPlanAgainstManifest(manifest, plan); err != nil {
		return nil, err
	}
	if len(files) != 1 || files[0].Path != AssemblyCodePath {
		return nil, fmt.Errorf("contract assembly runtime codegen: expected exactly %s", AssemblyCodePath)
	}
	routes, rpcClientConfigured, rpcServerCount, err := assemblyRuntimeInventory(manifest, plan)
	if err != nil {
		return nil, err
	}
	source := string(files[0].Content)
	source, err = addAssemblyRuntimeImports(source)
	if err != nil {
		return nil, err
	}
	source += renderAssemblyRuntimeDeclarations(routes, rpcClientConfigured, rpcServerCount)
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("contract assembly runtime codegen: format %s: %w\n%s", AssemblyCodePath, err, source)
	}
	return []GeneratedAssemblyFile{{Path: AssemblyCodePath, Content: formatted}}, nil
}

func addAssemblyRuntimeImports(source string) (string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), AssemblyCodePath, source, parser.ImportsOnly)
	if err != nil {
		return "", fmt.Errorf("contract assembly runtime codegen: parse generated imports: %w", err)
	}
	existing := make(map[string]struct{}, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", fmt.Errorf("contract assembly runtime codegen: invalid import %s: %w", spec.Path.Value, err)
		}
		existing[path] = struct{}{}
	}
	required := []string{"context", "fmt", "yunka.io/framework/core"}
	var additions strings.Builder
	for _, path := range required {
		if _, ok := existing[path]; ok {
			continue
		}
		fmt.Fprintf(&additions, "\t%q\n", path)
	}
	if additions.Len() == 0 {
		return source, nil
	}
	const marker = "import (\n"
	index := strings.Index(source, marker)
	if index < 0 {
		return "", fmt.Errorf("contract assembly runtime codegen: generated import block missing")
	}
	insert := index + len(marker)
	return source[:insert] + additions.String() + source[insert:], nil
}

func renderAssemblyRuntimeDeclarations(routes []string, rpcClientConfigured bool, rpcServerCount int) string {
	var b strings.Builder
	b.WriteString("\n\ntype RuntimeBindings struct {\n")
	b.WriteString("\tFactories ApplicationFactories\n")
	b.WriteString("\tExecutor operation.Executor\n")
	b.WriteString("}\n\n")
	b.WriteString("// RuntimeBinder constructs consumer-owned execution/application bindings only after kernel.New has prepared the App-owned Platform Provider.\n")
	b.WriteString("// It does not own lifecycle, discover services, or introduce a second authorization/transaction runtime.\n")
	b.WriteString("type RuntimeBinder func(context.Context, *platform.Provider) (RuntimeBindings, error)\n\n")
	b.WriteString("type BootstrapOptions struct {\n")
	b.WriteString("\tPlatform *platform.Provider\n")
	b.WriteString("\tFactories ApplicationFactories\n")
	b.WriteString("\tExecutor operation.Executor\n")
	b.WriteString("\tBindRuntime RuntimeBinder\n")
	b.WriteString("\tTransports TransportBindings\n")
	b.WriteString("\tRuntimeComponents []core.RuntimeComponent\n")
	b.WriteString("}\n\n")
	b.WriteString("func RuntimeInventory() core.RuntimeInventory {\n")
	b.WriteString("\treturn core.RuntimeInventory{\n")
	b.WriteString("\t\tRoutes: []string{")
	for _, route := range routes {
		fmt.Fprintf(&b, "%q,", route)
	}
	b.WriteString("},\n")
	fmt.Fprintf(&b, "\t\tRPCClientConfigured: %t,\n", rpcClientConfigured)
	fmt.Fprintf(&b, "\t\tRPCServerCount: %d,\n", rpcServerCount)
	b.WriteString("\t}\n}\n\n")
	b.WriteString("func Bootstrap(ctx context.Context, options BootstrapOptions) (kernel.BootstrapResult[Applications], error) {\n")
	b.WriteString("\tif options.BindRuntime != nil && (options.Factories != nil || options.Executor != nil) { return kernel.BootstrapResult[Applications]{}, fmt.Errorf(\"yunka assembly: BindRuntime cannot be combined with prebuilt Factories or Executor\") }\n")
	b.WriteString("\tcatalog, err := NewCatalog()\n")
	b.WriteString("\tif err != nil { return kernel.BootstrapResult[Applications]{}, fmt.Errorf(\"yunka assembly: build module catalog: %w\", err) }\n")
	b.WriteString("\tkernelOptions := KernelOptions(KernelDependencies{Platform: options.Platform, Catalog: catalog})\n")
	b.WriteString("\tkernelOptions.RuntimeComponents = append([]core.RuntimeComponent(nil), options.RuntimeComponents...)\n")
	b.WriteString("\tkernelOptions.RuntimeInventory = RuntimeInventory()\n")
	b.WriteString("\tvar runtime RuntimeBindings\n")
	b.WriteString("\treturn kernel.Bootstrap(ctx, kernel.BootstrapOptions[Applications]{\n")
	b.WriteString("\t\tKernel: kernelOptions,\n")
	b.WriteString("\t\tBuild: func() (Applications, error) {\n")
	b.WriteString("\t\t\tif options.BindRuntime == nil { runtime = RuntimeBindings{Factories: options.Factories, Executor: options.Executor}; return BuildApplications(options.Factories, options.Executor) }\n")
	b.WriteString("\t\t\tif options.Platform == nil { return Applications{}, fmt.Errorf(\"yunka assembly: Platform is required for BindRuntime\") }\n")
	b.WriteString("\t\t\truntime, err = options.BindRuntime(ctx, options.Platform)\n")
	b.WriteString("\t\t\tif err != nil { return Applications{}, fmt.Errorf(\"yunka assembly: bind runtime after Platform preparation: %w\", err) }\n")
	b.WriteString("\t\t\treturn BuildApplications(runtime.Factories, runtime.Executor)\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t\tRegister: func(applications Applications) error {\n")
	b.WriteString("\t\t\tif options.BindRuntime == nil { return RegisterTransports(options.Transports, applications, options.Executor) }\n")
	b.WriteString("\t\t\treturn RegisterTransports(options.Transports, applications, runtime.Executor)\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t})\n}\n")
	return b.String()
}

func assemblyRuntimeInventory(manifest Manifest, plan assemblyplan.Plan) ([]string, bool, int, error) {
	methods := make(map[string]Method)
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation == nil {
				continue
			}
			id := strings.TrimSpace(method.Operation.ID)
			if id == "" {
				continue
			}
			if _, duplicate := methods[id]; duplicate {
				return nil, false, 0, fmt.Errorf("contract assembly runtime codegen: duplicate method operation %s", id)
			}
			methods[id] = method
		}
	}
	routeSet := make(map[string]struct{})
	hasRPCServer := false
	for _, operation := range plan.Operations {
		for _, binding := range operation.Bindings {
			switch binding.Transport {
			case "http":
				method, ok := methods[operation.ID]
				if !ok {
					return nil, false, 0, fmt.Errorf("contract assembly runtime codegen: HTTP operation %s has no canonical method", operation.ID)
				}
				if binding.Index < 0 || binding.Index >= len(method.HTTP) {
					return nil, false, 0, fmt.Errorf("contract assembly runtime codegen: HTTP binding %s[%d] is out of range", operation.ID, binding.Index)
				}
				route := strings.TrimSpace(method.HTTP[binding.Index].Path)
				if route == "" {
					return nil, false, 0, fmt.Errorf("contract assembly runtime codegen: HTTP binding %s[%d] has empty path", operation.ID, binding.Index)
				}
				routeSet[route] = struct{}{}
			case "rpc":
				hasRPCServer = true
			default:
				return nil, false, 0, fmt.Errorf("contract assembly runtime codegen: unsupported transport %q for %s", binding.Transport, operation.ID)
			}
		}
	}
	routes := make([]string, 0, len(routeSet))
	for route := range routeSet {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	rpcClientConfigured := false
	for _, requirement := range plan.Requirements {
		if requirement.Kind == "rpc" {
			rpcClientConfigured = true
			break
		}
	}
	rpcServerCount := 0
	if hasRPCServer {
		rpcServerCount = 1
	}
	return routes, rpcClientConfigured, rpcServerCount, nil
}
