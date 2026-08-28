package operationplan

import (
	"bytes"
	"strings"
	"testing"
)

func TestCanonicalJSONDeterministic(t *testing.T) {
	set := Set{Operations: []Plan{
		{
			OperationID:  "device.update",
			Domain:       "device",
			Application:  "management",
			UseCase:      "update_device",
			RequestType:  "device.v1.UpdateDeviceRequest",
			ResponseType: "device.v1.DeviceDTO",
			Security:     Security{TenantRequired: true, Permissions: []string{"device.read", "device.write", "device.read"}, PermissionMode: "all", Authentication: []string{"jwt", "jwt"}},
			Composition:  Composition{Boundary: "local", RequiresOperations: []string{"device.get"}, PermissionClosure: []string{"device.read"}},
			Bindings:     Bindings{RPC: "/device.v1.DeviceApplication/UpdateDevice", HTTP: []HTTPBinding{{Method: "post", Path: "/v1/devices/{id}"}}},
		},
		{
			OperationID:  "device.get",
			Domain:       "device",
			Application:  "management",
			UseCase:      "get_device",
			RequestType:  "device.v1.GetDeviceRequest",
			ResponseType: "device.v1.DeviceDTO",
			Security:     Security{TenantRequired: true, Permissions: []string{"device.read"}, PermissionMode: "all", Authentication: []string{"jwt"}},
			Bindings:     Bindings{RPC: "/device.v1.DeviceApplication/GetDevice"},
		},
	}}
	first, err := CanonicalJSON(set)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalJSON(set)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("canonical output differs\nfirst=%s\nsecond=%s", first, second)
	}
	if !bytes.Contains(first, []byte(`"operationId": "device.get"`)) || bytes.Index(first, []byte(`"device.get"`)) > bytes.Index(first, []byte(`"device.update"`)) {
		t.Fatalf("operations are not sorted: %s", first)
	}
	if !bytes.Contains(first, []byte(`"method": "POST"`)) {
		t.Fatalf("HTTP method was not normalized: %s", first)
	}
	firstDigest, err := Digest(set)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := Digest(set)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || firstDigest == "" {
		t.Fatalf("digest mismatch first=%q second=%q", firstDigest, secondDigest)
	}
}

func TestValidateRejectsUnknownDependencyCycleAndPermissionClosure(t *testing.T) {
	base := Plan{
		OperationID:  "a",
		Domain:       "d",
		Application:  "app",
		UseCase:      "a",
		RequestType:  "d.ARequest",
		ResponseType: "d.AResponse",
		Security:     Security{Permissions: []string{"a.read"}, PermissionMode: "all"},
		Bindings:     Bindings{RPC: "/d.App/A"},
	}

	unknown := Set{Operations: []Plan{base}}
	unknown.Operations[0].Composition.RequiresOperations = []string{"missing"}
	if err := Validate(unknown); err == nil || !strings.Contains(err.Error(), "unknown operation") {
		t.Fatalf("unknown dependency err=%v", err)
	}

	second := base
	second.OperationID = "b"
	second.UseCase = "b"
	second.Bindings.RPC = "/d.App/B"
	second.Security.Permissions = []string{"b.read"}
	cycle := Set{Operations: []Plan{base, second}}
	cycle.Operations[0].Composition.RequiresOperations = []string{"b"}
	cycle.Operations[0].Composition.PermissionClosure = []string{"b.read"}
	cycle.Operations[0].Security.Permissions = []string{"a.read", "b.read"}
	cycle.Operations[1].Composition.RequiresOperations = []string{"a"}
	cycle.Operations[1].Composition.PermissionClosure = []string{"a.read", "b.read"}
	cycle.Operations[1].Security.Permissions = []string{"a.read", "b.read"}
	if err := Validate(cycle); err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("cycle err=%v", err)
	}

	closure := Set{Operations: []Plan{base, second}}
	closure.Operations[0].Composition.RequiresOperations = []string{"b"}
	closure.Operations[0].Composition.PermissionClosure = []string{"b.read"}
	if err := Validate(closure); err == nil || !strings.Contains(err.Error(), "permission closure missing") {
		t.Fatalf("closure err=%v", err)
	}
}

func TestNormalizeUpgradesV1ExecutionDefaultsWithoutMutatingInput(t *testing.T) {
	originalHTTP := []HTTPBinding{{Method: "post", Path: " /v1/a "}}
	input := Set{SchemaVersion: 1, Operations: []Plan{{
		OperationID: "a", Domain: "d", Application: "app", UseCase: "a",
		RequestType: "d.ARequest", ResponseType: "d.AResponse",
		Security: Security{PermissionMode: "all"},
		Bindings: Bindings{RPC: "/d.App/A", HTTP: originalHTTP},
	}}}
	normalized := Normalize(input)
	if normalized.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion=%d want %d", normalized.SchemaVersion, SchemaVersion)
	}
	if got := normalized.Operations[0].Execution; got.Transaction != "none" || got.Idempotency != "none" {
		t.Fatalf("execution=%#v", got)
	}
	if input.Operations[0].Bindings.HTTP[0].Method != "post" || input.Operations[0].Bindings.HTTP[0].Path != " /v1/a " {
		t.Fatalf("Normalize mutated caller input: %#v", input.Operations[0].Bindings.HTTP)
	}
	if normalized.Operations[0].Bindings.HTTP[0].Method != "POST" || normalized.Operations[0].Bindings.HTTP[0].Path != "/v1/a" {
		t.Fatalf("normalized HTTP=%#v", normalized.Operations[0].Bindings.HTTP)
	}
}

func TestValidateRejectsUnknownExecutionPolicy(t *testing.T) {
	plan := Plan{
		OperationID: "a", Domain: "d", Application: "app", UseCase: "a",
		RequestType: "d.ARequest", ResponseType: "d.AResponse",
		Security:  Security{PermissionMode: "all"},
		Execution: Execution{Transaction: "magic", Idempotency: "none"},
		Bindings:  Bindings{RPC: "/d.App/A"},
	}
	if err := Validate(Set{Operations: []Plan{plan}}); err == nil || !strings.Contains(err.Error(), "invalid transaction policy") {
		t.Fatalf("transaction policy err=%v", err)
	}
	plan.Execution = Execution{Transaction: "none", Idempotency: "magic"}
	if err := Validate(Set{Operations: []Plan{plan}}); err == nil || !strings.Contains(err.Error(), "invalid idempotency policy") {
		t.Fatalf("idempotency policy err=%v", err)
	}
}
