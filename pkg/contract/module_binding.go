package contract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ModuleBinding is compiler-local derived evidence that connects an explicit
// module identity to the Go package exporting its generated descriptor. It is
// discovered from generated module source, never inferred from directory or
// package naming conventions.
type ModuleBinding struct {
	Name             string
	ImportPath       string
	DescriptorSymbol string
	Evidence         string
}

// DiscoverModuleBindings reads only the fixed generated module evidence files
// below an explicit module root: <module>/module.go and
// <module>/autoload/register.go. It does not inspect arbitrary packages or
// activate modules at runtime.
func DiscoverModuleBindings(root string) ([]ModuleBinding, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("contract module binding: module root is required")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var bindings []ModuleBinding
	seen := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		moduleRoot := filepath.Join(root, entry.Name())
		modulePath := filepath.Join(moduleRoot, "module.go")
		autoloadPath := filepath.Join(moduleRoot, "autoload", "register.go")
		if _, err := os.Stat(modulePath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		if _, err := os.Stat(autoloadPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		name, err := parseGeneratedModuleName(modulePath)
		if err != nil {
			return nil, err
		}
		binding, err := parseGeneratedAutoloadBinding(autoloadPath)
		if err != nil {
			return nil, err
		}
		binding.Name = name
		binding.Evidence = filepath.ToSlash(filepath.Join(entry.Name(), "module.go")) + "+" + filepath.ToSlash(filepath.Join(entry.Name(), "autoload", "register.go"))
		if previous, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("contract module binding: duplicate module %q from %s and %s", name, previous, binding.Evidence)
		}
		seen[name] = binding.Evidence
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Name < bindings[j].Name })
	return bindings, nil
}

func ValidateModuleBindings(planNames []string, bindings []ModuleBinding) error {
	expected := make(map[string]struct{}, len(planNames))
	for _, name := range planNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		expected[name] = struct{}{}
	}
	actual := make(map[string]ModuleBinding, len(bindings))
	for _, binding := range bindings {
		binding.Name = strings.TrimSpace(binding.Name)
		binding.ImportPath = strings.TrimSpace(binding.ImportPath)
		binding.DescriptorSymbol = strings.TrimSpace(binding.DescriptorSymbol)
		if binding.Name == "" || binding.ImportPath == "" || binding.DescriptorSymbol == "" {
			return fmt.Errorf("contract module binding: name, import path, and descriptor symbol are required")
		}
		if binding.DescriptorSymbol != "GeneratedDescriptor" {
			return fmt.Errorf("contract module binding: module %s uses unsupported descriptor symbol %q", binding.Name, binding.DescriptorSymbol)
		}
		if _, duplicate := actual[binding.Name]; duplicate {
			return fmt.Errorf("contract module binding: duplicate module binding %s", binding.Name)
		}
		actual[binding.Name] = binding
	}
	for name := range expected {
		if _, ok := actual[name]; !ok {
			return fmt.Errorf("contract module binding: AssemblyPlan module %s has no explicit generated Go binding", name)
		}
	}
	for name := range actual {
		if _, ok := expected[name]; !ok {
			return fmt.Errorf("contract module binding: generated Go binding %s is not selected by AssemblyPlan", name)
		}
	}
	return nil
}

func parseGeneratedModuleName(path string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return "", fmt.Errorf("contract module binding: parse %s: %w", path, err)
	}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, specification := range general.Specs {
			value, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range value.Names {
				if name.Name != "ModuleName" || index >= len(value.Values) {
					continue
				}
				literal, ok := value.Values[index].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return "", fmt.Errorf("contract module binding: %s ModuleName must be a string literal", path)
				}
				decoded, err := strconv.Unquote(literal.Value)
				if err != nil || strings.TrimSpace(decoded) == "" {
					return "", fmt.Errorf("contract module binding: %s has invalid ModuleName", path)
				}
				return strings.TrimSpace(decoded), nil
			}
		}
	}
	return "", fmt.Errorf("contract module binding: %s has no explicit ModuleName constant", path)
}

func parseGeneratedAutoloadBinding(path string) (ModuleBinding, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return ModuleBinding{}, fmt.Errorf("contract module binding: parse %s: %w", path, err)
	}
	importPath := ""
	for _, imported := range file.Imports {
		if imported.Name == nil || imported.Name.Name != "module" {
			continue
		}
		decoded, err := strconv.Unquote(imported.Path.Value)
		if err != nil || strings.TrimSpace(decoded) == "" {
			return ModuleBinding{}, fmt.Errorf("contract module binding: %s has invalid module import", path)
		}
		importPath = strings.TrimSpace(decoded)
	}
	if importPath == "" {
		return ModuleBinding{}, fmt.Errorf("contract module binding: %s has no explicit module import alias", path)
	}
	registered := false
	ast.Inspect(file, func(node ast.Node) bool {
		outer, ok := node.(*ast.CallExpr)
		if !ok || len(outer.Args) != 1 {
			return true
		}
		outerSelector, ok := outer.Fun.(*ast.SelectorExpr)
		if !ok || outerSelector.Sel.Name != "MustRegister" {
			return true
		}
		outerPackage, ok := outerSelector.X.(*ast.Ident)
		if !ok || outerPackage.Name != "modulecatalog" {
			return true
		}
		inner, ok := outer.Args[0].(*ast.CallExpr)
		if !ok || len(inner.Args) != 0 {
			return true
		}
		innerSelector, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok || innerSelector.Sel.Name != "GeneratedDescriptor" {
			return true
		}
		innerPackage, ok := innerSelector.X.(*ast.Ident)
		if ok && innerPackage.Name == "module" {
			registered = true
		}
		return true
	})
	if !registered {
		return ModuleBinding{}, fmt.Errorf("contract module binding: %s does not contain modulecatalog.MustRegister(module.GeneratedDescriptor())", path)
	}
	return ModuleBinding{ImportPath: importPath, DescriptorSymbol: "GeneratedDescriptor"}, nil
}
