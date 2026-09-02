package contract

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
	"github.com/hvritual/yunka.io/pkg/modulespec"
)

const AssemblyModuleCodePath = "assembly/zz_yunka_modules_gen.go"

func RenderAssemblyModuleCode(plan assemblyplan.Plan, bindings []ModuleBinding) ([]GeneratedAssemblyFile, error) {
	plan = assemblyplan.Normalize(plan)
	if err := assemblyplan.Validate(plan); err != nil {
		return nil, fmt.Errorf("contract assembly module codegen: assembly plan: %w", err)
	}
	byModule := make(map[string]assemblyplan.Module, len(plan.Modules))
	names := make([]string, 0, len(plan.Modules))
	for _, module := range plan.Modules {
		byModule[module.Name] = module
		names = append(names, module.Name)
	}
	sort.Strings(names)

	bindingNames := make([]string, 0, len(bindings))
	byName := make(map[string]ModuleBinding, len(bindings))
	for _, binding := range bindings {
		bindingNames = append(bindingNames, binding.Name)
		byName[binding.Name] = binding
	}
	if err := ValidateModuleBindings(bindingNames, bindings); err != nil {
		return nil, err
	}
	for _, bindingName := range bindingNames {
		module, exists := byModule[bindingName]
		if !exists {
			return nil, fmt.Errorf("contract module binding: generated Go binding %s is not selected by AssemblyPlan", bindingName)
		}
		if module.Evidence.Source == modulespec.EvidenceSource {
			return nil, fmt.Errorf("contract assembly module codegen: declarative module %s must not have a generated Go binding", bindingName)
		}
	}
	for _, module := range plan.Modules {
		if module.Evidence.Source == "generated-module-source" {
			if _, ok := byName[module.Name]; !ok {
				return nil, fmt.Errorf("contract module binding: AssemblyPlan module %s has no explicit generated Go binding", module.Name)
			}
		}
	}

	imports := newImportSet()
	imports.add("fmt", "fmt")
	imports.add("github.com/hvritual/yunka.io/framework/core/modulecatalog", "modulecatalog")
	aliases := make(map[string]string, len(bindings))
	for _, name := range names {
		if binding, ok := byName[name]; ok {
			aliases[name] = imports.add(binding.ImportPath, safeFileName(name)+"module")
		}
	}
	dependencies := moduleDependencyMap(plan)

	var b strings.Builder
	b.WriteString(GeneratedAssemblyMarker + "\n\npackage assembly\n\nimport (\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	b.WriteString("func NewCatalog() (*modulecatalog.Catalog, error) {\n")
	b.WriteString("\tcatalog := modulecatalog.New()\n")
	for _, name := range names {
		if binding, ok := byName[name]; ok {
			alias := aliases[name]
			fmt.Fprintf(&b, "\tif err := catalog.Register(%s.%s()); err != nil { return nil, fmt.Errorf(%q, err) }\n", alias, binding.DescriptorSymbol, "yunka assembly: register module "+name+": %w")
			continue
		}
		writeInlineModuleRegistration(&b, byModule[name], dependencies[name])
	}
	b.WriteString("\tif _, err := catalog.Seal(); err != nil { return nil, fmt.Errorf(\"yunka assembly: seal module catalog: %w\", err) }\n")
	b.WriteString("\treturn catalog, nil\n}\n")
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("contract assembly module codegen: format %s: %w\n%s", AssemblyModuleCodePath, err, b.String())
	}
	return []GeneratedAssemblyFile{{Path: AssemblyModuleCodePath, Content: formatted}}, nil
}

func moduleDependencyMap(plan assemblyplan.Plan) map[string][]string {
	result := make(map[string][]string, len(plan.Modules))
	for _, dependency := range plan.ModuleDependencies {
		result[dependency.From] = append(result[dependency.From], dependency.To)
	}
	for name := range result {
		sort.Strings(result[name])
	}
	return result
}

func writeInlineModuleRegistration(builder *strings.Builder, module assemblyplan.Module, dependencies []string) {
	builder.WriteString("\tif err := catalog.Register(modulecatalog.Descriptor{")
	fmt.Fprintf(builder, "Name: %q, Version: %q, ", module.Name, module.Version)
	if len(dependencies) > 0 {
		builder.WriteString("DependsOn: []string{")
		for index, dependency := range dependencies {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(builder, "%q", dependency)
		}
		builder.WriteString("}, ")
	}
	builder.WriteString("Requirements: modulecatalog.Requirements{")
	if module.Requirements.ConfigKey != "" {
		fmt.Fprintf(builder, "ConfigKey: %q, ", module.Requirements.ConfigKey)
	}
	if module.Requirements.Logger {
		builder.WriteString("Logger: true, ")
	}
	if len(module.Requirements.Databases) > 0 {
		builder.WriteString("Databases: []modulecatalog.DatabaseRequirement{")
		for index, database := range module.Requirements.Databases {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(builder, "{Name: %q}", database)
		}
		builder.WriteString("}, ")
	}
	if module.Requirements.EventBus {
		builder.WriteString("EventBus: true, ")
	}
	if len(module.Requirements.RPC) > 0 {
		builder.WriteString("RPC: []modulecatalog.RPCRequirement{")
		for index, rpc := range module.Requirements.RPC {
			if index > 0 {
				builder.WriteString(", ")
			}
			fmt.Fprintf(builder, "{Name: %q}", rpc)
		}
		builder.WriteString("}, ")
	}
	builder.WriteString("}}); err != nil { return nil, fmt.Errorf(")
	fmt.Fprintf(builder, "%q, err) }\n", "yunka assembly: register module "+module.Name+": %w")
}

func WriteAssemblyModuleCode(root string, files []GeneratedAssemblyFile) error {
	expected, err := assemblyModuleFileMap(files)
	if err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		if len(expected) == 0 {
			return nil
		}
		return fmt.Errorf("contract assembly module codegen: output root is required")
	}
	for relative, content := range expected {
		target, err := containedApplicationPath(root, relative)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeFileAtomic(target, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func CheckAssemblyModuleCode(root string, files []GeneratedAssemblyFile) ([]Drift, error) {
	expected, err := assemblyModuleFileMap(files)
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		if len(expected) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("contract assembly module codegen: output root is required")
	}
	var drift []Drift
	for relative, want := range expected {
		target, err := containedApplicationPath(root, relative)
		if err != nil {
			return nil, err
		}
		got, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, Drift{File: relative, Reason: "generated module assembly artifact is missing", Missing: true})
				continue
			}
			return nil, err
		}
		if !bytes.Equal(got, want) {
			drift = append(drift, Drift{File: relative, Reason: "generated module assembly artifact differs from canonical module binding"})
		}
	}
	return drift, nil
}

func assemblyModuleFileMap(files []GeneratedAssemblyFile) (map[string][]byte, error) {
	expected := make(map[string][]byte, len(files))
	for _, file := range files {
		relative := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file.Path)))
		if relative != AssemblyModuleCodePath {
			return nil, fmt.Errorf("contract assembly module codegen: unmanaged generated path %q", file.Path)
		}
		if _, duplicate := expected[relative]; duplicate {
			return nil, fmt.Errorf("contract assembly module codegen: duplicate generated path %q", relative)
		}
		if !bytes.HasPrefix(file.Content, []byte(GeneratedAssemblyMarker)) {
			return nil, fmt.Errorf("contract assembly module codegen: generated marker missing from %q", relative)
		}
		expected[relative] = append([]byte(nil), file.Content...)
	}
	return expected, nil
}
