package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func httpBindingManifest(fields ...Field) Manifest {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "echo.proto", Package: "echo.v1", GoPackage: "example.com/echo/contracts;echov1", Domain: &DomainDeclaration{Name: "echo"}}},
		Messages: []Message{
			{Name: "Request", FullName: "echo.v1.Request", DTO: &DTODeclaration{Kind: "input"}, Fields: fields},
			{Name: "Response", FullName: "echo.v1.Response", DTO: &DTODeclaration{Kind: "output"}},
		},
		Services: []Service{{Name: "EchoApplication", FullName: "echo.v1.EchoApplication", Domain: "echo", Application: &ApplicationDeclaration{Name: "echo"}, Methods: []Method{{
			Name: "Read", FullName: "echo.v1.EchoApplication.Read", Request: "echo.v1.Request", Response: "echo.v1.Response",
			HTTP:      []HTTPBinding{{Method: "GET", Path: "/v1/echo/{tenant_id}"}},
			Operation: &OperationDeclaration{ID: "echo.read", UseCase: "read", PermissionMode: "all", Public: true},
		}}}},
	}
	manifest.Normalize()
	return manifest
}

func httpTenantField() Field {
	return Field{Name: "tenant_id", JSONName: "tenantId", Number: 1, Kind: "scalar", Type: "string"}
}

func TestHTTPBindingPlanPartitionsPathQueryAndBody(t *testing.T) {
	codes := Field{Name: "capability_codes", JSONName: "capabilityCodes", Number: 2, Kind: "scalar", Type: "string", Repeated: true}
	manifest := httpBindingManifest(httpTenantField(), codes)
	method := manifest.Services[0].Methods[0]
	get, err := compileHTTPBindingPlan(method, method.HTTP[0], messageIndex(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if len(get.Path) != 1 || get.Path[0].Name != "tenant_id" || !reflect.DeepEqual(get.Query, []Field{codes}) {
		t.Fatalf("incorrect parameter ownership: %#v", get)
	}
	binding := HTTPBinding{Method: "POST", Path: "/v1/echo/{tenant_id}", Body: "*"}
	body, err := compileHTTPBindingPlan(method, binding, messageIndex(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Query) != 0 || len(body.Path) != 1 || body.Body != "*" {
		t.Fatalf("body ownership: %#v", body)
	}
}

func TestHTTPBindingRejectsUnsupportedShapesAcrossAllTypedProjections(t *testing.T) {
	for _, sample := range []Field{
		{Name: "filter", Kind: "message", Type: "echo.v1.Response"},
		{Name: "filters", Kind: "message", Type: "echo.v1.Response", Repeated: true},
		{Name: "lookup", Kind: "map", Type: "map", Map: true, MapKeyType: "string", MapValueKind: "scalar", MapValueType: "string"},
		{Name: "state", Kind: "enum", Type: "echo.v1.State"},
		{Name: "selection", Kind: "scalar", Type: "string", Oneof: true},
	} {
		t.Run(sample.Name, func(t *testing.T) {
			sample.Number, sample.JSONName = 2, sample.Name
			manifest := httpBindingManifest(httpTenantField(), sample)
			manifest.Enums = []Enum{{Name: "State", FullName: "echo.v1.State", Values: []EnumValue{{Name: "STATE_UNSPECIFIED", Number: 0}}}}
			method := manifest.Services[0].Methods[0]
			assertUnsupported := func(label string, err error) {
				t.Helper()
				if err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_HTTP_BINDING") || !strings.Contains(err.Error(), sample.Name) || !strings.Contains(err.Error(), "echo.read") {
					t.Fatalf("%s: missing field/operation diagnostic: %v", label, err)
				}
			}
			_, err := compileHTTPBindingPlan(method, method.HTTP[0], messageIndex(manifest))
			assertUnsupported("plan", err)
			_, err = GenerateOpenAPI(manifest, OpenAPIOptions{})
			assertUnsupported("openapi", err)
			_, err = GenerateTypeScript(manifest, TypeScriptOptions{})
			assertUnsupported("typescript", err)
			_, err = RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/echo/internal"})
			assertUnsupported("runtime", err)
			var bindingError bool
			for _, diagnostic := range Lint(manifest) {
				bindingError = bindingError || (diagnostic.Severity == SeverityError && strings.Contains(diagnostic.Message, "UNSUPPORTED_HTTP_BINDING"))
			}
			if !bindingError {
				t.Fatal("lint accepted unsupported URL input")
			}
			// The same shape remains legal in the existing ProtoJSON whole body.
			manifest.Services[0].Methods[0].HTTP = []HTTPBinding{{Method: "POST", Path: "/v1/echo/{tenant_id}", Body: "*"}}
			if _, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/echo/internal"}); err != nil {
				t.Fatalf("body:* regressed: %v", err)
			}
		})
	}
}

func TestHTTPBindingRejectsAmbiguousAliasesAndInvalidPaths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fields  []Field
		binding HTTPBinding
	}{
		{"repeated path", []Field{{Name: "tenant_id", JSONName: "tenantId", Type: "string", Kind: "scalar", Number: 1, Repeated: true}}, HTTPBinding{Method: "GET", Path: "/v1/{tenant_id}"}},
		{"missing path", []Field{httpTenantField()}, HTTPBinding{Method: "GET", Path: "/v1/{absent}"}},
		{"same field twice", []Field{httpTenantField()}, HTTPBinding{Method: "GET", Path: "/v1/{tenant_id}/{tenantId}"}},
		{"alias collision", []Field{httpTenantField(), {Name: "other", JSONName: "tenant_id", Type: "string", Kind: "scalar", Number: 2}}, HTTPBinding{Method: "GET", Path: "/v1/{tenant_id}"}},
		{"named body", []Field{httpTenantField()}, HTTPBinding{Method: "POST", Path: "/v1/{tenant_id}", Body: "tenant_id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := httpBindingManifest(tc.fields...)
			method := m.Services[0].Methods[0]
			if _, err := compileHTTPBindingPlan(method, tc.binding, messageIndex(m)); err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_HTTP_BINDING") {
				t.Fatalf("missing rejection: %v", err)
			}
		})
	}
}

func TestHTTPBindingPublishedParametersMatchRuntimePlan(t *testing.T) {
	m := httpBindingManifest(httpTenantField(), Field{Name: "capability_codes", JSONName: "capabilityCodes", Number: 2, Kind: "scalar", Type: "string", Repeated: true})
	data, err := GenerateOpenAPI(m, OpenAPIOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	op := doc["paths"].(map[string]any)["/v1/echo/{tenant_id}"].(map[string]any)["get"].(map[string]any)
	params := op["parameters"].([]any)
	path, query := params[0].(map[string]any), params[1].(map[string]any)
	if path["name"] != "tenant_id" || query["name"] != "capabilityCodes" || query["style"] != "form" || query["explode"] != true || query["schema"].(map[string]any)["type"] != "array" {
		t.Fatalf("published parameters disagree: %s", data)
	}
	ts, err := GenerateTypeScript(m, TypeScriptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(ts, []byte(`capabilityCodes?: readonly string[]`)) || !bytes.Contains(ts, []byte(`{ field: "capability_codes", jsonName: "capabilityCodes", repeated: true }`)) {
		t.Fatalf("TS disagrees: %s", ts)
	}
	files, err := RenderC9ApplicationCode(m, ApplicationCodeOptions{RootImport: "example.com/echo/internal"})
	if err != nil {
		t.Fatal(err)
	}
	var rest string
	for _, f := range files {
		if strings.HasSuffix(f.Path, "transport/rest/zz_yunka_echo_operation_executor_gen.go") {
			rest = string(f.Content)
		}
	}
	for _, part := range []string{`query["capabilityCodes"]`, `query["capability_codes"]`, `append(wire.CapabilityCodes, value)`, `request.PathValue("tenant_id")`, `operation.ExecuteTyped`} {
		if !strings.Contains(rest, part) {
			t.Fatalf("runtime missing %q: %s", part, rest)
		}
	}
	again, err := RenderC9ApplicationCode(m, ApplicationCodeOptions{RootImport: "example.com/echo/internal"})
	if err != nil || !reflect.DeepEqual(files, again) {
		t.Fatalf("generation drift: %v", err)
	}
}

func TestHTTPBindingRetainsDescriptorOneofAndOptionalFacts(t *testing.T) {
	protoc := testProtoc(t)
	root := t.TempDir()
	text := `syntax = "proto3"; package fields; message Request { oneof choice { string key = 1; int32 index = 2; } optional string token = 3; }`
	if err := os.WriteFile(filepath.Join(root, "fields.proto"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(context.Background(), CompileOptions{Dir: root, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	fields := compiled.Manifest.Messages[0].Fields
	if !fields[0].Oneof || !fields[1].Oneof || fields[2].Oneof || !fields[2].Optional {
		t.Fatalf("descriptor facts lost: %#v", fields)
	}
	legacy := legacyManifestProjection(compiled.Manifest)
	if legacy.Messages[0].Fields[0].Oneof || !compiled.Manifest.Messages[0].Fields[0].Oneof {
		t.Fatal("legacy projection mutated compiler facts or changed legacy ABI")
	}
}
