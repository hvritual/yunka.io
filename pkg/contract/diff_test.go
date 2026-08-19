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
