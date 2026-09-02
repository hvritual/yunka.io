package contract

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestCompileOperationPlansDeterministicAndClosed(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion, Services: []Service{
		{
			Name: "DeviceQueryApplication", FullName: "device.v1.DeviceQueryApplication", Domain: "device",
			Application: &ApplicationDeclaration{Name: "query"},
			Methods: []Method{{
				Name: "GetDevice", FullName: "device.v1.DeviceQueryApplication.GetDevice", Request: "device.v1.GetDeviceRequest", Response: "device.v1.DeviceDTO",
				Operation: &OperationDeclaration{ID: "device.get", UseCase: "get_device", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
				HTTP:      []HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}},
			}},
		},
		{
			Name: "DeviceCommandApplication", FullName: "device.v1.DeviceCommandApplication", Domain: "device",
			Application: &ApplicationDeclaration{Name: "command", Requires: []string{"device/query"}},
			Methods: []Method{{
				Name: "UpdateDevice", FullName: "device.v1.DeviceCommandApplication.UpdateDevice", Request: "device.v1.UpdateDeviceRequest", Response: "device.v1.DeviceDTO",
				Operation: &OperationDeclaration{ID: "device.update", UseCase: "update_device", Permissions: []string{"device.read", "device.write"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}, RequiresOperations: []string{"device.get"}, Composition: "local"},
				HTTP:      []HTTPBinding{{Method: "PATCH", Path: "/v1/devices/{id}", Body: "*"}},
			}},
		},
	}}
	first, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := operationplan.CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := operationplan.CanonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("operation plans are not deterministic\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
	if len(first.Operations) != 2 {
		t.Fatalf("operations=%d want=2", len(first.Operations))
	}
	var update operationplan.Plan
	for _, item := range first.Operations {
		if item.OperationID == "device.update" {
			update = item
		}
	}
	if len(update.Composition.PermissionClosure) != 1 || update.Composition.PermissionClosure[0] != "device.read" {
		t.Fatalf("closure=%v", update.Composition.PermissionClosure)
	}
	if update.Bindings.RPC != "/device.v1.DeviceCommandApplication/UpdateDevice" {
		t.Fatalf("rpc binding=%q", update.Bindings.RPC)
	}
}

func TestCompileOperationPlansRejectsDuplicateAndUndeclaredCapability(t *testing.T) {
	duplicate := Manifest{SchemaVersion: ManifestVersion, Services: []Service{{
		Name: "A", FullName: "d.A", Domain: "d", Application: &ApplicationDeclaration{Name: "a"},
		Methods: []Method{
			{Name: "One", FullName: "d.A.One", Request: "d.OneRequest", Response: "d.OneResponse", Operation: &OperationDeclaration{ID: "same", UseCase: "one", PermissionMode: "all"}},
			{Name: "Two", FullName: "d.A.Two", Request: "d.TwoRequest", Response: "d.TwoResponse", Operation: &OperationDeclaration{ID: "same", UseCase: "two", PermissionMode: "all"}},
		},
	}}}
	if _, err := CompileOperationPlans(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate operation id") {
		t.Fatalf("duplicate err=%v", err)
	}

	undeclared := Manifest{SchemaVersion: ManifestVersion, Services: []Service{
		{Name: "A", FullName: "d.A", Domain: "d", Application: &ApplicationDeclaration{Name: "a"}, Methods: []Method{{Name: "One", FullName: "d.A.One", Request: "d.OneRequest", Response: "d.OneResponse", Operation: &OperationDeclaration{ID: "d.one", UseCase: "one", Permissions: []string{"d.one"}, PermissionMode: "all", RequiresOperations: []string{"d.two"}, Composition: "local"}}}},
		{Name: "B", FullName: "d.B", Domain: "d", Application: &ApplicationDeclaration{Name: "b"}, Methods: []Method{{Name: "Two", FullName: "d.B.Two", Request: "d.TwoRequest", Response: "d.TwoResponse", Operation: &OperationDeclaration{ID: "d.two", UseCase: "two", Permissions: []string{"d.two"}, PermissionMode: "all"}}}},
	}}
	if _, err := CompileOperationPlans(undeclared); err == nil || !strings.Contains(err.Error(), "undeclared application capability") {
		t.Fatalf("undeclared err=%v", err)
	}
}

func TestCompileOperationPlansCarriesExplicitExecutionPolicy(t *testing.T) {
	manifest := Manifest{Services: []Service{{
		Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device",
		Application: &ApplicationDeclaration{Name: "management"},
		Methods: []Method{{
			Name: "UpdateDevice", FullName: "device.v1.DeviceApplication.UpdateDevice",
			Request: "device.v1.UpdateDeviceRequest", Response: "device.v1.DeviceDTO",
			Operation: &OperationDeclaration{
				ID: "device.update", UseCase: "update_device", PermissionMode: "all",
				Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"},
			},
		}},
	}}}
	set, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Operations) != 1 {
		t.Fatalf("operations=%d", len(set.Operations))
	}
	got := set.Operations[0].Execution
	if got.Transaction != "local" || got.Idempotency != "required" {
		t.Fatalf("execution=%#v", got)
	}
}

func TestCompileOperationPlansIncludesApplicationLevelInternalOperation(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion, Services: []Service{{
		Name: "SiteApplication", FullName: "site.v1.SiteApplication", Domain: "site",
		Application: &ApplicationDeclaration{Name: "management", Operations: []OperationDeclaration{{
			ID: "site.validate", UseCase: "validate_site", Permissions: []string{"site.read"}, PermissionMode: "all",
			RequestType: "site.v1.ValidateRequest", ResponseType: "site.v1.SiteDTO", ApplicationMethod: "Validate",
			Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"},
		}}},
	}}}
	set, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Operations) != 1 {
		t.Fatalf("operations=%d", len(set.Operations))
	}
	got := set.Operations[0]
	if got.OperationID != "site.validate" || got.RequestType != "site.v1.ValidateRequest" || got.ResponseType != "site.v1.SiteDTO" || got.Bindings.RPC != "" || len(got.Bindings.HTTP) != 0 {
		t.Fatalf("internal plan=%#v", got)
	}
}
