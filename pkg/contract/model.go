package contract

import (
	"sort"
	"strings"
)

const ManifestVersion = 1

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Files         []File    `json:"files"`
	Messages      []Message `json:"messages"`
	Enums         []Enum    `json:"enums"`
	Services      []Service `json:"services"`
}

type File struct {
	Name      string `json:"name"`
	Package   string `json:"package,omitempty"`
	Syntax    string `json:"syntax,omitempty"`
	GoPackage string `json:"goPackage,omitempty"`
}

type Message struct {
	Name     string  `json:"name"`
	FullName string  `json:"fullName"`
	Fields   []Field `json:"fields"`
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
	Name     string   `json:"name"`
	FullName string   `json:"fullName"`
	Methods  []Method `json:"methods"`
}

type Method struct {
	Name            string            `json:"name"`
	FullName        string            `json:"fullName"`
	Request         string            `json:"request"`
	Response        string            `json:"response"`
	ClientStreaming bool              `json:"clientStreaming,omitempty"`
	ServerStreaming bool              `json:"serverStreaming,omitempty"`
	HTTP            []HTTPBinding     `json:"http,omitempty"`
	Directives      map[string]string `json:"directives,omitempty"`
}

type HTTPBinding struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Body         string `json:"body,omitempty"`
	ResponseBody string `json:"responseBody,omitempty"`
}

func (manifest *Manifest) Normalize() {
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = ManifestVersion
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
		for j := range manifest.Services[i].Methods {
			sort.Slice(manifest.Services[i].Methods[j].HTTP, func(a, b int) bool {
				left := manifest.Services[i].Methods[j].HTTP[a]
				right := manifest.Services[i].Methods[j].HTTP[b]
				if left.Method == right.Method {
					return left.Path < right.Path
				}
				return left.Method < right.Method
			})
		}
		sort.Slice(manifest.Services[i].Methods, func(a, b int) bool {
			return manifest.Services[i].Methods[a].Name < manifest.Services[i].Methods[b].Name
		})
	}
	sort.Slice(manifest.Services, func(i, j int) bool { return manifest.Services[i].FullName < manifest.Services[j].FullName })
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
