package contract

import (
	"fmt"
	"sort"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Diagnostic struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Message  string   `json:"message"`
}

func Lint(manifest Manifest) []Diagnostic {
	manifest.Normalize()
	messages := make(map[string]Message, len(manifest.Messages))
	enums := make(map[string]Enum, len(manifest.Enums))
	for _, message := range manifest.Messages {
		messages[message.FullName] = message
	}
	for _, enum := range manifest.Enums {
		enums[enum.FullName] = enum
	}

	var diagnostics []Diagnostic
	for _, file := range manifest.Files {
		if file.Domain == nil {
			continue
		}
		path := "file." + file.Name + ".domain"
		if !validPolicyKey(file.Domain.Name) {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "domain name must be a stable lowercase business key"})
		}
	}

	for _, message := range manifest.Messages {
		seenNames := make(map[string]struct{})
		seenNumbers := make(map[int32]struct{})
		if message.DTO != nil {
			switch message.DTO.Kind {
			case "input", "output", "shared", "event", "value_object":
			default:
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: "message." + message.FullName + ".dto", Message: "dto kind must be input, output, shared, event, or value_object"})
			}
		}
		for _, field := range message.Fields {
			path := "message." + message.FullName + ".field." + field.Name
			if field.Number <= 0 {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "field number must be positive"})
			}
			if _, ok := seenNames[field.Name]; ok {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "duplicate field name"})
			}
			if _, ok := seenNumbers[field.Number]; ok {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: fmt.Sprintf("duplicate field number %d", field.Number)})
			}
			seenNames[field.Name] = struct{}{}
			seenNumbers[field.Number] = struct{}{}
			if field.Kind == "message" && !knownExternalType(field.Type) {
				if _, ok := messages[field.Type]; !ok {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved message type " + field.Type})
				}
			}
			if field.Kind == "enum" {
				if _, ok := enums[field.Type]; !ok {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved enum type " + field.Type})
				}
			}
		}
	}

	httpRoutes := make(map[string]string)
	operationOwners := make(map[string]string)
	for _, service := range manifest.Services {
		servicePath := "service." + service.FullName
		typedService := service.Application != nil || service.Domain != ""
		if service.Application != nil {
			if !validPolicyKey(service.Domain) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: servicePath, Message: "typed application requires a file-level domain declaration"})
			}
			if !validPolicyKey(service.Application.Name) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: servicePath + ".application", Message: "application name must be a stable lowercase business key"})
			}
		} else if service.Domain != "" {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: servicePath, Message: "service in a typed domain must declare a typed application"})
		}

		seenMethods := make(map[string]struct{})
		for _, method := range service.Methods {
			path := servicePath + ".method." + method.Name
			if _, ok := seenMethods[method.Name]; ok {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "duplicate method name"})
			}
			seenMethods[method.Name] = struct{}{}
			requestMessage, requestKnown := messages[method.Request]
			responseMessage, responseKnown := messages[method.Response]
			if !requestKnown && !knownExternalType(method.Request) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved request type " + method.Request})
			}
			if !responseKnown && !knownExternalType(method.Response) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved response type " + method.Response})
			}
			if typedService {
				if method.Operation == nil {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed application method must declare a typed operation"})
				}
				if requestKnown && requestMessage.DTO == nil {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed application request must be classified as a DTO"})
				}
				if responseKnown && responseMessage.DTO == nil {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed application response must be classified as a DTO"})
				}
			}

			if legacyHTTP, ok := directiveHTTPBinding(method.Directives); ok && len(method.HTTP) > 0 && bindingKey(legacyHTTP) != bindingKey(method.HTTP[0]) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "google.api.http binding conflicts with legacy @yunka.http declaration"})
			}

			if operation := method.Operation; operation != nil {
				if service.Application == nil || service.Domain == "" {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed operation requires typed domain and application declarations"})
				}
				if !validPolicyKey(operation.ID) {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "operation id must be a stable lowercase business key"})
				} else if owner, duplicate := operationOwners[operation.ID]; duplicate && owner != method.FullName {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "duplicate operation id also owned by " + owner})
				} else {
					operationOwners[operation.ID] = method.FullName
				}
				if !validPolicyKey(operation.UseCase) {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "use_case must be a stable lowercase business key"})
				}
				if operation.PermissionMode != "all" && operation.PermissionMode != "any" {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed permission mode must be all or any"})
				}
				for _, permission := range operation.Permissions {
					if !validPolicyKey(permission) {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "invalid permission key " + permission})
					}
				}
				legacyPolicy := authorizationFromDirectives(method.Directives)
				typedPolicy := authorizationFromOperation(operation)
				if legacyPolicy != nil && authorizationKey(legacyPolicy) != authorizationKey(typedPolicy) {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed operation conflicts with legacy @yunka authorization directives"})
				}
				if operation.Public {
					if len(operation.Permissions) > 0 || len(operation.Authentication) > 0 || operation.TenantRequired {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "public operation cannot declare permissions, authentication, or tenant requirement"})
					}
					if method.Authorization != nil {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "public typed operation must not normalize to an authorization policy"})
					}
				} else {
					if len(operation.Permissions) == 0 {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "protected typed operation requires at least one permission"})
					}
					if authorizationKey(typedPolicy) != authorizationKey(method.Authorization) {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "typed operation and normalized authorization policy differ"})
					}
				}
			} else if hasLegacyAuthzDirectives(method.Directives) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityWarning, Path: path, Message: "legacy @yunka authorization directives are read-only migration input; use typed protobuf operation options"})
			}

			if policy := method.Authorization; policy != nil {
				if strings.TrimSpace(policy.OperationID) == "" {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "authz operationId is required when authz policy is present"})
				}
				if policy.PermissionMode != "all" && policy.PermissionMode != "any" {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "authz permission_mode must be all or any"})
				}
				for _, permission := range policy.Permissions {
					if !validPolicyKey(permission) {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "invalid permission key " + permission})
					}
				}
			}
			for _, binding := range method.HTTP {
				if err := validateHTTPMethod(binding.Method); err != nil {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: err.Error()})
				}
				if !strings.HasPrefix(binding.Path, "/") {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "HTTP path must start with /"})
				}
				key := strings.ToUpper(binding.Method) + " " + binding.Path
				if owner, ok := httpRoutes[key]; ok && owner != method.FullName {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "duplicate HTTP binding with " + owner + ": " + key})
				} else {
					httpRoutes[key] = method.FullName
				}
			}
		}
	}

	diagnostics = append(diagnostics, lintComposition(manifest)...)

	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Severity == diagnostics[j].Severity {
			if diagnostics[i].Path == diagnostics[j].Path {
				return diagnostics[i].Message < diagnostics[j].Message
			}
			return diagnostics[i].Path < diagnostics[j].Path
		}
		return severityRank(diagnostics[i].Severity) < severityRank(diagnostics[j].Severity)
	})
	return diagnostics
}

func hasLegacyAuthzDirectives(directives map[string]string) bool {
	for _, key := range []string{"operation", "permission", "permission_mode", "tenant_required", "authentication"} {
		if strings.TrimSpace(directives[key]) != "" {
			return true
		}
	}
	return false
}

func HasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func knownExternalType(name string) bool {
	return strings.HasPrefix(name, "google.protobuf.")
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func validPolicyKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for i, current := range value {
		if current >= 'a' && current <= 'z' {
			continue
		}
		if current >= '0' && current <= '9' && i > 0 {
			continue
		}
		if (current == '.' || current == '_' || current == '-' || current == ':') && i > 0 {
			continue
		}
		return false
	}
	return true
}

type compositionOperationOwner struct {
	method         Method
	applicationKey string
}

func lintComposition(manifest Manifest) []Diagnostic {
	manifest.Normalize()
	applications := map[string]Service{}
	operations := map[string]compositionOperationOwner{}
	var diagnostics []Diagnostic
	for _, service := range manifest.Services {
		if service.Application == nil || service.Domain == "" {
			continue
		}
		key := service.Domain + "/" + service.Application.Name
		if owner, exists := applications[key]; exists && owner.FullName != service.FullName {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: "service." + service.FullName + ".application", Message: "duplicate application capability key also owned by " + owner.FullName + ": " + key})
		} else {
			applications[key] = service
		}
		for _, method := range service.Methods {
			if method.Operation == nil || method.Operation.ID == "" {
				continue
			}
			operations[method.Operation.ID] = compositionOperationOwner{method: method, applicationKey: key}
		}
	}

	appDeps := map[string][]string{}
	for key, service := range applications {
		declared := map[string]struct{}{}
		for _, dependency := range service.Application.Requires {
			path := "service." + service.FullName + ".application"
			if !validApplicationDependency(dependency) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "application dependency must be <domain>/<application>: " + dependency})
				continue
			}
			if dependency == key {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "application cannot depend on itself: " + dependency})
				continue
			}
			if _, ok := applications[dependency]; !ok {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unknown application capability dependency: " + dependency})
				continue
			}
			declared[dependency] = struct{}{}
			appDeps[key] = append(appDeps[key], dependency)
		}
		for _, method := range service.Methods {
			if method.Operation == nil {
				continue
			}
			opPath := "service." + service.FullName + ".method." + method.Name
			if method.Operation.Composition != "" && method.Operation.Composition != "local" && method.Operation.Composition != "remote_saga" {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: opPath, Message: "composition must be local or remote_saga"})
			}
			for _, required := range method.Operation.RequiresOperations {
				owner, ok := operations[required]
				if !ok {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: opPath, Message: "unknown required operation: " + required})
					continue
				}
				if required == method.Operation.ID {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: opPath, Message: "operation cannot depend on itself: " + required})
				}
				if owner.applicationKey != key {
					if _, ok := declared[owner.applicationKey]; !ok {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: opPath, Message: "required operation belongs to undeclared application capability: " + owner.applicationKey})
					}
				}
			}
		}
	}
	for _, cycle := range dependencyCycles(appDeps) {
		diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: "application." + cycle[0], Message: "application dependency cycle: " + strings.Join(cycle, " -> ")})
	}

	opDeps := map[string][]string{}
	for id, owner := range operations {
		opDeps[id] = append([]string(nil), owner.method.Operation.RequiresOperations...)
	}
	for _, cycle := range dependencyCycles(opDeps) {
		diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: "operation." + cycle[0], Message: "operation dependency cycle: " + strings.Join(cycle, " -> ")})
	}
	for id, owner := range operations {
		closure := map[string]struct{}{}
		collectRequiredPermissions(id, operations, map[string]bool{}, closure)
		declared := map[string]struct{}{}
		for _, permission := range owner.method.Operation.Permissions {
			declared[permission] = struct{}{}
		}
		missing := make([]string, 0)
		for permission := range closure {
			if _, ok := declared[permission]; !ok {
				missing = append(missing, permission)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: "operation." + id, Message: "composite permission closure missing: " + strings.Join(missing, ",")})
		}
	}
	return diagnostics
}

func validApplicationDependency(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "/")
	return len(parts) == 2 && validPolicyKey(parts[0]) && validPolicyKey(parts[1])
}

func dependencyCycles(graph map[string][]string) [][]string {
	state := map[string]uint8{}
	stack := []string{}
	position := map[string]int{}
	seen := map[string]struct{}{}
	var cycles [][]string
	var visit func(string)
	visit = func(node string) {
		if state[node] == 2 {
			return
		}
		if state[node] == 1 {
			start := position[node]
			cycle := append(append([]string(nil), stack[start:]...), node)
			key := strings.Join(cycle, "\x00")
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				cycles = append(cycles, cycle)
			}
			return
		}
		state[node] = 1
		position[node] = len(stack)
		stack = append(stack, node)
		deps := append([]string(nil), graph[node]...)
		sort.Strings(deps)
		for _, next := range deps {
			if _, exists := graph[next]; exists {
				visit(next)
			}
		}
		stack = stack[:len(stack)-1]
		delete(position, node)
		state[node] = 2
	}
	nodes := make([]string, 0, len(graph))
	for node := range graph {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		visit(node)
	}
	return cycles
}

func collectRequiredPermissions(id string, operations map[string]compositionOperationOwner, active map[string]bool, result map[string]struct{}) {
	if active[id] {
		return
	}
	active[id] = true
	owner, ok := operations[id]
	if !ok {
		delete(active, id)
		return
	}
	for _, required := range owner.method.Operation.RequiresOperations {
		target, ok := operations[required]
		if !ok {
			continue
		}
		for _, permission := range target.method.Operation.Permissions {
			result[permission] = struct{}{}
		}
		collectRequiredPermissions(required, operations, active, result)
	}
	delete(active, id)
}
