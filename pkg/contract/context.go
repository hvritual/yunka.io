package contract

import (
	"fmt"
	"sort"
	"strings"
)

// OperationContractContext is read-only source context, not edit authority.
// SourceFiles use Manifest.Files' namespace (descriptor-relative for Compile,
// repository-relative for CompileInventory). ExternalImports are descriptor
// import names and must never be interpreted as local writable paths.
type OperationContractContext struct {
	OperationID string   `json:"operationId"`
	Service     string   `json:"service"`
	SourceFiles []string `json:"sourceFiles"`
	// DeclarationFiles stops at the Operation and its transitive DTO type graph.
	// Unlike SourceFiles it does not follow unrelated service-file imports.
	DeclarationFiles []string `json:"declarationFiles"`
	MessageTypes     []string `json:"messageTypes"`
	EnumTypes        []string `json:"enumTypes"`
	ExternalImports  []string `json:"externalImports,omitempty"`
}

type operationContextTarget struct{ id, service, source, request, response string }

func operationContextTargets(manifest Manifest) ([]operationContextTarget, error) {
	targets := []operationContextTarget{}
	seen := map[string]bool{}
	add := func(value operationContextTarget) error {
		value.id = strings.TrimSpace(value.id)
		if value.id == "" {
			return fmt.Errorf("contract context: declared operationId is empty")
		}
		if seen[value.id] {
			return fmt.Errorf("contract context: operation %q is declared more than once", value.id)
		}
		seen[value.id] = true
		targets = append(targets, value)
		return nil
	}
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation == nil {
				continue
			}
			if method.SourceFile != "" && method.SourceFile != service.SourceFile {
				return nil, fmt.Errorf("contract context: method %s and service %s have inconsistent provenance", method.Name, service.FullName)
			}
			if err := add(operationContextTarget{method.Operation.ID, service.FullName, service.SourceFile, method.Request, method.Response}); err != nil {
				return nil, err
			}
		}
		if service.Application != nil {
			for _, op := range service.Application.Operations {
				if err := add(operationContextTarget{op.ID, service.FullName, service.SourceFile, op.RequestType, op.ResponseType}); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].id < targets[j].id })
	return targets, nil
}

// ResolveOperationContractContext follows typed DTO references and canonical
// file imports. Import closure may include other DTOs co-located in a service;
// it does not authorize changes to every declaration in those files.
func ResolveOperationContractContext(manifest Manifest, operationID string) (OperationContractContext, error) {
	manifest.Normalize()
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return OperationContractContext{}, fmt.Errorf("contract context: operationId is required")
	}
	targets, err := operationContextTargets(manifest)
	if err != nil {
		return OperationContractContext{}, err
	}
	var target operationContextTarget
	found := false
	for _, item := range targets {
		if item.id == operationID {
			target = item
			found = true
			break
		}
	}
	if !found {
		return OperationContractContext{}, fmt.Errorf("contract context: operation %q was not found", operationID)
	}
	messages := map[string]Message{}
	enums := map[string]Enum{}
	files := map[string]File{}
	for _, file := range manifest.Files {
		if !validProvenancePath(file.Name) {
			return OperationContractContext{}, fmt.Errorf("contract context: invalid canonical source %q", file.Name)
		}
		if _, exists := files[file.Name]; exists {
			return OperationContractContext{}, fmt.Errorf("contract context: duplicate source %s", file.Name)
		}
		files[file.Name] = file
	}
	for _, message := range manifest.Messages {
		messages[normalizeTypeName(message.FullName)] = message
	}
	for _, enum := range manifest.Enums {
		enums[normalizeTypeName(enum.FullName)] = enum
	}
	required := map[string]bool{}
	messageTypes, enumTypes := []string{}, []string{}
	external := []string{}
	addFile := func(name string) error {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("contract context: missing canonical source provenance %q; recompile the contract", name)
		}
		required[name] = true
		return nil
	}
	if err := addFile(target.source); err != nil {
		return OperationContractContext{}, err
	}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		name = normalizeTypeName(name)
		if name == "" || visited[name] {
			return nil
		}
		visited[name] = true
		if message, ok := messages[name]; ok {
			messageTypes = append(messageTypes, name)
			if err := addFile(message.SourceFile); err != nil {
				return err
			}
			for _, field := range message.Fields {
				if field.Kind == "message" || field.Kind == "enum" {
					if err := visit(field.Type); err != nil {
						return err
					}
				}
				if field.Map && (field.MapValueKind == "message" || field.MapValueKind == "enum") {
					if err := visit(field.MapValueType); err != nil {
						return err
					}
				}
			}
		} else if enum, ok := enums[name]; ok {
			enumTypes = append(enumTypes, name)
			return addFile(enum.SourceFile)
		}
		// Types outside the canonical inventory (e.g. well-known protobuf types)
		// are not assigned invented local ownership. Their imports remain external.
		return nil
	}
	if err := visit(target.request); err != nil {
		return OperationContractContext{}, err
	}
	if err := visit(target.response); err != nil {
		return OperationContractContext{}, err
	}
	declarationFiles := make([]string, 0, len(required))
	for name := range required {
		declarationFiles = append(declarationFiles, name)
	}
	sort.Strings(declarationFiles)
	queue := make([]string, 0, len(required))
	for name := range required {
		queue = append(queue, name)
	}
	sort.Strings(queue)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		file := files[name]
		external = append(external, file.ExternalDependencies...)
		for _, dependency := range file.Dependencies {
			if required[dependency] {
				continue
			}
			if err := addFile(dependency); err != nil {
				return OperationContractContext{}, err
			}
			queue = append(queue, dependency)
		}
	}
	paths := make([]string, 0, len(required))
	for name := range required {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return OperationContractContext{OperationID: operationID, Service: target.service, SourceFiles: paths, DeclarationFiles: declarationFiles, MessageTypes: stableStrings(messageTypes), EnumTypes: stableStrings(enumTypes), ExternalImports: stableStrings(external)}, nil
}

func OperationContractContexts(manifest Manifest) ([]OperationContractContext, error) {
	targets, err := operationContextTargets(manifest)
	if err != nil {
		return nil, err
	}
	result := make([]OperationContractContext, 0, len(targets))
	for _, target := range targets {
		value, err := ResolveOperationContractContext(manifest, target.id)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
