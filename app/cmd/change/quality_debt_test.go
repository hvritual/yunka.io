package change

import (
	"strings"
	"testing"
	"time"

	"yunka.io/app/cmd/auditcore"
)

func TestQualityDebtHistoricalAndNonBlockingDebtRemainVisibleWithoutBlocking(t *testing.T) {
	base := strings.Repeat("1", 40)
	head := strings.Repeat("2", 40)
	debt := auditcore.DebtDelta{
		BaseRef: "base", BaseSHA: base,
		Existing: []auditcore.Finding{qualityTestFinding("existing", true)},
		New:      []auditcore.Finding{qualityTestFinding("new-advisory-policy", false)},
		Fixed:    []auditcore.Finding{qualityTestFinding("fixed", true)},
	}
	proof, err := BuildQualityDebtProof(debt, nil, nil, base, head, time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.BlockingNew) != 0 || len(proof.UnwaivedBlocking) != 0 {
		t.Fatalf("proof blocking=%d unwaived=%d", len(proof.BlockingNew), len(proof.UnwaivedBlocking))
	}
	if len(proof.Deterministic.Existing) != 1 || len(proof.Deterministic.New) != 1 || len(proof.Deterministic.Fixed) != 1 {
		t.Fatalf("deterministic delta=%#v", proof.Deterministic)
	}
	attestation := ChangeAttestation{}
	recordQualityDebt(&attestation, proof)
	if len(attestation.Diagnostics) != 0 || len(attestation.Gates) != 1 || attestation.Gates[0].Status != "pass" {
		t.Fatalf("attestation=%#v", attestation)
	}
}

func TestQualityDebtNewBlockingFindingFailsWithoutWaiver(t *testing.T) {
	base := strings.Repeat("3", 40)
	head := strings.Repeat("4", 40)
	finding := qualityTestFinding("blocking-new", true)
	debt := auditcore.DebtDelta{BaseRef: "base", BaseSHA: base, New: []auditcore.Finding{finding}}
	proof, err := BuildQualityDebtProof(debt, nil, nil, base, head, time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.UnwaivedBlocking) != 1 || proof.UnwaivedBlocking[0].ID != finding.ID {
		t.Fatalf("proof=%#v", proof)
	}
	attestation := ChangeAttestation{}
	recordQualityDebt(&attestation, proof)
	if len(attestation.Diagnostics) != 1 || len(attestation.Gates) != 1 || attestation.Gates[0].Status != "fail" {
		t.Fatalf("attestation=%#v", attestation)
	}
}

func TestQualityDebtExactWaiverAllowsOnlySelectedBlockingFinding(t *testing.T) {
	base := strings.Repeat("5", 40)
	head := strings.Repeat("6", 40)
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	first := qualityTestFinding("first", true)
	second := qualityTestFinding("second", true)
	debt := auditcore.DebtDelta{BaseRef: "base", BaseSHA: base, New: []auditcore.Finding{first, second}}
	waivers, err := NewQualityWaiverSet(
		base, head, "platform-owner", "temporary migration debt", "2026-09-20T10:00:00Z", "review when migration lands",
		debt.New, []string{first.ID}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := BuildQualityDebtProof(debt, nil, &waivers, base, head, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.WaivedBlocking) != 1 || proof.WaivedBlocking[0].FindingID != first.ID {
		t.Fatalf("waived=%#v", proof.WaivedBlocking)
	}
	if len(proof.UnwaivedBlocking) != 1 || proof.UnwaivedBlocking[0].ID != second.ID {
		t.Fatalf("unwaived=%#v", proof.UnwaivedBlocking)
	}
}

func TestQualityWaiverRejectsStaleExpiredAndScopeMismatch(t *testing.T) {
	base := strings.Repeat("7", 40)
	head := strings.Repeat("8", 40)
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	finding := qualityTestFinding("scope", true)
	set, err := NewQualityWaiverSet(base, head, "owner", "reason", "2026-09-20T10:00:00Z", "review condition", []auditcore.Finding{finding}, []string{finding.ID}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalQualityWaiverSet(set, base, strings.Repeat("9", 40), []auditcore.Finding{finding}, now, true); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale err=%v", err)
	}
	if _, err := canonicalQualityWaiverSet(set, base, head, []auditcore.Finding{finding}, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), true); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired err=%v", err)
	}
	tampered := set
	tampered.Waivers = append([]QualityWaiver(nil), set.Waivers...)
	tampered.Waivers[0].Scope.FindingSHA256 = strings.Repeat("a", 64)
	tampered.Waivers[0].ID = qualityWaiverID(tampered.Waivers[0])
	tampered.SetSHA256 = ""
	tampered, err = canonicalQualityWaiverSet(tampered, base, head, []auditcore.Finding{finding}, now, false)
	if err == nil {
		t.Fatal("scope-mismatched waiver unexpectedly normalized")
	}
}

func TestQualityDebtProofIsDeterministicAndTamperEvident(t *testing.T) {
	base := strings.Repeat("b", 40)
	head := strings.Repeat("c", 40)
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	finding := qualityTestFinding("deterministic", true)
	debt := auditcore.DebtDelta{BaseRef: "base", BaseSHA: base, New: []auditcore.Finding{finding}}
	first, err := BuildQualityDebtProof(debt, nil, nil, base, head, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildQualityDebtProof(debt, nil, nil, base, head, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProofSHA256 != second.ProofSHA256 {
		t.Fatalf("proof digests differ %s %s", first.ProofSHA256, second.ProofSHA256)
	}
	if err := ValidateQualityDebtProof(first); err != nil {
		t.Fatal(err)
	}
	tampered := first
	tampered.UnwaivedBlocking = nil
	if err := ValidateQualityDebtProof(tampered); err == nil {
		t.Fatal("tampered debt proof was accepted")
	}
}

func TestReviewQualityDebtProjectsProofAndBlockingPartition(t *testing.T) {
	base := strings.Repeat("d", 40)
	head := strings.Repeat("e", 40)
	finding := qualityTestFinding("review", true)
	proof, err := BuildQualityDebtProof(
		auditcore.DebtDelta{BaseRef: "base", BaseSHA: base, New: []auditcore.Finding{finding}},
		nil, nil, base, head, time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	projection := reviewQualityDebt(&proof)
	if projection == nil || projection.BlockingNew != 1 || projection.UnwaivedBlocking != 1 || projection.ProofSHA256 != proof.ProofSHA256 {
		t.Fatalf("projection=%#v", projection)
	}
	if err := validateReviewQualityDebt(projection); err != nil {
		t.Fatal(err)
	}
}

func qualityTestFinding(id string, blocking bool) auditcore.Finding {
	return auditcore.Finding{
		ID: id,
		Rule: "AUDIT-SIZE-001",
		Class: auditcore.FindingProvenViolation,
		Blocking: blocking,
		Subject: "internal/tenant/application/service.go",
		Summary: "test engineering-quality debt",
		Invariant: "test invariant",
		Path: "internal/tenant/application/service.go",
		Symbol: "service.go",
		Reason: "test reason",
		Remediation: "test remediation",
		Evidence: []auditcore.Evidence{{Kind: auditcore.EvidenceSource, Source: "test", Path: "internal/tenant/application/service.go", Detail: "test"}},
	}
}
