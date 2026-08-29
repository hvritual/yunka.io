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

	"yunka.io/pkg/assemblyplan"
)

// DiscoverModuleSnapshot compiles the static module snapshot and Go bindings
// from fixed generated module source files. No module package is imported or
// executed by the compiler.
func DiscoverModuleSnapshot(root string) ([]assemblyplan.ModuleInput, []ModuleBinding, error) {
	bindings, err := DiscoverModuleBindings(root)
	if err != nil {
		return nil, nil, err
	}
	byName := make(map[string]ModuleBinding, len(bindings))
	for _, binding := range bindings {
		byName[binding.Name] = binding
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []assemblyplan.ModuleInput{}, []ModuleBinding{}, nil
		}
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var modules []assemblyplan.ModuleInput
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		moduleRoot := filepath.Join(root, entry.Name())
		modulePath := filepath.Join(moduleRoot, "module.go")
		generatedPath := filepath.Join(moduleRoot, "zz_yunka_module_gen.go")
		if _, err := os.Stat(modulePath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(generatedPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, nil, err
		}
		name, err := parseGeneratedModuleName(modulePath)
		if err != nil {
			return nil, nil, err
		}
		binding, ok := byName[name]
		if !ok {
			return nil, nil, fmt.Errorf("contract module snapshot: module %s has no explicit autoload binding", name)
		}
		module, err := parseGeneratedDescriptor(generatedPath, name)
		if err != nil {
			return nil, nil, err
		}
		module.Evidence = assemblyplan.Evidence{
			Ownership: assemblyplan.OwnershipReused,
			Source:    "generated-module-source",
			Ref:       filepath.ToSlash(filepath.Join(entry.Name(), "zz_yunka_module_gen.go")),
		}
		if strings.TrimSpace(binding.ImportPath) == "" {
			return nil, nil, fmt.Errorf("contract module snapshot: module %s has empty Go binding", name)
		}
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, module.Name)
	}
	if err := ValidateModuleBindings(names, bindings); err != nil {
		return nil, nil, err
	}
	return modules, bindings, nil
}

func parseGeneratedDescriptor(path, moduleName string) (assemblyplan.ModuleInput, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: parse %s: %w", path, err)
	}
	var descriptor *ast.CompositeLit
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "GeneratedDescriptor" || function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if descriptor != nil {
				return false
			}
			composite, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			selector, ok := composite.Type.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "Descriptor" {
				descriptor = composite
				return false
			}
			return true
		})
	}
	if descriptor == nil {
		return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s has no GeneratedDescriptor literal", path)
	}
	result := assemblyplan.ModuleInput{Name: moduleName}
	for _, element := range descriptor.Elts {
		field, value, ok := keyedElement(element)
		if !ok {
			continue
		}
		switch field {
		case "Name":
			identifier, ok := value.(*ast.Ident)
			if !ok || identifier.Name != "ModuleName" {
				return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s descriptor Name must reference ModuleName", path)
			}
		case "Version":
			text, err := stringLiteral(value)
			if err != nil {
				return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s Version: %w", path, err)
			}
			result.Version = text
		case "DependsOn":
			values, err := stringSliceLiteral(value)
			if err != nil {
				return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s DependsOn: %w", path, err)
			}
			result.DependsOn = values
		case "Requirements":
			requirements, err := moduleRequirementsLiteral(value)
			if err != nil {
				return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s Requirements: %w", path, err)
			}
			result.Requirements = requirements
		}
	}
	return result, nil
}

func moduleRequirementsLiteral(expression ast.Expr) (assemblyplan.ModuleRequirements, error) {
	composite, ok := expression.(*ast.CompositeLit)
	if !ok {
		return assemblyplan.ModuleRequirements{}, fmt.Errorf("expected composite literal")
	}
	var result assemblyplan.ModuleRequirements
	for _, element := range composite.Elts {
		field, value, ok := keyedElement(element)
		if !ok {
			continue
		}
		switch field {
		case "ConfigKey":
			text, err := stringLiteral(value)
			if err != nil {
				return result, err
			}
			result.ConfigKey = text
		case "Logger":
			value, err := boolLiteral(value)
			if err != nil {
				return result, err
			}
			result.Logger = value
		case "EventBus":
			value, err := boolLiteral(value)
			if err != nil {
				return result, err
			}
			result.EventBus = value
		case "Databases":
			values, err := namedRequirementSlice(value)
			if err != nil {
				return result, err
			}
			result.Databases = values
		case "RPC":
			values, err := namedRequirementSlice(value)
			if err != nil {
				return result, err
			}
			result.RPC = values
		}
	}
	sort.Strings(result.Databases)
	sort.Strings(result.RPC)
	return result, nil
}

func namedRequirementSlice(expression ast.Expr) ([]string, error) {
	composite, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("expected requirement slice literal")
	}
	var result []string
	for _, element := range composite.Elts {
		item, ok := element.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("expected requirement item literal")
		}
		found := false
		for _, itemElement := range item.Elts {
			field, value, ok := keyedElement(itemElement)
			if !ok || field != "Name" {
				continue
			}
			name, err := stringLiteral(value)
			if err != nil {
				return nil, err
			}
			result = append(result, name)
			found = true
		}
		if !found {
			return nil, fmt.Errorf("requirement item has no Name")
		}
	}
	sort.Strings(result)
	return result, nil
}

func keyedElement(expression ast.Expr) (string, ast.Expr, bool) {
	keyed, ok := expression.(*ast.KeyValueExpr)
	if !ok {
		return "", nil, false
	}
	identifier, ok := keyed.Key.(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	return identifier.Name, keyed.Value, true
}

func stringLiteral(expression ast.Expr) (string, error) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", fmt.Errorf("expected string literal")
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func boolLiteral(expression ast.Expr) (bool, error) {
	identifier, ok := expression.(*ast.Ident)
	if !ok || (identifier.Name != "true" && identifier.Name != "false") {
		return false, fmt.Errorf("expected bool literal")
	}
	return identifier.Name == "true", nil
}

func stringSliceLiteral(expression ast.Expr) ([]string, error) {
	composite, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("expected string slice literal")
	}
	result := make([]string, 0, len(composite.Elts))
	for _, element := range composite.Elts {
		value, err := stringLiteral(element)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}
