package contract

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

// BindAssemblyCapabilities projects compiler-visible Application capability
// requirements into the generated constructor boundary. The resolver exists
// only inside BuildApplicationsWithCapabilities; Applications receive concrete
// typed dependencies and never retain CapabilitySet.
func BindAssemblyCapabilities(plan assemblyplan.Plan, files []GeneratedAssemblyFile) ([]GeneratedAssemblyFile, error) {
	plan = assemblyplan.Normalize(plan)
	if err := assemblyplan.Validate(plan); err != nil {
		return nil, fmt.Errorf("contract assembly capability codegen: assembly plan: %w", err)
	}
	if len(files) != 1 || files[0].Path != AssemblyCodePath {
		return nil, fmt.Errorf("contract assembly capability codegen: expected exactly %s", AssemblyCodePath)
	}
	if !assemblyPlanHasCapabilities(plan) {
		return []GeneratedAssemblyFile{{Path: files[0].Path, Content: append([]byte(nil), files[0].Content...)}}, nil
	}

	source, aliases, err := addAssemblyCapabilityImports(string(files[0].Content), plan)
	if err != nil {
		return nil, err
	}
	for _, application := range plan.Applications {
		if len(application.Capabilities) == 0 {
			continue
		}
		symbol := exportedApplicationSymbol(strings.ReplaceAll(application.ID, "/", "_"))
		if symbol == "" {
			return nil, fmt.Errorf("contract assembly capability codegen: application %s has no generated symbol", application.ID)
		}
		bindings, err := assemblyCapabilityBindings(plan, application, symbol, aliases)
		if err != nil {
			return nil, err
		}
		source, err = addCapabilityDependencyFields(source, symbol, bindings)
		if err != nil {
			return nil, err
		}
	}

	const buildSignature = "func BuildApplications(factories ApplicationFactories, executor operation.Executor) (Applications, error) {\n"
	if strings.Count(source, buildSignature) != 1 {
		return nil, fmt.Errorf("contract assembly capability codegen: generated BuildApplications signature missing or ambiguous")
	}
	source = strings.Replace(source, buildSignature,
		"func BuildApplications(factories ApplicationFactories, executor operation.Executor) (Applications, error) {\n"+
			"\treturn BuildApplicationsWithCapabilities(factories, executor, modulecatalog.EmptyCapabilitySet())\n"+
			"}\n\n"+
			"func BuildApplicationsWithCapabilities(factories ApplicationFactories, executor operation.Executor, capabilities modulecatalog.CapabilitySet) (Applications, error) {\n", 1)

	for _, application := range plan.Applications {
		if len(application.Capabilities) == 0 {
			continue
		}
		symbol := exportedApplicationSymbol(strings.ReplaceAll(application.ID, "/", "_"))
		bindings, err := assemblyCapabilityBindings(plan, application, symbol, aliases)
		if err != nil {
			return nil, err
		}
		source, err = bindCapabilityFactoryCall(source, application.ID, symbol, bindings)
		if err != nil {
			return nil, err
		}
	}

	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("contract assembly capability codegen: format %s: %w\n%s", AssemblyCodePath, err, source)
	}
	return []GeneratedAssemblyFile{{Path: AssemblyCodePath, Content: formatted}}, nil
}

// BindAssemblyCapabilityRuntime switches generated Bootstrap call sites from
// the compatibility BuildApplications wrapper to the capability-aware
// constructor. Kernel still owns App preparation and provides the immutable
// CapabilitySet before transport registration or RuntimeComponent start.
func BindAssemblyCapabilityRuntime(plan assemblyplan.Plan, files []GeneratedAssemblyFile) ([]GeneratedAssemblyFile, error) {
	plan = assemblyplan.Normalize(plan)
	if err := assemblyplan.Validate(plan); err != nil {
		return nil, fmt.Errorf("contract assembly capability runtime: assembly plan: %w", err)
	}
	if len(files) != 1 || files[0].Path != AssemblyCodePath {
		return nil, fmt.Errorf("contract assembly capability runtime: expected exactly %s", AssemblyCodePath)
	}
	if !assemblyPlanHasCapabilities(plan) {
		return []GeneratedAssemblyFile{{Path: files[0].Path, Content: append([]byte(nil), files[0].Content...)}}, nil
	}
	source := string(files[0].Content)
	replacements := [][2]string{
		{
			"return BuildApplications(options.Factories, options.Executor)",
			"return BuildApplicationsWithCapabilities(options.Factories, options.Executor, capabilities)",
		},
		{
			"return BuildApplications(runtime.Factories, runtime.Executor)",
			"return BuildApplicationsWithCapabilities(runtime.Factories, runtime.Executor, capabilities)",
		},
	}
	for _, replacement := range replacements {
		if strings.Count(source, replacement[0]) != 1 {
			return nil, fmt.Errorf("contract assembly capability runtime: expected generated call %q exactly once", replacement[0])
		}
		source = strings.Replace(source, replacement[0], replacement[1], 1)
	}
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("contract assembly capability runtime: format %s: %w\n%s", AssemblyCodePath, err, source)
	}
	return []GeneratedAssemblyFile{{Path: AssemblyCodePath, Content: formatted}}, nil
}

type generatedCapabilityBinding struct {
	Name     string
	Package  string
	Type     string
	Alias    string
	Field    string
	Local    string
	TypeExpr string
}

func assemblyPlanHasCapabilities(plan assemblyplan.Plan) bool {
	for _, application := range plan.Applications {
		if len(application.Capabilities) > 0 {
			return true
		}
	}
	return false
}

func assemblyCapabilityBindings(plan assemblyplan.Plan, application assemblyplan.Application, symbol string, aliases map[string]string) ([]generatedCapabilityBinding, error) {
	usedFields := make(map[string]string)
	for _, edge := range plan.ApplicationDependencies {
		if edge.From != application.ID {
			continue
		}
		field := exportedApplicationSymbol(strings.ReplaceAll(edge.To, "/", "_"))
		usedFields[field] = "application dependency " + edge.To
	}
	result := make([]generatedCapabilityBinding, 0, len(application.Capabilities))
	for _, capability := range application.Capabilities {
		field := exportedApplicationSymbol(capability.Name)
		if field == "" {
			return nil, fmt.Errorf("contract assembly capability codegen: application %s capability %s has no generated field", application.ID, capability.Name)
		}
		if owner, collision := usedFields[field]; collision {
			return nil, fmt.Errorf("contract assembly capability codegen: application %s capability %s field %s collides with %s", application.ID, capability.Name, field, owner)
		}
		usedFields[field] = "capability " + capability.Name
		alias := aliases[capability.Package]
		if alias == "" {
			return nil, fmt.Errorf("contract assembly capability codegen: package alias missing for %s", capability.Package)
		}
		result = append(result, generatedCapabilityBinding{
			Name: capability.Name, Package: capability.Package, Type: capability.Type,
			Alias: alias, Field: field, Local: lowerFirstIdentifier(symbol + field + "Capability"),
			TypeExpr: alias + "." + capability.Type,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func addCapabilityDependencyFields(source, symbol string, bindings []generatedCapabilityBinding) (string, error) {
	var fields strings.Builder
	for _, binding := range bindings {
		fmt.Fprintf(&fields, "\t%s %s\n", binding.Field, binding.TypeExpr)
	}
	empty := "type " + symbol + "Dependencies struct{}\n"
	if strings.Count(source, empty) == 1 {
		return strings.Replace(source, empty, "type "+symbol+"Dependencies struct {\n"+fields.String()+"}\n", 1), nil
	}
	marker := "type " + symbol + "Dependencies struct {\n"
	if strings.Count(source, marker) != 1 {
		return "", fmt.Errorf("contract assembly capability codegen: dependency struct for %s missing or ambiguous", symbol)
	}
	return strings.Replace(source, marker, marker+fields.String(), 1), nil
}

func bindCapabilityFactoryCall(source, applicationID, symbol string, bindings []generatedCapabilityBinding) (string, error) {
	prefix := "\tapplications." + symbol + ", err = factories.Build" + symbol + "(" + symbol + "Dependencies{"
	start := strings.Index(source, prefix)
	if start < 0 || strings.Index(source[start+len(prefix):], prefix) >= 0 {
		return "", fmt.Errorf("contract assembly capability codegen: factory call for %s missing or ambiguous", applicationID)
	}
	end := strings.IndexByte(source[start:], '\n')
	if end < 0 {
		return "", fmt.Errorf("contract assembly capability codegen: factory call for %s is unterminated", applicationID)
	}
	end += start
	line := source[start:end]
	if !strings.HasSuffix(line, "})") {
		return "", fmt.Errorf("contract assembly capability codegen: factory call for %s has unexpected shape %q", applicationID, line)
	}
	existing := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "})")

	var before strings.Builder
	entries := make([]string, 0, len(bindings)+1)
	if strings.TrimSpace(existing) != "" {
		entries = append(entries, existing)
	}
	for _, binding := range bindings {
		fmt.Fprintf(&before,
			"\t%s, err := modulecatalog.ResolveCapability(capabilities, modulecatalog.MustCapabilityKey[%s](%q, %q, %q))\n",
			binding.Local, binding.TypeExpr, binding.Name, binding.Package, binding.Type)
		fmt.Fprintf(&before,
			"\tif err != nil { return Applications{}, fmt.Errorf(%q, err) }\n",
			"yunka assembly: build "+applicationID+" capability "+binding.Name+": %w")
		entries = append(entries, binding.Field+": "+binding.Local)
	}
	replacement := before.String() + prefix + strings.Join(entries, ", ") + "})"
	return source[:start] + replacement + source[end:], nil
}

func addAssemblyCapabilityImports(source string, plan assemblyplan.Plan) (string, map[string]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), AssemblyCodePath, source, parser.ImportsOnly)
	if err != nil {
		return "", nil, fmt.Errorf("contract assembly capability codegen: parse generated imports: %w", err)
	}
	byPath := make(map[string]string, len(parsed.Imports))
	used := make(map[string]string, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", nil, fmt.Errorf("contract assembly capability codegen: invalid import %s: %w", spec.Path.Value, err)
		}
		alias := ""
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias == "" {
			alias = sanitizeGoIdentifier(filepath.Base(path))
		}
		byPath[path] = alias
		used[alias] = path
	}
	packageSet := make(map[string]struct{})
	for _, application := range plan.Applications {
		for _, capability := range application.Capabilities {
			packageSet[capability.Package] = struct{}{}
		}
	}
	paths := make([]string, 0, len(packageSet))
	for path := range packageSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var additions strings.Builder
	for _, path := range paths {
		if _, exists := byPath[path]; exists {
			continue
		}
		preferred := sanitizeGoIdentifier(filepath.Base(path))
		if preferred == "" {
			preferred = "capability"
		}
		alias := preferred
		for index := 2; ; index++ {
			if owner, exists := used[alias]; !exists || owner == path {
				break
			}
			alias = preferred + strconv.Itoa(index)
		}
		byPath[path] = alias
		used[alias] = path
		fmt.Fprintf(&additions, "\t%s %q\n", alias, path)
	}
	if additions.Len() > 0 {
		const marker = "import (\n"
		index := strings.Index(source, marker)
		if index < 0 {
			return "", nil, fmt.Errorf("contract assembly capability codegen: generated import block missing")
		}
		insert := index + len(marker)
		source = source[:insert] + additions.String() + source[insert:]
	}
	return source, byPath, nil
}
