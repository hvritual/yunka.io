package boundarycore

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
)

func fixture() contract.Manifest {
	return contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files:         []contract.File{{Name: "service.proto", Package: "sales.v1", Dependencies: []string{"dto.proto"}, Domain: &contract.DomainDeclaration{Name: "sales"}}, {Name: "dto.proto", Package: "sales.v1"}},
		Messages: []contract.Message{
			{Name: "Request", FullName: "sales.v1.Request", SourceFile: "dto.proto", DTO: &contract.DTODeclaration{Kind: "input"}, Fields: []contract.Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}},
			{Name: "Reply", FullName: "sales.v1.Reply", SourceFile: "dto.proto", DTO: &contract.DTODeclaration{Kind: "output"}, Fields: []contract.Field{{Name: "state", JSONName: "state", Number: 1, Kind: "enum", Type: "sales.v1.State"}}},
		},
		Enums: []contract.Enum{{Name: "State", FullName: "sales.v1.State", SourceFile: "dto.proto", Values: []contract.EnumValue{{Name: "UNKNOWN", Number: 0}}}},
		Services: []contract.Service{{Name: "Orders", FullName: "sales.v1.Orders", SourceFile: "service.proto", Domain: "sales", Application: &contract.ApplicationDeclaration{Name: "orders"}, Methods: []contract.Method{
			{Name: "Get", FullName: "sales.v1.Orders.Get", SourceFile: "service.proto", Request: "sales.v1.Request", Response: "sales.v1.Reply", Operation: &contract.OperationDeclaration{ID: "sales.get", UseCase: "get", PermissionMode: "all", Public: true, Boundary: &contract.BoundaryIntent{Context: "sales.orders", Aggregate: "order"}}},
			{Name: "List", FullName: "sales.v1.Orders.List", SourceFile: "service.proto", Request: "sales.v1.Request", Response: "sales.v1.Reply", Operation: &contract.OperationDeclaration{ID: "sales.list", UseCase: "list", PermissionMode: "all", Public: true, Boundary: &contract.BoundaryIntent{Context: "sales.reporting", AggregateNotApplicableReason: "cross-aggregate projection"}}},
		}}},
	}
}
func mustInspect(t *testing.T, m contract.Manifest) Inspection {
	t.Helper()
	r, e := Inspect(m, "sales/orders")
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestIssue161FingerprintDeterministicAndDetached(t *testing.T) {
	m := fixture()
	m.Files[0], m.Files[1] = m.Files[1], m.Files[0]
	m.Services[0].Methods[0], m.Services[0].Methods[1] = m.Services[0].Methods[1], m.Services[0].Methods[0]
	before, _ := json.Marshal(m)
	first := mustInspect(t, m)
	after, _ := json.Marshal(m)
	if string(before) != string(after) {
		t.Fatal("inspection mutated input")
	}
	canonical := mustInspect(t, fixture())
	if !reflect.DeepEqual(first, canonical) {
		t.Fatal("ordering affects evidence")
	}
	if first.Authority != "read_only" || first.SchemaVersion != 1 || len(first.FingerprintDigest) != 64 || first.IntentCoverage.State != "declared" {
		t.Fatalf("unexpected inspection %#v", first)
	}
	first.Fingerprint.Operations[0].Boundary.Context = "changed"
	if m.Services[0].Methods[1].Operation.Boundary.Context != "sales.orders" {
		t.Fatal("output aliases source intent")
	}
	first.Fingerprint.Messages[0].Fields[0].Name = "changed"
	if m.Messages[0].Fields[0].Name != "id" {
		t.Fatal("output aliases source DTO")
	}
}

func TestIssue161FingerprintBindsRelationsAndCanonicalFacts(t *testing.T) {
	baseline := mustInspect(t, fixture())
	changes := map[string]func(*contract.Manifest){
		"intent_relationship": func(m *contract.Manifest) {
			ops := m.Services[0].Methods
			ops[0].Operation.Boundary, ops[1].Operation.Boundary = ops[1].Operation.Boundary, ops[0].Operation.Boundary
		},
		"dto_field": func(m *contract.Manifest) { m.Messages[0].Fields[0].Type = "int64" },
		"enum_value": func(m *contract.Manifest) {
			m.Enums[0].Values = append(m.Enums[0].Values, contract.EnumValue{Name: "READY", Number: 1})
		},
		"execution": func(m *contract.Manifest) {
			m.Services[0].Methods[0].Operation.Execution = &contract.ExecutionPolicy{Transaction: "read_only"}
		},
		"streaming": func(m *contract.Manifest) { m.Services[0].Methods[0].ServerStreaming = true },
		"capability": func(m *contract.Manifest) {
			m.Services[0].Application.Capabilities = []contract.CapabilityRequirement{{Name: "clock", Package: "example.com/clock", Type: "Clock"}}
		},
		"source_move": func(m *contract.Manifest) {
			m.Files[1].Name = "models/dto.proto"
			m.Files[0].Dependencies = []string{"models/dto.proto"}
			for i := range m.Messages {
				m.Messages[i].SourceFile = "models/dto.proto"
			}
			m.Enums[0].SourceFile = "models/dto.proto"
		},
	}
	for name, mutate := range changes {
		t.Run(name, func(t *testing.T) {
			m := fixture()
			mutate(&m)
			got := mustInspect(t, m)
			if got.FingerprintDigest == baseline.FingerprintDigest {
				t.Fatal("stale fingerprint")
			}
			if name == "intent_relationship" {
				if !reflect.DeepEqual(got.IntentCoverage.Contexts, baseline.IntentCoverage.Contexts) || !reflect.DeepEqual(got.IntentCoverage.Aggregates, baseline.IntentCoverage.Aggregates) {
					t.Fatal("fixture must preserve union summaries")
				}
				if got.OperationPlansDigest != baseline.OperationPlansDigest {
					t.Fatal("intent changed execution IR")
				}
			}
		})
	}
}

func TestIssue161FingerprintDependencyEvidence(t *testing.T) {
	m := fixture()
	s := m.Services[0]
	s.Name = "Lookup"
	s.FullName = "sales.v1.Lookup"
	s.SourceFile = "lookup.proto"
	s.Application = &contract.ApplicationDeclaration{Name: "lookup"}
	s.Methods = []contract.Method{{Name: "Find", FullName: "sales.v1.Lookup.Find", SourceFile: "lookup.proto", Request: "sales.v1.Request", Response: "sales.v1.Reply", Operation: &contract.OperationDeclaration{ID: "sales.find", UseCase: "find", Public: true, PermissionMode: "all"}}}
	m.Services = append(m.Services, s)
	m.Files = append(m.Files, contract.File{Name: "lookup.proto", Dependencies: []string{"dto.proto"}})
	m.Services[0].Application.Requires = []string{"sales/lookup"}
	m.Services[0].Methods[0].Operation.RequiresOperations = []string{"sales.find"}
	m.Services[0].Methods[0].Operation.Composition = "local"
	first := mustInspect(t, m)
	if len(first.Fingerprint.DependencyOperations) != 1 || first.Fingerprint.DependencyOperations[0].Plan.OperationID != "sales.find" {
		t.Fatal("dependency facts missing")
	}
	m.Services[1].Methods[0].Operation.Execution = &contract.ExecutionPolicy{Transaction: "read_only"}
	second := mustInspect(t, m)
	if first.FingerprintDigest == second.FingerprintDigest {
		t.Fatal("dependent execution change invisible")
	}
}

func TestIssue161FingerprintUnknownAndFailClosed(t *testing.T) {
	m := fixture()
	for i := range m.Services[0].Methods {
		m.Services[0].Methods[i].Operation.Boundary = nil
	}
	unknown := mustInspect(t, m)
	if unknown.IntentCoverage.State != "unknown" || len(unknown.IntentCoverage.UnknownOperations) != 2 || len(unknown.IntentCoverage.Contexts) != 0 {
		t.Fatal("guessed legacy intent")
	}
	encoded, _ := json.Marshal(unknown)
	for _, bad := range []string{`"outcome"`, `"safe_to_merge"`, `"boundaryDecision"`, `"mutationAuthorized":true`} {
		if strings.Contains(string(encoded), bad) {
			t.Fatal("inspection granted authority")
		}
	}
	m.Services[0].Methods[0].Operation.Boundary = &contract.BoundaryIntent{Context: "sales.orders", Aggregate: "order"}
	if mustInspect(t, m).IntentCoverage.State != "partial" {
		t.Fatal("unknown silently became declared")
	}
	m.Services[0].Methods = nil
	if mustInspect(t, m).IntentCoverage.State != "empty" {
		t.Fatal("empty service treated as declared")
	}
	for _, target := range []string{"", "sales", "sales/absent", "sales/orders/extra"} {
		if _, err := Inspect(fixture(), target); err == nil {
			t.Fatalf("accepted target %q", target)
		}
	}
	bad := fixture()
	bad.Services[0].Methods[0].Operation.Boundary = &contract.BoundaryIntent{Context: "sales.orders"}
	if _, err := Inspect(bad, "sales/orders"); err == nil {
		t.Fatal("ignored incomplete explicit intent")
	}
	bad = fixture()
	bad.Services = append(bad.Services, bad.Services[0])
	if _, err := Inspect(bad, "sales/orders"); err == nil {
		t.Fatal("ambiguous projection accepted")
	}
	bad = fixture()
	bad.Services[0].SourceFile = ""
	if _, err := Inspect(bad, "sales/orders"); err == nil {
		t.Fatal("missing source identity accepted")
	}
}
