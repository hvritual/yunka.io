package contract

import (
	"strings"
	"testing"
)

func TestExternalContractProjectionExcludesInternalOnlyOperationTypes(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Messages: []Message{
			{FullName: "external.Request", Fields: []Field{
				{Name: "shared", JSONName: "shared", Kind: "message", Type: "external.Shared"},
				{Name: "mode", JSONName: "mode", Kind: "enum", Type: "external.Mode"},
			}},
			{FullName: "external.Shared", Fields: []Field{{Name: "value", JSONName: "value", Kind: "scalar", Type: "string"}}},
			{FullName: "external.Response"},
			{FullName: "internal.Request"},
			{FullName: "internal.Response"},
		},
		Enums: []Enum{
			{FullName: "external.Mode", Values: []EnumValue{{Name: "MODE_UNSPECIFIED", Number: 0}}},
			{FullName: "internal.Mode", Values: []EnumValue{{Name: "INTERNAL_UNSPECIFIED", Number: 0}}},
		},
		Services: []Service{
			{
				Name:     "ExternalAPI",
				FullName: "external.ExternalAPI",
				Methods: []Method{{
					Name: "Do", FullName: "external.ExternalAPI.Do",
					Request: "external.Request", Response: "external.Response",
				}},
			},
			{
				Name:     "InternalApplication",
				FullName: "internal.InternalApplication",
				Domain:   "internal",
				Application: &ApplicationDeclaration{
					Name: "internal_application",
					Operations: []OperationDeclaration{{
						ID: "internal.do", UseCase: "do_internal",
						RequestType: "internal.Request", ResponseType: "internal.Response", ApplicationMethod: "DoInternal",
					}},
				},
			},
		},
	}

	openapi, err := GenerateOpenAPI(manifest, OpenAPIOptions{})
	if err != nil {
		t.Fatal(err)
	}
	openapiText := string(openapi)
	for _, required := range []string{"external_Request", "external_Shared", "external_Response", "external_Mode"} {
		if !strings.Contains(openapiText, required) {
			t.Fatalf("OpenAPI missing transport-reachable type %q", required)
		}
	}
	for _, forbidden := range []string{"internal_Request", "internal_Response", "internal_Mode", "internal.do"} {
		if strings.Contains(openapiText, forbidden) {
			t.Fatalf("OpenAPI leaked internal-only contract %q", forbidden)
		}
	}

	typescript, err := GenerateTypeScript(manifest, TypeScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	typescriptText := string(typescript)
	for _, required := range []string{"External_Request", "External_Shared", "External_Response", "External_Mode"} {
		if !strings.Contains(typescriptText, required) {
			t.Fatalf("TypeScript missing transport-reachable type %q", required)
		}
	}
	for _, forbidden := range []string{"Internal_Request", "Internal_Response", "Internal_Mode", "InternalApplicationClient", "internal.do"} {
		if strings.Contains(typescriptText, forbidden) {
			t.Fatalf("TypeScript leaked internal-only contract %q", forbidden)
		}
	}
}
