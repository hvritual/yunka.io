package contract

import "testing"

func TestAuthorizationFromDirectives(t *testing.T) {
	policy := authorizationFromDirectives(map[string]string{
		"operation":       "device.machine.get",
		"permission":      "device.machine.read, device.telemetry.read",
		"permission_mode": "all",
		"tenant_required": "true",
		"authentication":  "jwt|service-token",
	})
	if policy == nil || policy.OperationID != "device.machine.get" || !policy.TenantRequired || len(policy.Permissions) != 2 || len(policy.Authentication) != 2 {
		t.Fatalf("policy=%+v", policy)
	}
}

func TestLintRejectsIncompleteAuthzPolicy(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion, Services: []Service{{Name: "S", FullName: "x.S", Methods: []Method{{Name: "Get", FullName: "x.S.Get", Request: "google.protobuf.Empty", Response: "google.protobuf.Empty", Authorization: &AuthorizationPolicy{PermissionMode: "all", Permissions: []string{"Device.Read"}}}}}}}
	diagnostics := Lint(manifest)
	if !HasErrors(diagnostics) {
		t.Fatalf("expected authz lint error: %+v", diagnostics)
	}
}
