package contract

import (
	"strings"
	"testing"
)

func TestRenderC9RESTCustomVerbUsesCanonicalRegistrar(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{
			Name: "resource.proto", Package: "resource.v1", GoPackage: "example.com/biz/contracts/resource/v1;resourcev1",
			Domain: &DomainDeclaration{Name: "resource", Version: "v1"},
		}},
		Messages: []Message{
			{Name: "RevokeResourceRequest", FullName: "resource.v1.RevokeResourceRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "ResourceDTO", FullName: "resource.v1.ResourceDTO"},
		},
		Services: []Service{{
			Name: "ResourceApplication", FullName: "resource.v1.ResourceApplication", Domain: "resource",
			Application: &ApplicationDeclaration{Name: "management"},
			Methods: []Method{{
				Name: "RevokeResource", FullName: "resource.v1.ResourceApplication.RevokeResource",
				Request: "resource.v1.RevokeResourceRequest", Response: "resource.v1.ResourceDTO",
				HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/resources/{id}:revoke"}},
				Operation: &OperationDeclaration{ID: "resource.revoke", UseCase: "revoke_resource", PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "none"}},
			}},
		}},
	}

	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	var rest string
	for _, file := range files {
		if file.Path == "resource/transport/rest/zz_yunka_management_operation_executor_gen.go" {
			rest = string(file.Content)
			break
		}
	}
	if rest == "" {
		t.Fatal("generated REST adapter missing")
	}
	if !strings.Contains(rest, `httpbinding.Register(mux, "POST", "/v1/resources/{id}:revoke", handler.handleOperationRevokeResource)`) {
		t.Fatalf("custom verb binding did not use canonical registrar:\n%s", rest)
	}
	if !strings.Contains(rest, `httpbinding "github.com/hvritual/yunka.io/gateway/httpbinding"`) {
		t.Fatalf("canonical registrar import missing:\n%s", rest)
	}
	if strings.Contains(rest, `mux.HandleFunc("POST /v1/resources/{id}:revoke"`) {
		t.Fatalf("raw ServeMux custom verb registration leaked into generated code:\n%s", rest)
	}
}

func TestRenderC9RESTRejectsPartialWildcardTemplate(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{
			Name: "resource.proto", Package: "resource.v1", GoPackage: "example.com/biz/contracts/resource/v1;resourcev1",
			Domain: &DomainDeclaration{Name: "resource", Version: "v1"},
		}},
		Messages: []Message{
			{Name: "RevokeResourceRequest", FullName: "resource.v1.RevokeResourceRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "ResourceDTO", FullName: "resource.v1.ResourceDTO"},
		},
		Services: []Service{{
			Name: "ResourceApplication", FullName: "resource.v1.ResourceApplication", Domain: "resource",
			Application: &ApplicationDeclaration{Name: "management"},
			Methods: []Method{{
				Name: "RevokeResource", FullName: "resource.v1.ResourceApplication.RevokeResource",
				Request: "resource.v1.RevokeResourceRequest", Response: "resource.v1.ResourceDTO",
				HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/resources/prefix-{id}:revoke"}},
				Operation: &OperationDeclaration{ID: "resource.revoke", UseCase: "revoke_resource", PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "none"}},
			}},
		}},
	}

	_, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err == nil || !strings.Contains(err.Error(), "requires handwritten routing") {
		t.Fatalf("expected fail-closed partial wildcard diagnostic, got %v", err)
	}
}
