package contract

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/pkg/assemblyplan"
)

const AssemblyModuleCodePath = "assembly/zz_yunka_modules_gen.go"

func RenderAssemblyModuleCode(plan assemblyplan.Plan, bindings []ModuleBinding) ([]GeneratedAssemblyFile, error) {
	plan = assemblyplan.Normalize(plan)
	if err := assemblyplan.Validate(plan); err != nil {
		return nil, fmt.Errorf("contract assembly module codegen: assembly plan: %w", err)
	}
	names := make([]string, 0, len(plan.Modules))
	for _, module := range plan.Modules {
		names = append(names, module.Name)
	}
	if err := ValidateModuleBindings(names, bindings); err != nil {
		return nil, err
	}
	byName := make(map[string]ModuleBinding, len(bindings))
	for _, binding := range bindings {
		byName[binding.Name] = binding
	}
	sort.Strings(names)
	imports := newImportSet()
	imports.add("fmt", "fmt")
	imports.add("yunka.io/framework/core/modulecatalog", "modulecatalog")
	aliases := make(map[string]string, len(names))
	for _, name := range names {
		binding := byName[name]
		aliases[name] = imports.add(binding.ImportPath, safeFileName(name)+"module")
	}
	var b strings.Builder
	b.WriteString(GeneratedAssemblyMarker + "\n\npackage assembly\n\nimport (\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	b.WriteString("func NewCatalog() (*modulecatalog.Catalog, error) {\n")
	b.WriteString("\tcatalog := modulecatalog.New()\n")
	for _, name := range names {
		binding := byName[name]
		alias := aliases[name]
		fmt.Fprintf(&b, "\tif err := catalog.Register(%s.%s()); err != nil { return nil, fmt.Errorf(%q, err) }\n", alias, binding.DescriptorSymbol, "yunka assembly: register module "+name+": %w")
	}
	b.WriteString("\tif _, err := catalog.Seal(); err != nil { return nil, fmt.Errorf(\"yunka assembly: seal module catalog: %w\", err) }\n")
	b.WriteString("\treturn catalog, nil\n}\n")
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("contract assembly module codegen: format %s: %w\n%s", AssemblyModuleCodePath, err, b.String())
	}
	return []GeneratedAssemblyFile{{Path: AssemblyModuleCodePath, Content: formatted}}, nil
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
