package boundarycore

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
)

func additionFixture() (AdditionRequest, contract.Manifest, contract.Manifest) {
	before := fixture()
	before.Services[0].Methods = before.Services[0].Methods[:1]
	after := detachedManifest(before)
	// A second detached copy keeps the candidate from aliasing an existing declaration.
	m := detachedManifest(before).Services[0].Methods[0]
	m.Name, m.FullName = "Next", "sales.v1.Orders.Next"
	m.Operation.ID, m.Operation.UseCase = "sales.next", "next"
	after.Services[0].Methods = append(after.Services[0].Methods, m)
	return AdditionRequest{strings.Repeat("a", 40), "sales/orders", "sales.next"}, before, after
}
func evaluate(t *testing.T, r AdditionRequest, before, after contract.Manifest) ServiceBoundaryDecision {
	t.Helper()
	d, err := EvaluateAddition(r, before, after)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func dimension(t *testing.T, d ServiceBoundaryDecision, name string) BoundaryDimension {
	t.Helper()
	for _, value := range d.Dimensions {
		if value.Name == name {
			return value
		}
	}
	t.Fatalf("missing dimension %s", name)
	return BoundaryDimension{}
}

func TestIssue161DecisionReuseAndEvidence(t *testing.T) {
	r, before, after := additionFixture()
	d := evaluate(t, r, before, after)
	if d.Outcome != ReuseExistingApplication || d.Authority != "read_only" || !reflect.DeepEqual(d.PeerWitnesses, []string{"sales.get"}) {
		t.Fatalf("unexpected decision: %+v", d)
	}
	if len(d.DecisionDigest) != 64 || d.BeforeFingerprint == d.AfterFingerprint || d.BeforeManifestDigest == d.AfterManifestDigest {
		t.Fatal("unbound evidence")
	}
	for _, name := range []string{"lifecycle", "availability"} {
		v := dimension(t, d, name)
		if v.Verdict != "unknown" || v.Critical {
			t.Fatal("invented observed evidence")
		}
	}
	refs := map[string]bool{}
	for _, ref := range d.Evidence {
		if len(ref.Digest) != 64 || refs[ref.ID] {
			t.Fatal("invalid reference")
		}
		refs[ref.ID] = true
	}
	for _, dim := range d.Dimensions {
		for _, id := range append(append([]string{}, dim.Evidence...), dim.CounterEvidence...) {
			if !refs[id] {
				t.Fatalf("unknown evidence %s", id)
			}
		}
	}
	if err := RevalidateAddition(r, before, after, d); err != nil {
		t.Fatal(err)
	}
}

func TestIssue161DecisionContradictionFirstAndUnknown(t *testing.T) {
	tests := []struct {
		name string
		edit func(*contract.Manifest, *contract.Manifest)
		want string
	}{
		{"candidate_unknown", func(b, a *contract.Manifest) { a.Services[0].Methods[1].Operation.Boundary = nil }, ArchitectureReviewRequired},
		{"legacy_unknown", func(b, a *contract.Manifest) {
			b.Services[0].Methods[0].Operation.Boundary = nil
			a.Services[0].Methods[0].Operation.Boundary = nil
		}, ArchitectureReviewRequired},
		{"context_contradiction", func(b, a *contract.Manifest) {
			a.Services[0].Methods[1].Operation.Boundary.Context = "fulfilment.shipping"
		}, CreateNewApplication},
		{"aggregate_difference", func(b, a *contract.Manifest) { a.Services[0].Methods[1].Operation.Boundary.Aggregate = "invoice" }, ArchitectureReviewRequired},
		{"transaction_difference", func(b, a *contract.Manifest) {
			a.Services[0].Methods[1].Operation.Execution = &contract.ExecutionPolicy{Transaction: "read_only"}
		}, ArchitectureReviewRequired},
		{"hard_contradiction_with_unknown", func(b, a *contract.Manifest) {
			m := detachedManifest(*b).Services[0].Methods[0]
			m.Name = "Unknown"
			m.FullName = "sales.v1.Orders.Unknown"
			m.Operation.ID = "sales.unknown"
			m.Operation.UseCase = "unknown"
			m.Operation.Boundary = nil
			b.Services[0].Methods = append(b.Services[0].Methods, m)
			a.Services[0].Methods = append(a.Services[0].Methods, m)
			a.Services[0].Methods[1].Operation.Boundary.Context = "fulfilment.shipping"
		}, CreateNewApplication},
		{"existing_mixed_context", func(b, a *contract.Manifest) {
			m := detachedManifest(*b).Services[0].Methods[0]
			m.Name = "Report"
			m.FullName = "sales.v1.Orders.Report"
			m.Operation.ID = "sales.report"
			m.Operation.UseCase = "report"
			m.Operation.Boundary.Context = "sales.reporting"
			b.Services[0].Methods = append(b.Services[0].Methods, m)
			a.Services[0].Methods = append(a.Services[0].Methods, m)
		}, ArchitectureReviewRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, b, a := additionFixture()
			tt.edit(&b, &a)
			d := evaluate(t, r, b, a)
			if d.Outcome != tt.want {
				t.Fatalf("want %s got %+v", tt.want, d)
			}
			if len(d.CounterEvidence) == 0 {
				t.Fatal("counter-evidence omitted")
			}
			if err := RevalidateAddition(r, b, a, d); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIssue161DecisionRejectsMixedPeerWitnesses(t *testing.T) {
	r, b, _ := additionFixture()
	// A witnesses security but not execution; B witnesses execution but not
	// security. Every individual dimension matches, but no whole peer does.
	a := detachedManifest(b)
	p := a.Services[0].Methods[0]
	p.Operation.Execution = &contract.ExecutionPolicy{Transaction: "read_only"}
	b = detachedManifest(a)
	q := detachedManifest(b).Services[0].Methods[0]
	q.Name = "Private"
	q.FullName = "sales.v1.Orders.Private"
	q.Operation.ID = "sales.private"
	q.Operation.UseCase = "private"
	q.Operation.Public = false
	q.Operation.Authentication = []string{"authenticated"}
	q.Operation.Permissions = []string{"sales.read"}
	q.Authorization = &contract.AuthorizationPolicy{OperationID: q.Operation.ID, Permissions: []string{"sales.read"}, PermissionMode: "all", Authentication: []string{"authenticated"}}
	q.Operation.Execution = nil
	b.Services[0].Methods = append(b.Services[0].Methods, q)
	a = detachedManifest(b)
	n := detachedManifest(a).Services[0].Methods[0]
	n.Name = "Next"
	n.FullName = "sales.v1.Orders.Next"
	n.Operation.ID = r.OperationID
	n.Operation.UseCase = "next"
	n.Operation.Execution = nil
	a.Services[0].Methods = append(a.Services[0].Methods, n)
	d := evaluate(t, r, b, a)
	for _, name := range []string{"authorization", "consistency", "dependency", "client_contract"} {
		if dimension(t, d, name).Verdict != "same" {
			t.Fatalf("fixture lacks local match: %+v", d)
		}
	}
	if d.Outcome != ArchitectureReviewRequired || len(d.PeerWitnesses) != 0 {
		t.Fatalf("manufactured precedent: %+v", d)
	}
}

func TestIssue161DecisionAdditionScopeAndNewDTO(t *testing.T) {
	tests := []struct {
		name string
		edit func(*contract.Manifest)
		dim  string
	}{
		{"existing_dto", func(a *contract.Manifest) { a.Messages[0].Fields[0].Type = "int64" }, "change_scope"},
		{"existing_operation", func(a *contract.Manifest) {
			a.Services[0].Methods[0].Operation.Execution = &contract.ExecutionPolicy{Transaction: "read_only"}
		}, "change_scope"},
		{"capability", func(a *contract.Manifest) {
			a.Services[0].Application.Capabilities = []contract.CapabilityRequirement{{Name: "clock", Package: "example.com/clock", Type: "Clock"}}
		}, "change_scope"},
		{"unrelated_new_dto", func(a *contract.Manifest) {
			a.Messages = append(a.Messages, contract.Message{Name: "Other", FullName: "sales.v1.Other", SourceFile: "dto.proto"})
		}, "change_scope"},
		{"new_reachable_dto", func(a *contract.Manifest) {
			a.Messages = append(a.Messages, contract.Message{Name: "NewRequest", FullName: "sales.v1.NewRequest", SourceFile: "dto.proto", DTO: &contract.DTODeclaration{Kind: "input"}})
			a.Services[0].Methods[1].Request = "sales.v1.NewRequest"
		}, "client_contract"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, b, a := additionFixture()
			tt.edit(&a)
			d := evaluate(t, r, b, a)
			if d.Outcome != ArchitectureReviewRequired || dimension(t, d, tt.dim).Verdict != "different" {
				t.Fatalf("unexpected: %+v", d)
			}
			if tt.name == "new_reachable_dto" && dimension(t, d, "change_scope").Verdict != "same" {
				t.Fatal("new reachable DTO incorrectly rejected as out of scope")
			}
		})
	}
}

func TestIssue161DecisionProofRecomputedNotSelfAttested(t *testing.T) {
	r, b, a := additionFixture()
	original := evaluate(t, r, b, a)
	mutations := map[string]func(*ServiceBoundaryDecision){
		"base":        func(d *ServiceBoundaryDecision) { d.BaseSHA = strings.Repeat("b", 40) },
		"candidate":   func(d *ServiceBoundaryDecision) { d.OperationPlanDigest = strings.Repeat("0", 64) },
		"fingerprint": func(d *ServiceBoundaryDecision) { d.BeforeFingerprint = strings.Repeat("0", 64) },
		"policy":      func(d *ServiceBoundaryDecision) { d.PolicyVersion = "other/v1" },
		"authority":   func(d *ServiceBoundaryDecision) { d.Authority = "mutation_allowed" },
		"outcome":     func(d *ServiceBoundaryDecision) { d.Outcome = CreateNewApplication },
		"dimension":   func(d *ServiceBoundaryDecision) { d.Dimensions[0].Critical = false },
		"evidence":    func(d *ServiceBoundaryDecision) { d.Evidence[0].Digest = strings.Repeat("0", 64) },
		"unknown_evidence": func(d *ServiceBoundaryDecision) {
			d.Dimensions[0].Evidence = append(d.Dimensions[0].Evidence, "invented")
		},
		"witness": func(d *ServiceBoundaryDecision) { d.PeerWitnesses = []string{"invented"} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var proof ServiceBoundaryDecision
			data, _ := json.Marshal(original)
			_ = json.Unmarshal(data, &proof)
			mutate(&proof)
			proof.DecisionDigest = ""
			proof.DecisionDigest = modelDigest("service-boundary-decision/v1", proof)
			if !errors.Is(RevalidateAddition(r, b, a, proof), ErrStaleBoundaryProof) {
				t.Fatal("accepted forged but internally consistent proof")
			}
		})
	}
	// Counter-evidence deletion on an authentic blocked result cannot be hidden
	// by recomputing its unkeyed digest either.
	a.Services[0].Methods[1].Operation.Boundary.Context = "other.context"
	blocked := evaluate(t, r, b, a)
	blocked.CounterEvidence = []EvidenceRef{}
	blocked.DecisionDigest = ""
	blocked.DecisionDigest = modelDigest("service-boundary-decision/v1", blocked)
	if !errors.Is(RevalidateAddition(r, b, a, blocked), ErrStaleBoundaryProof) {
		t.Fatal("accepted omitted counter-evidence")
	}
}

func TestIssue161DecisionFreshnessAndInputIsolation(t *testing.T) {
	r, b, a := additionFixture()
	rawB, _ := json.Marshal(b)
	rawA, _ := json.Marshal(a)
	d := evaluate(t, r, b, a)
	postB, _ := json.Marshal(b)
	postA, _ := json.Marshal(a)
	if string(rawB) != string(postB) || string(rawA) != string(postA) {
		t.Fatal("mutated canonical inputs")
	}
	a.Files[0], a.Files[1] = a.Files[1], a.Files[0]
	a.Services[0].Methods[0], a.Services[0].Methods[1] = a.Services[0].Methods[1], a.Services[0].Methods[0]
	if !reflect.DeepEqual(d, evaluate(t, r, b, a)) {
		t.Fatal("input ordering changed decision")
	}
	r2 := r
	r2.BaseSHA = strings.Repeat("b", 40)
	if !errors.Is(RevalidateAddition(r2, b, a, d), ErrStaleBoundaryProof) {
		t.Fatal("old-base proof reused")
	}
	// Same execution plans but a changed architecture declaration invalidates it.
	a.Services[0].Methods[0].Operation.Boundary.Aggregate = "invoice"
	if !errors.Is(RevalidateAddition(r, b, a, d), ErrStaleBoundaryProof) {
		t.Fatal("intent-only change invisible")
	}
}

func TestIssue161DecisionMalformedAndUnsupportedInputs(t *testing.T) {
	r, b, a := additionFixture()
	for _, sha := range []string{"", "main", "abc123", strings.Repeat("0", 40), strings.Repeat("A", 40), strings.Repeat("x", 40)} {
		bad := r
		bad.BaseSHA = sha
		if _, err := EvaluateAddition(bad, b, a); err == nil {
			t.Fatalf("accepted base %q", sha)
		}
	}
	for _, id := range []string{"", " sales.next", "sales.get", "sales.absent"} {
		bad := r
		bad.OperationID = id
		if _, err := EvaluateAddition(bad, b, a); err == nil {
			t.Fatalf("accepted operation %q", id)
		}
	}
	bad := r
	bad.Application = "sales/missing"
	if _, err := EvaluateAddition(bad, b, a); err == nil {
		t.Fatal("missing application accepted")
	}
	a.SchemaVersion = 999
	if _, err := EvaluateAddition(r, b, a); err == nil {
		t.Fatal("unsupported manifest accepted")
	}
}

func TestIssue161DecisionNoCountThreshold(t *testing.T) {
	r, b, a := additionFixture()
	for i := 0; i < 32; i++ {
		m := detachedManifest(b).Services[0].Methods[0]
		m.Name = "Peer" + strings.Repeat("X", i+1)
		m.FullName = "sales.v1.Orders." + m.Name
		m.Operation.ID = "sales.peer" + strings.Repeat("x", i+1)
		m.Operation.UseCase = "peer" + strings.Repeat("x", i+1)
		b.Services[0].Methods = append(b.Services[0].Methods, m)
		a.Services[0].Methods = append(a.Services[0].Methods, m)
	}
	if d := evaluate(t, r, b, a); d.Outcome != ReuseExistingApplication || len(d.PeerWitnesses) != 33 {
		t.Fatalf("count replaced semantic evidence: %+v", d)
	}
}
