package contract

import (
	"fmt"
	"sort"
	"strings"
)

// OperationContractContext is a deterministic projection of the minimum
// canonical protobuf source set required to understand or edit one Operation.
// It is derived from Manifest provenance and never persisted as another source
// of truth.
type OperationContractContext struct {
	OperationID string   `json:"operationId"`
	Service     string   `json:"service"`
	SourceFiles []string `json:"sourceFiles"`
}

// ResolveOperationContractContext finds one canonical Operation and closes over
// its declaring service file, request/response message provenance, all nested
// message/enum type references, and protobuf file imports among manifest files.
func ResolveOperationContractContext(manifest Manifest, operationID string) (OperationContractContext, error) {
	manifest.Normalize()
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return OperationContractContext{}, fmt.Errorf("contract context: operationId is required")
	}

	messages := make(map[string]Message, len(manifest.Messages))
	enums := make(map[string]Enum, len(manifest.Enums))
	files := make(map[string]File, len(manifest.Files))
	for _, message := range manifest.Messages {
		messages[normalizeTypeName(message.FullName)] = message
	}
	for _, enum := range manifest.Enums {
		enums[normalizeTypeName(enum.FullName)] = enum
	}
	for _, file := range manifest.Files {
		files[file.Name] = file
	}

	var targetService Service
	var targetMethod Method
	found := false
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation == nil || strings.TrimSpace(method.Operation.ID) != operationID {
				continue
			}
			if found {
				return OperationContractContext{}, fmt.Errorf("contract context: operation %q is declared more than once", operationID)
			}
			targetService, targetMethod, found = service, method, true
		}
	}
	if !found {
		return OperationContractContext{}, fmt.Errorf("contract context: operation %q was not found", operationID)
	}

	requiredFiles := map[string]struct{}{}
	visitedTypes := map[string]struct{}{}
	var visitType func(string)
	visitType = func(typeName string) {
		typeName = normalizeTypeName(typeName)
		if typeName == "" {
			return
		}
		if _, seen := visitedTypes[typeName]; seen {
			return
		}
		visitedTypes[typeName] = struct{}{}
		if message, ok := messages[typeName]; ok {
			if message.SourceFile != "" {
				requiredFiles[message.SourceFile] = struct{}{}
			}
			for _, field := range message.Fields {
				if field.Kind == "message" || field.Kind == "enum" {
					visitType(field.Type)
				}
				if field.Map && (field.MapValueKind == "message" || field.MapValueKind == "enum") {
					visitType(field.MapValueType)
				}
			}
			return
		}
		if enum, ok := enums[typeName]; ok && enum.SourceFile != "" {
			requiredFiles[enum.SourceFile] = struct{}{}
		}
	}

	if targetService.SourceFile != "" {
		requiredFiles[targetService.SourceFile] = struct{}{}
	}
	if targetMethod.SourceFile != "" {
		requiredFiles[targetMethod.SourceFile] = struct{}{}
	}
	visitType(targetMethod.Request)
	visitType(targetMethod.Response)

	// Preserve source-level import dependencies for every required file. This
	// admits shared option/common files that do not appear as a field type but
	// are still part of the operation's compilable protobuf context.
	queue := make([]string, 0, len(requiredFiles))
	for name := range requiredFiles {
		queue = append(queue, name)
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		file, ok := files[name]
		if !ok {
			continue
		}
		for _, dependency := range file.Dependencies {
			if _, canonical := files[dependency]; !canonical {
				continue
			}
			if _, exists := requiredFiles[dependency]; exists {
				continue
			}
			requiredFiles[dependency] = struct{}{}
			queue = append(queue, dependency)
		}
	}

	paths := make([]string, 0, len(requiredFiles))
	for name := range requiredFiles {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return OperationContractContext{
		OperationID: operationID,
		Service:     targetService.FullName,
		SourceFiles: paths,
	}, nil
}

// OperationContractContexts derives all Operation contexts in stable order.
func OperationContractContexts(manifest Manifest) ([]OperationContractContext, error) {
	ids := make([]string, 0)
	seen := map[string]struct{}{}
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation == nil {
				continue
			}
			id := strings.TrimSpace(method.Operation.ID)
			if id == "" {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				return nil, fmt.Errorf("contract context: operation %q is declared more than once", id)
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	contexts := make([]OperationContractContext, 0, len(ids))
	for _, id := range ids {
		context, err := ResolveOperationContractContext(manifest, id)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, context)
	}
	return contexts, nil
}
