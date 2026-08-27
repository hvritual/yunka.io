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
			if _, ok := messages[method.Request]; !ok && !knownExternalType(method.Request) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved request type " + method.Request})
			}
			if _, ok := messages[method.Response]; !ok && !knownExternalType(method.Response) {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "unresolved response type " + method.Response})
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
				if operation.Public {
					if len(operation.Permissions) > 0 || len(operation.Authentication) > 0 || operation.TenantRequired {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "public operation cannot declare permissions, authentication, or tenant requirement"})
					}
				} else {
					if len(operation.Permissions) == 0 {
						diagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path, Message: "protected typed operation requires at least one permission"})
					}
					expected := authorizationFromOperation(operation)
					if authorizationKey(expected) != authorizationKey(method.Authorization) {
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
