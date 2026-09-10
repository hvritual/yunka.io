package implementation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
)

type dependencyShape struct {
	Key            string
	ContractImport string
	ContractName   string
	Methods        []string
}

func (dependency dependencyShape) Uses(method string) bool {
	index := sort.SearchStrings(dependency.Methods, method)
	return index < len(dependency.Methods) && dependency.Methods[index] == method
}

func selectDependencies(files []contract.GeneratedApplicationFile, manifest contract.Manifest, service contract.Service, port portShape, contractImport string) ([]dependencyShape, error) {
	if service.Application == nil || len(service.Application.Requires) == 0 {
		return nil, nil
	}
	const suffix = "_application_port_gen.go"
	if !strings.HasSuffix(port.path, suffix) {
		return nil, fmt.Errorf("add implementation: canonical port path has no capability sibling")
	}
	capabilityPath := strings.TrimSuffix(port.path, suffix) + "_capability_ports_gen.go"
	var source []byte
	for _, file := range files {
		if file.Path != capabilityPath {
			continue
		}
		if source != nil {
			return nil, fmt.Errorf("add implementation: duplicate canonical capability artifact %s", capabilityPath)
		}
		source = file.Content
	}
	if source == nil {
		return nil, fmt.Errorf("add implementation: canonical capability artifact %s is missing", capabilityPath)
	}

	parsed, err := parser.ParseFile(token.NewFileSet(), capabilityPath, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	interfaces := map[string]*ast.InterfaceType{}
	var provider *ast.InterfaceType
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			interfaces[typeSpec.Name.Name] = iface
			if strings.HasSuffix(typeSpec.Name.Name, "Capabilities") {
				if provider != nil {
					return nil, fmt.Errorf("add implementation: multiple canonical capability providers in %s", capabilityPath)
				}
				provider = iface
			}
		}
	}
	if provider == nil {
		return nil, fmt.Errorf("add implementation: canonical capability provider missing in %s", capabilityPath)
	}

	requires := append([]string(nil), service.Application.Requires...)
	sort.Strings(requires)
	if len(provider.Methods.List) != len(requires) {
		return nil, fmt.Errorf("add implementation: canonical capability dependency count mismatch")
	}

	targets := map[string]contract.Service{}
	for _, candidate := range manifest.Services {
		if candidate.Application == nil {
			continue
		}
		targets[candidate.Domain+"/"+candidate.Application.Name] = candidate
	}

	result := make([]dependencyShape, 0, len(requires))
	for index, field := range provider.Methods.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("add implementation: malformed canonical capability provider method")
		}
		fn, ok := field.Type.(*ast.FuncType)
		if !ok || fn.Params == nil || len(fn.Params.List) != 0 || fn.Results == nil || len(fn.Results.List) != 1 {
			return nil, fmt.Errorf("add implementation: malformed canonical capability provider signature")
		}
		contractType, ok := fn.Results.List[0].Type.(*ast.Ident)
		if !ok || !strings.HasSuffix(contractType.Name, "ChildCapability") {
			return nil, fmt.Errorf("add implementation: canonical dependency result is not a child capability")
		}
		capability, ok := interfaces[contractType.Name]
		if !ok {
			return nil, fmt.Errorf("add implementation: canonical child capability %s is missing", contractType.Name)
		}

		key := requires[index]
		target, ok := targets[key]
		if !ok || target.Application == nil {
			return nil, fmt.Errorf("add implementation: canonical dependency target %s is missing", key)
		}
		targetMethods, err := operationMethodIndex(target)
		if err != nil {
			return nil, err
		}
		capabilityMethods := map[string]struct{}{}
		for _, method := range capability.Methods.List {
			if len(method.Names) != 1 {
				return nil, fmt.Errorf("add implementation: malformed canonical child capability method")
			}
			capabilityMethods[method.Names[0].Name] = struct{}{}
		}

		usedBy := map[string]struct{}{}
		for sourceMethod, requiredOperations := range sourceOperationRequirements(service) {
			for _, operationID := range requiredOperations {
				targetMethod, belongs := targetMethods[operationID]
				if !belongs {
					continue
				}
				if _, generated := capabilityMethods[targetMethod]; !generated {
					return nil, fmt.Errorf("add implementation: canonical child capability %s omits required operation %s", contractType.Name, operationID)
				}
				usedBy[sourceMethod] = struct{}{}
			}
		}
		methods := make([]string, 0, len(usedBy))
		for method := range usedBy {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		result = append(result, dependencyShape{Key: key, ContractImport: contractImport, ContractName: contractType.Name, Methods: methods})
	}
	return result, nil
}

func validateDependencyOperations(manifest contract.Manifest, service contract.Service) error {
	if service.Application == nil {
		return nil
	}
	sourceKey := service.Domain + "/" + service.Application.Name
	declared := map[string]struct{}{}
	for _, dependency := range service.Application.Requires {
		declared[dependency] = struct{}{}
	}
	owners := map[string]string{}
	for _, candidate := range manifest.Services {
		if candidate.Application == nil {
			continue
		}
		applicationKey := candidate.Domain + "/" + candidate.Application.Name
		methods, err := operationMethodIndex(candidate)
		if err != nil {
			return err
		}
		for operationID := range methods {
			if existing, duplicate := owners[operationID]; duplicate && existing != applicationKey {
				return fmt.Errorf("add implementation: operation %s has ambiguous canonical owner", operationID)
			}
			owners[operationID] = applicationKey
		}
	}
	for _, requiredOperations := range sourceOperationRequirements(service) {
		for _, operationID := range requiredOperations {
			owner, ok := owners[operationID]
			if !ok {
				return fmt.Errorf("add implementation: unknown required operation %s", operationID)
			}
			if owner == sourceKey {
				continue
			}
			if _, ok := declared[owner]; !ok {
				return fmt.Errorf("add implementation: required operation belongs to undeclared application capability: %s", owner)
			}
		}
	}
	return nil
}

func operationMethodIndex(service contract.Service) (map[string]string, error) {
	index := map[string]string{}
	add := func(operation contract.OperationDeclaration, method string) error {
		if operation.ID == "" {
			return nil
		}
		if existing, duplicate := index[operation.ID]; duplicate && existing != method {
			return fmt.Errorf("add implementation: operation %s has ambiguous canonical method", operation.ID)
		}
		index[operation.ID] = method
		return nil
	}
	for _, method := range service.Methods {
		if method.Operation != nil {
			if err := add(*method.Operation, method.Name); err != nil {
				return nil, err
			}
		}
	}
	if service.Application != nil {
		for _, operation := range service.Application.Operations {
			if err := add(operation, operation.ApplicationMethod); err != nil {
				return nil, err
			}
		}
	}
	return index, nil
}

func sourceOperationRequirements(service contract.Service) map[string][]string {
	result := map[string][]string{}
	for _, method := range service.Methods {
		if method.Operation != nil {
			result[method.Name] = append([]string(nil), method.Operation.RequiresOperations...)
		}
	}
	if service.Application != nil {
		for _, operation := range service.Application.Operations {
			result[operation.ApplicationMethod] = append([]string(nil), operation.RequiresOperations...)
		}
	}
	return result
}
