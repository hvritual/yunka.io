package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedDomainOwnsUseCasePolicyLifecycle(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "device.go", `package persistence

type DevicePO struct {
	SiteID string `+"`gorm:\"column:site_id\"`"+`
	CreatedBy string `+"`gorm:\"column:created_by\"`"+`
	Name string `+"`gorm:\"column:name\"`"+`
}
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal"), NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(root, "internal", "device")
	application := mustReadPolicyGenerated(t, filepath.Join(domainRoot, "application", "zz_yunka_service_gen.go"))
	for _, expected := range []string{
		"type DeviceUseCases interface",
		"type DevicePolicy interface",
		"type DeviceRules interface",
		"type DeviceAccessPolicy struct",
		"WithDevicePolicy",
		"AuthorizeUpdate",
		"ListScope",
	} {
		if !strings.Contains(application, expected) {
			t.Fatalf("generated application missing %q", expected)
		}
	}
	repositories := mustReadPolicyGenerated(t, filepath.Join(domainRoot, "infrastructure", "persistence", "zz_yunka_repositories_gen.go"))
	for _, expected := range []string{"filter policy.Filter", "site_id IN ?", "created_by = ?", "strings.Join"} {
		if !strings.Contains(repositories, expected) {
			t.Fatalf("generated repository missing %q", expected)
		}
	}
	rest := mustReadPolicyGenerated(t, filepath.Join(domainRoot, "transport", "rest", "zz_yunka_rest_gen.go"))
	for _, expected := range []string{"type Middleware func(http.Handler) http.Handler", "policy.ErrUnauthorized", "http.StatusForbidden"} {
		if !strings.Contains(rest, expected) {
			t.Fatalf("generated REST missing %q", expected)
		}
	}
	wire := mustReadPolicyGenerated(t, filepath.Join(domainRoot, "wire", "zz_yunka_wiring_gen.go"))
	if !strings.Contains(wire, "options ...application.Option") || !strings.Contains(wire, "middleware ...rest.Middleware") {
		t.Fatalf("generated wire does not expose typed policy/middleware options:\n%s", wire)
	}
	if err := Check(filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedGRPCMapsPolicyErrors(t *testing.T) {
	spec := Spec{
		Version:      SpecVersion,
		Domain:       "device",
		TablePrefix:  "yk",
		TenantScoped: true,
		Objects:      []ObjectSpec{{Name: "device", GoName: "Device", File: "device.go", TableName: "yk_device_device"}},
		REST:         RESTSpec{Enabled: true, Prefix: "/v1"},
		RPC:          RPCSpec{Enabled: true},
	}
	bridge := multiPolicyGRPCBridgeTemplate(spec, "example.com/biz/internal/device")
	for _, expected := range []string{"codes.Unauthenticated", "codes.PermissionDenied", "ports.ErrNotFound", "rpcError(err)"} {
		if !strings.Contains(bridge, expected) {
			t.Fatalf("generated gRPC bridge missing %q", expected)
		}
	}
}

func TestObjectWithoutScopeColumnsRejectsScopedListAtPersistenceBoundary(t *testing.T) {
	spec := Spec{Version: SpecVersion, Domain: "catalog", TablePrefix: "yk", TenantScoped: true, Objects: []ObjectSpec{{Name: "item", GoName: "Item", File: "item.go", TableName: "yk_catalog_item"}}}
	contents := multiPolicyRepositoriesTemplate(spec, "example.com/biz/internal/catalog")
	if !strings.Contains(contents, "site scope is not supported by this object") || !strings.Contains(contents, "self scope is not supported by this object") {
		t.Fatal("repository must fail closed for unsupported scope dimensions")
	}
}

func mustReadPolicyGenerated(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
