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
	messages := make(map[string]Message, len(manifest.Messages))
	enums := make(map[string]Enum, len(manifest.Enums))
	for _, message := range manifest.Messages {
		messages[message.FullName] = message
	}
	for _, enum := range manifest.Enums {
		enums[enum.FullName] = enum
	}

	var diagnostics []Diagnostic
	for _, message := range manifest.Messages {
		seenNames := make(map[string]struct{})
		seenNumbers := make(map[int32]struct{})
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
	for _, service := range manifest.Services {
		seenMethods := make(map[string]struct{})
		for _, method := range service.Methods {
			path := "service." + service.FullName + ".method." + method.Name
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
