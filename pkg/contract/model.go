package contract

import (
	"sort"
	"strings"
)

const ManifestVersion = 2

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Files         []File    `json:"files"`
	Messages      []Message `json:"messages"`
	Enums         []Enum    `json:"enums"`
	Services      []Service `json:"services"`
}

type File struct {
	Name      string             `json:"name"`
	Package   string             `json:"package,omitempty"`
	Syntax    string             `json:"syntax,omitempty"`
	GoPackage string             `json:"goPackage,omitempty"`
	Domain    *DomainDeclaration `json:"domain,omitempty"`
}

type DomainDeclaration struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type DTODeclaration struct {
	Kind string `json:"kind"`
}

type ApplicationDeclaration struct {
	Name     string   `json:"name"`
	Requires []string `json:"requires,omitempty"`
}

type ExecutionPolicy struct {
	Transaction string `json:"transaction,omitempty"`
	Idempotency string `json:"idempotency,omitempty"`
}

type OperationDeclaration struct {
	ID                 string           `json:"id"`
	UseCase            string           `json:"useCase"`
	Permissions        []string         `json:"permissions,omitempty"`
	PermissionMode     string           `json:"permissionMode,omitempty"`
	TenantRequired     bool             `json:"tenantRequired,omitempty"`
	Authentication     []string         `json:"authentication,omitempty"`
	Public             bool             `json:"public,omitempty"`
	RequiresOperations []string         `json:"requiresOperations,omitempty"`
	Composition        string           `json:"composition,omitempty"`
	Execution          *ExecutionPolicy `json:"execution,omitempty"`
}

type Message struct {
	Name     string          `json:"name"`
	FullName string          `json:"fullName"`
	Fields   []Field         `json:"fields"`
	DTO      *DTODeclaration `json:"dto,omitempty"`
}

type Field struct {
	Name         string `json:"name"`
	JSONName     string `json:"jsonName"`
	Number       int32  `json:"number"`
	Kind         string `json:"kind"`
	Type         string `json:"type"`
	Repeated     bool   `json:"repeated,omitempty"`
	Required     bool   `json:"required,omitempty"`
	Optional     bool   `json:"optional,omitempty"`
	Map          bool   `json:"map,omitempty"`
	MapKeyType   string `json:"mapKeyType,omitempty"`
	MapValueKind string `json:"mapValueKind,omitempty"`
	MapValueType string `json:"mapValueType,omitempty"`
}

type Enum struct {
	Name     string      `json:"name"`
	FullName string      `json:"fullName"`
	Values   []EnumValue `json:"values"`
}

type EnumValue struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

type Service struct {
	Name        string                  `json:"name"`
	FullName    string                  `json:"fullName"`
	Domain      string                  `json:"domain,omitempty"`
	Application *ApplicationDeclaration `json:"application,omitempty"`
	Methods     []Method                `json:"methods"`
}

type Method struct {
	Name            string                `json:"name"`
	FullName        string                `json:"fullName"`
	Request         string                `json:"request"`
	Response        string                `json:"response"`
	ClientStreaming bool                  `json:"clientStreaming,omitempty"`
	ServerStreaming bool                  `json:"serverStreaming,omitempty"`
	HTTP            []HTTPBinding         `json:"http,omitempty"`
	Directives      map[string]string     `json:"directives,omitempty"`
	Operation       *OperationDeclaration `json:"operation,omitempty"`
	Authorization   *AuthorizationPolicy  `json:"authorization,omitempty"`
}

type AuthorizationPolicy struct {
	OperationID    string   `json:"operationId"`
	Permissions    []string `json:"permissions,omitempty"`
	PermissionMode string   `json:"permissionMode,omitempty"`
	TenantRequired bool     `json:"tenantRequired,omitempty"`
	Authentication []string `json:"authentication,omitempty"`
}

type HTTPBinding struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Body         string `json:"body,omitempty"`
	ResponseBody string `json:"responseBody,omitempty"`
}

func (manifest *Manifest) Normalize() {
	if manifest.SchemaVersion == 0 || manifest.SchemaVersion == 1 {
		manifest.SchemaVersion = ManifestVersion
	}
	for i := range manifest.Files {
		if manifest.Files[i].Domain != nil {
			manifest.Files[i].Domain.Name = strings.TrimSpace(manifest.Files[i].Domain.Name)
			manifest.Files[i].Domain.Version = strings.TrimSpace(manifest.Files[i].Domain.Version)
		}
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Name < manifest.Files[j].Name })
	for i := range manifest.Messages {
		sort.Slice(manifest.Messages[i].Fields, func(a, b int) bool {
			if manifest.Messages[i].Fields[a].Number == manifest.Messages[i].Fields[b].Number {
				return manifest.Messages[i].Fields[a].Name < manifest.Messages[i].Fields[b].Name
			}
			return manifest.Messages[i].Fields[a].Number < manifest.Messages[i].Fields[b].Number
		})
	}
	sort.Slice(manifest.Messages, func(i, j int) bool { return manifest.Messages[i].FullName < manifest.Messages[j].FullName })
	for i := range manifest.Enums {
		sort.Slice(manifest.Enums[i].Values, func(a, b int) bool {
			if manifest.Enums[i].Values[a].Number == manifest.Enums[i].Values[b].Number {
				return manifest.Enums[i].Values[a].Name < manifest.Enums[i].Values[b].Name
			}
			return manifest.Enums[i].Values[a].Number < manifest.Enums[i].Values[b].Number
		})
	}
	sort.Slice(manifest.Enums, func(i, j int) bool { return manifest.Enums[i].FullName < manifest.Enums[j].FullName })
	for i := range manifest.Services {
		manifest.Services[i].Domain = strings.TrimSpace(manifest.Services[i].Domain)
		if manifest.Services[i].Application != nil {
			manifest.Services[i].Application.Name = strings.TrimSpace(manifest.Services[i].Application.Name)
			manifest.Services[i].Application.Requires = stableStrings(manifest.Services[i].Application.Requires)
		}
		for j := range manifest.Services[i].Methods {
			method := &manifest.Services[i].Methods[j]
			sort.Slice(method.HTTP, func(a, b int) bool {
				left := method.HTTP[a]
				right := method.HTTP[b]
				if left.Method == right.Method {
					return left.Path < right.Path
				}
				return left.Method < right.Method
			})
			if method.Operation != nil {
				method.Operation.ID = strings.TrimSpace(method.Operation.ID)
				method.Operation.UseCase = strings.TrimSpace(method.Operation.UseCase)
				method.Operation.PermissionMode = strings.TrimSpace(method.Operation.PermissionMode)
				method.Operation.Permissions = stableStrings(method.Operation.Permissions)
				method.Operation.Authentication = stableStrings(method.Operation.Authentication)
				method.Operation.RequiresOperations = stableStrings(method.Operation.RequiresOperations)
				method.Operation.Composition = strings.TrimSpace(method.Operation.Composition)
				if method.Operation.Execution != nil {
					method.Operation.Execution.Transaction = strings.TrimSpace(method.Operation.Execution.Transaction)
					method.Operation.Execution.Idempotency = strings.TrimSpace(method.Operation.Execution.Idempotency)
				}
			}
			if method.Authorization != nil {
				method.Authorization.Permissions = stableStrings(method.Authorization.Permissions)
				method.Authorization.Authentication = stableStrings(method.Authorization.Authentication)
			}
		}
		sort.Slice(manifest.Services[i].Methods, func(a, b int) bool {
			return manifest.Services[i].Methods[a].Name < manifest.Services[i].Methods[b].Name
		})
	}
	sort.Slice(manifest.Services, func(i, j int) bool { return manifest.Services[i].FullName < manifest.Services[j].FullName })
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func fullName(pkg, parent, name string) string {
	parts := make([]string, 0, 3)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if parent != "" {
		parts = append(parts, parent)
	}
	if name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, ".")
}

func normalizeTypeName(name string) string {
	return strings.TrimPrefix(name, ".")
}
