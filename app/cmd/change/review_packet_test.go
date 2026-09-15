package change

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuralRefactorProjectsExplicitZeroSemanticDelta(t *testing.T) {
	narrative := ReviewNarrative{
		Problem:          "mixed responsibilities make review difficult",
		CurrentConcepts:  []string{"tenant role aggregate"},
		DesiredOwnership: []string{"tenant role policy"},
		Why:              "make ownership reviewable",
		What:             "move handwritten helpers without behavior change",
		Boundary:         "no behavior, API, persistence, or generated contract change",
	}
	if err := normalizeReviewNarrative(&narrative); err != nil {
		t.Fatal(err)
	}
	semantic := SemanticReport{SchemaVersion: SemanticReportSchemaVersion, OperationID: "tenant.role.read", Deltas: []SemanticDelta{}, Violations: []SemanticDelta{}}
	changes := []FileChange{{Status: "M", Path: "internal/tenant/application/read_role.go", Class: "editable", Owner: "developer"}}

	behavior := semanticReviewDelta(semantic.Deltas)
	api := publicAPIReviewDelta(semantic.Deltas)
	persistence := persistenceReviewDelta(changes)
	generated := generatedReviewDelta(changes)
	for name, delta := range map[string]ReviewDelta{"behavior": behavior, "api": api, "persistence": persistence, "generated": generated} {
		if delta.State != ReviewDeltaNone || len(delta.Facts) != 0 {
			t.Fatalf("%s delta=%#v want explicit NONE", name, delta)
		}
	}
}

func TestBehaviorChangeSurfacesCanonicalInvariantAPIAndPersistenceFacts(t *testing.T) {
	semantic := []SemanticDelta{
		{Category: SemanticPermission, Subject: "operation:tenant.role.update", Field: "security.permissions", Before: `["role.read"]`, After: `["role.write"]`, Allowed: true},
		{Category: SemanticContract, Subject: "operation:tenant.role.update", Field: "contract", Before: `{"response":"Old"}`, After: `{"response":"New"}`, Allowed: true},
	}
	changes := []FileChange{{Status: "M", Path: "internal/tenant/infrastructure/persistence/role.go", Class: "editable", Owner: "developer"}}
	narrative := ReviewNarrative{AffectedInvariants: []string{"owner role remains manageable"}}

	behavior := semanticReviewDelta(semantic)
	api := publicAPIReviewDelta(semantic)
	persistence := persistenceReviewDelta(changes)
	invariants := deriveAffectedInvariants(narrative, semantic)
	if behavior.State != ReviewDeltaChanged || len(behavior.Facts) != 2 {
		t.Fatalf("behavior=%#v", behavior)
	}
	if api.State != ReviewDeltaChanged || len(api.Facts) != 1 || !strings.Contains(api.Facts[0].Kind, SemanticContract) {
		t.Fatalf("api=%#v", api)
	}
	if persistence.State != ReviewDeltaChanged || len(persistence.Facts) != 1 {
		t.Fatalf("persistence=%#v", persistence)
	}
	if !contains(invariants, "operation:tenant.role.update:security.permissions") || !contains(invariants, "owner role remains manageable") {
		t.Fatalf("invariants=%v", invariants)
	}
}

func TestCandidateDigestDetectsSamePathContentMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "tenant", "application", "service.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package application\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changes := []FileChange{{Status: "M", Path: "internal/tenant/application/service.go", Class: "editable", Owner: "developer"}}
	before, err := digestCandidate(root, "base", "head", changes)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package application\n\nfunc A() { println(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := digestCandidate(root, "base", "head", changes)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("same-path content mutation did not change exact-candidate digest")
	}
}

func TestChangedPathDigestChangesWhenCandidatePathSetChanges(t *testing.T) {
	left, err := digestJSON([]FileChange{{Status: "M", Path: "internal/a.go", Class: "editable"}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := digestJSON([]FileChange{{Status: "M", Path: "internal/a.go", Class: "editable"}, {Status: "A", Path: "internal/b.go", Class: "editable"}})
	if err != nil {
		t.Fatal(err)
	}
	if left == right {
		t.Fatal("changed path set did not change digest")
	}
}

func TestReviewPacketValidationRejectsNarrativeOrEvidenceTamper(t *testing.T) {
	packet := validReviewPacketFixture(t)
	if err := validateReviewPacket(packet); err != nil {
		t.Fatalf("valid fixture: %v", err)
	}

	tamperedNarrative := packet
	tamperedNarrative.Narrative.What = "silently change behavior"
	if err := validateReviewPacket(tamperedNarrative); err == nil || !strings.Contains(err.Error(), "narrative digest") {
		t.Fatalf("expected narrative tamper rejection, got %v", err)
	}

	tamperedEvidence := packet
	tamperedEvidence.Evidence.CandidateSHA256 = strings.Repeat("a", 64)
	if err := validateReviewPacket(tamperedEvidence); err == nil || !strings.Contains(err.Error(), "evidence digest") {
		t.Fatalf("expected evidence tamper rejection, got %v", err)
	}
}

func TestReviewPacketRenderingIsDeterministicAcrossHumanAndAgentSurfaces(t *testing.T) {
	packet := validReviewPacketFixture(t)
	first, err := RenderReviewPacket(packet, DefaultReviewPacketPath, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderReviewPacket(packet, DefaultReviewPacketPath, FormatAgentJSON)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("agent-json review packet is not byte-stable")
	}
	human, err := RenderReviewPacket(packet, DefaultReviewPacketPath, FormatText)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"WHY", "WHAT", "BOUNDARY", "behavior", "persistence", "PROOF", packet.Evidence.CandidateSHA256} {
		if !strings.Contains(human, expected) {
			t.Fatalf("human packet missing %q:\n%s", expected, human)
		}
	}
}

func validReviewPacketFixture(t *testing.T) ReviewPacket {
	t.Helper()
	narrative := ReviewNarrative{
		Problem:          "reviewers cannot see semantic intent before the diff",
		CurrentConcepts:  []string{"change attestation"},
		DesiredOwnership: []string{"human review evidence"},
		Why:              "make intent reviewable",
		What:             "project exact candidate evidence",
		Boundary:         "does not authorize mutation or merge",
	}
	if err := normalizeReviewNarrative(&narrative); err != nil {
		t.Fatal(err)
	}
	narrativeDigest, err := digestJSON(narrative)
	if err != nil {
		t.Fatal(err)
	}
	packet := ReviewPacket{
		SchemaVersion:       ReviewPacketSchemaVersion,
		Narrative:           narrative,
		BehaviorChange:      ReviewDelta{State: ReviewDeltaNone, Facts: []ReviewFact{}},
		PublicAPIChange:     ReviewDelta{State: ReviewDeltaNone, Facts: []ReviewFact{}},
		PersistenceChange:   ReviewDelta{State: ReviewDeltaNone, Facts: []ReviewFact{}},
		GeneratedCodeChange: ReviewDelta{State: ReviewDeltaNone, Facts: []ReviewFact{}},
		Verification:        ReviewVerification{Conformant: true, Gates: []GateResult{{Name: "go-test", Status: "pass"}}},
		AffectedInvariants:  []string{},
		Risks:               []string{},
		UnresolvedFindings:  []string{},
		Evidence: ReviewEvidenceIdentity{
			BaseSHA:            "base",
			HeadSHA:            "head",
			OperationID:        "tenant.role.read",
			ContractSHA256:     strings.Repeat("1", 64),
			AttestationSHA256:  strings.Repeat("2", 64),
			NarrativeSHA256:    narrativeDigest,
			ChangedPathsSHA256: strings.Repeat("3", 64),
			CandidateSHA256:    strings.Repeat("4", 64),
		},
	}
	packet.Evidence.EvidenceSHA256, err = reviewEvidenceDigest(packet.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	packet.Projection = ReviewProjection{Why: narrative.Why, What: narrative.What, Boundary: narrative.Boundary, Proof: reviewProof(packet)}
	normalizeReviewPacket(&packet)
	return packet
}
