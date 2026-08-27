package contract

import "testing"

func TestCompareDetectsBreakingChanges(t *testing.T) {
	baseline := Manifest{
		SchemaVersion: ManifestVersion,
		Messages: []Message{{
			Name: "Request", FullName: "demo.Request",
			Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}},
		}},
		Services: []Service{{
			Name: "Demo", FullName: "demo.Demo",
			Methods: []Method{{Name: "Get", FullName: "demo.Demo.Get", Request: "demo.Request", Response: "demo.Request", HTTP: []HTTPBinding{{Method: "GET", Path: "/v1/demo/{id}"}}}},
		}},
	}
	current := baseline
	current.Messages = []Message{{
		Name: "Request", FullName: "demo.Request",
		Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "int32"}},
	}}
	current.Services = []Service{{
		Name: "Demo", FullName: "demo.Demo",
		Methods: []Method{{Name: "Get", FullName: "demo.Demo.Get", Request: "demo.Request", Response: "demo.Request", HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/demo/{id}"}}}},
	}}
	diff := Compare(baseline, current)
	if !diff.HasBreaking() {
		t.Fatalf("expected breaking diff: %#v", diff)
	}
	foundType, foundHTTP := false, false
	for _, change := range diff.Changes {
		foundType = foundType || change.Kind == "field_type_changed"
		foundHTTP = foundHTTP || change.Kind == "http_binding_removed"
	}
	if !foundType || !foundHTTP {
		t.Fatalf("missing expected changes: %#v", diff.Changes)
	}
}

func TestCompareTreatsOptionalFieldAdditionAsCompatible(t *testing.T) {
	baseline := Manifest{SchemaVersion: ManifestVersion, Messages: []Message{{Name: "Request", FullName: "Request"}}}
	current := Manifest{SchemaVersion: ManifestVersion, Messages: []Message{{Name: "Request", FullName: "Request", Fields: []Field{{Name: "name", JSONName: "name", Number: 1, Kind: "scalar", Type: "string"}}}}}
	diff := Compare(baseline, current)
	if diff.HasBreaking() {
		t.Fatalf("unexpected breaking diff: %#v", diff)
	}
	if len(diff.Changes) != 1 || diff.Changes[0].Kind != "field_added" {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestCompareGuardsC84DSLIdentitiesAndPolicy(t *testing.T) {
	baseline := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{Name: "device.proto", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{
			{Name: "GetRequest", FullName: "device.v1.GetRequest", DTO: &DTODeclaration{Kind: "input"}},
			{Name: "GetResponse", FullName: "device.v1.GetResponse", DTO: &DTODeclaration{Kind: "output"}},
		},
		Services: []Service{{
			Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "device_management"},
			Methods: []Method{{
				Name: "Get", FullName: "device.v1.DeviceApplication.Get", Request: "device.v1.GetRequest", Response: "device.v1.GetResponse",
				Operation: &OperationDeclaration{ID: "device.get", UseCase: "get", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
				Authorization: &AuthorizationPolicy{OperationID: "device.get", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}},
			}},
		}},
	}
	current := baseline
	current.Files = []File{{Name: "device.proto", Domain: &DomainDeclaration{Name: "asset", Version: "v2"}}}
	current.Messages = append([]Message(nil), baseline.Messages...)
	current.Messages[1].DTO = &DTODeclaration{Kind: "shared"}
	current.Services = []Service{{
		Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "asset", Application: &ApplicationDeclaration{Name: "asset_management"},
		Methods: []Method{{
			Name: "Get", FullName: "device.v1.DeviceApplication.Get", Request: "device.v1.GetRequest", Response: "device.v1.GetResponse",
			Operation: &OperationDeclaration{ID: "asset.get", UseCase: "lookup", Permissions: []string{"asset.read"}, PermissionMode: "any", TenantRequired: false, Authentication: []string{"api_key"}},
			Authorization: &AuthorizationPolicy{OperationID: "asset.get", Permissions: []string{"asset.read"}, PermissionMode: "any", Authentication: []string{"api_key"}},
		}},
	}}
	diff := Compare(baseline, current)
	if !diff.HasBreaking() {
		t.Fatalf("expected C8.4 identity/policy changes to be breaking: %#v", diff.Changes)
	}
	want := map[string]bool{
		"domain_identity_changed": false,
		"service_domain_changed": false,
		"application_identity_changed": false,
		"dto_kind_changed": false,
		"operation_id_changed": false,
		"use_case_changed": false,
		"permissions_changed": false,
		"permission_mode_changed": false,
		"authentication_changed": false,
		"tenant_requirement_changed": false,
	}
	for _, change := range diff.Changes {
		if _, ok := want[change.Kind]; ok {
			want[change.Kind] = true
		}
	}
	for kind, found := range want {
		if !found {
			t.Fatalf("missing C8.4 compatibility classification %s: %#v", kind, diff.Changes)
		}
	}
}

func TestCompareTreatsTypedDeclarationCutoverAsAdditiveWhenSemanticsStayStable(t *testing.T) {
	policy := &AuthorizationPolicy{OperationID: "device.get", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}}
	baseline := Manifest{
		SchemaVersion: 1,
		Files: []File{{Name: "device.proto"}},
		Messages: []Message{{Name: "Request", FullName: "device.Request"}, {Name: "Response", FullName: "device.Response"}},
		Services: []Service{{Name: "Device", FullName: "device.Device", Methods: []Method{{Name: "Get", FullName: "device.Device.Get", Request: "device.Request", Response: "device.Response", Authorization: policy}}}},
	}
	current := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{Name: "device.proto", Domain: &DomainDeclaration{Name: "device"}}},
		Messages: []Message{{Name: "Request", FullName: "device.Request", DTO: &DTODeclaration{Kind: "input"}}, {Name: "Response", FullName: "device.Response", DTO: &DTODeclaration{Kind: "output"}}},
		Services: []Service{{Name: "Device", FullName: "device.Device", Domain: "device", Application: &ApplicationDeclaration{Name: "device"}, Methods: []Method{{Name: "Get", FullName: "device.Device.Get", Request: "device.Request", Response: "device.Response", Operation: &OperationDeclaration{ID: "device.get", UseCase: "get", Permissions: []string{"device.read"}, PermissionMode: "all", TenantRequired: true, Authentication: []string{"jwt"}}, Authorization: policy}}}},
	}
	if diff := Compare(baseline, current); diff.HasBreaking() {
		t.Fatalf("typed metadata cutover should be additive when wire/policy semantics stay stable: %#v", diff.Changes)
	}
}
