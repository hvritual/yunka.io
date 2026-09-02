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
	"yunka.io/pkg/modulespec"
)

// DiscoverModuleSnapshot compiles the static module snapshot from either the
// canonical declarative module spec or the legacy generated module source.
// No module package is imported or executed by the compiler.
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
	var legacyNames []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		moduleRoot := filepath.Join(root, entry.Name())
		specPath := filepath.Join(moduleRoot, modulespec.Filename)
		if _, statErr := os.Stat(specPath); statErr == nil {
			legacy, legacyErr := legacyModuleSourcePresent(moduleRoot)
			if legacyErr != nil {
				return nil, nil, legacyErr
			}
			if legacy {
				return nil, nil, fmt.Errorf("contract module snapshot: module %s has both %s and legacy generated module source", entry.Name(), modulespec.Filename)
			}
			spec, loadErr := modulespec.Load(specPath)
			if loadErr != nil {
				return nil, nil, loadErr
			}
			if validateErr := modulespec.ValidateForModule(entry.Name(), spec); validateErr != nil {
				return nil, nil, fmt.Errorf("contract module snapshot: module %s: %w", entry.Name(), validateErr)
			}
			modules = append(modules, assemblyplan.ModuleInput{
				Name:      entry.Name(),
				Version:   spec.Version,
				DependsOn: append([]string(nil), spec.DependsOn...),
				Requirements: assemblyplan.ModuleRequirements{
					ConfigKey: spec.Requirements.ConfigKey,
					Logger:    spec.Requirements.Logger,
					Databases: append([]string(nil), spec.Requirements.Databases...),
					EventBus:  spec.Requirements.EventBus,
					RPC:       append([]string(nil), spec.Requirements.RPC...),
				},
				Evidence: assemblyplan.Evidence{
					Ownership: assemblyplan.OwnershipCanonical,
					Source:    modulespec.EvidenceSource,
					Ref:       filepath.ToSlash(filepath.Join(entry.Name(), modulespec.Filename)),
				},
			})
			continue
		} else if !os.IsNotExist(statErr) {
			return nil, nil, statErr
		}

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
		legacyNames = append(legacyNames, name)
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	if err := ValidateModuleBindings(legacyNames, bindings); err != nil {
		return nil, nil, err
	}
	return modules, bindings, nil
}

func legacyModuleSourcePresent(root string) (bool, error) {
	for _, relative := range []string{"module.go", "zz_yunka_module_gen.go", filepath.Join("autoload", "register.go")} {
		_, err := os.Stat(filepath.Join(root, relative))
		if err == nil {
			return true, nil
		}
		if !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
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
	seen := map[string]bool{}
	for _, element := range descriptor.Elts {
		field, value, ok := keyedElement(element)
		if !ok {
			return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s descriptor must use keyed fields", path)
		}
		if seen[field] {
			return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s descriptor repeats field %s", path, field)
		}
		seen[field] = true
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
		case "Build":
			// Build is intentionally runtime-only. The compiler validates that the
			// legacy generated descriptor has the field, but never executes or serializes it.
			if _, ok := value.(*ast.Ident); !ok {
				return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s Build must be a generated function identifier", path)
			}
		default:
			return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s descriptor has unsupported field %s", path, field)
		}
	}
	for _, required := range []string{"Name", "Version", "Requirements", "Build"} {
		if !seen[required] {
			return assemblyplan.ModuleInput{}, fmt.Errorf("contract module snapshot: %s descriptor is missing field %s", path, required)
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
	seen := map[string]bool{}
	for _, element := range composite.Elts {
		field, value, ok := keyedElement(element)
		if !ok {
			return result, fmt.Errorf("requirements must use keyed fields")
		}
		if seen[field] {
			return result, fmt.Errorf("requirements repeat field %s", field)
		}
		seen[field] = true
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
		default:
			return result, fmt.Errorf("unsupported requirements field %s", field)
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
			if !ok {
				return nil, fmt.Errorf("requirement item must use keyed fields")
			}
			if field != "Name" {
				return nil, fmt.Errorf("requirement item has unsupported field %s", field)
			}
			if found {
				return nil, fmt.Errorf("requirement item repeats Name")
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
