package change

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/auditcore"
)

func TestQualityDebtAdvisoryEvidenceBinding(t *testing.T) {
	root := t.TempDir()
	runEvidenceGit(t, root, "init")
	runEvidenceGit(t, root, "config", "user.email", "quality-test@example.invalid")
	runEvidenceGit(t, root, "config", "user.name", "Quality Test")

	sourcePath := "internal/tenant/application/service.go"
	absoluteSource := filepath.Join(root, filepath.FromSlash(sourcePath))
	if err := os.MkdirAll(filepath.Dir(absoluteSource), 0o755); err != nil {
		t.Fatal(err)
	}
	baseContent := "package application\n\ntype Service struct{}\n"
	if err := os.WriteFile(absoluteSource, []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runEvidenceGit(t, root, "add", ".")
	runEvidenceGit(t, root, "commit", "-m", "base")
	baseSHA := strings.TrimSpace(runEvidenceGit(t, root, "rev-parse", "HEAD"))

	currentContent := "package application\n\ntype Service struct{ Name string }\n"
	if err := os.WriteFile(absoluteSource, []byte(currentContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runEvidenceGit(t, root, "add", ".")
	runEvidenceGit(t, root, "commit", "-m", "current")
	headSHA := strings.TrimSpace(runEvidenceGit(t, root, "rev-parse", "HEAD"))
	candidateSHA := strings.Repeat("c", 64)
	changeEvidenceSHA := strings.Repeat("d", 64)

	baseline := semanticEvidenceAttestation(t, baseSHA, sourcePath, baseContent, nil)
	current := semanticEvidenceAttestation(t, headSHA, sourcePath, currentContent, &advisorcore.SemanticChangeIdentity{
		BaseSHA: baseSHA, HeadSHA: headSHA, CandidateSHA256: candidateSHA, EvidenceSHA256: changeEvidenceSHA,
	})
	writeSemanticEvidenceAttestation(t, root, "baseline.json", baseline)
	writeSemanticEvidenceAttestation(t, root, "current.json", current)

	delta, err := loadAdvisoryQualityDebt(context.Background(), root, "baseline.json", "current.json", baseSHA, headSHA, candidateSHA)
	if err != nil {
		t.Fatalf("matching evidence rejected: %v", err)
	}
	proof, err := BuildQualityDebtProof(
		auditcore.DebtDelta{BaseRef: "base", BaseSHA: baseSHA, Existing: []auditcore.Finding{}, New: []auditcore.Finding{}, Fixed: []auditcore.Finding{}},
		delta, nil, baseSHA, headSHA, time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateQualityDebtProof(proof); err != nil {
		t.Fatalf("bound quality debt proof rejected: %v", err)
	}
	if err := validateQualityDebtCandidateBinding(&proof, baseSHA, headSHA, candidateSHA); err != nil {
		t.Fatalf("bound candidate rejected: %v", err)
	}

	t.Run("stale current review head", func(t *testing.T) {
		stale := semanticEvidenceAttestation(t, baseSHA, sourcePath, baseContent, &advisorcore.SemanticChangeIdentity{
			BaseSHA: baseSHA, HeadSHA: baseSHA, CandidateSHA256: candidateSHA, EvidenceSHA256: changeEvidenceSHA,
		})
		writeSemanticEvidenceAttestation(t, root, "stale-head.json", stale)
		if _, err := loadAdvisoryQualityDebt(context.Background(), root, "baseline.json", "stale-head.json", baseSHA, headSHA, candidateSHA); err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("STALE_ADVISORY_EVIDENCE_ACCEPTED: stale current review head: %v", err)
		}
	})

	t.Run("same head old source bytes", func(t *testing.T) {
		stale := semanticEvidenceAttestation(t, headSHA, sourcePath, "package application\n// stale bytes\n", &advisorcore.SemanticChangeIdentity{
			BaseSHA: baseSHA, HeadSHA: headSHA, CandidateSHA256: candidateSHA, EvidenceSHA256: changeEvidenceSHA,
		})
		writeSemanticEvidenceAttestation(t, root, "stale-source.json", stale)
		if _, err := loadAdvisoryQualityDebt(context.Background(), root, "baseline.json", "stale-source.json", baseSHA, headSHA, candidateSHA); err == nil {
			t.Fatal("STALE_ADVISORY_EVIDENCE_ACCEPTED: same head old source bytes")
		}
	})

	t.Run("review baseline differs from Change base", func(t *testing.T) {
		wrong := semanticEvidenceAttestation(t, headSHA, sourcePath, currentContent, nil)
		writeSemanticEvidenceAttestation(t, root, "wrong-baseline.json", wrong)
		if _, err := loadAdvisoryQualityDebt(context.Background(), root, "wrong-baseline.json", "current.json", baseSHA, headSHA, candidateSHA); err == nil {
			t.Fatal("STALE_ADVISORY_EVIDENCE_ACCEPTED: review baseline differs from Change base")
		}
	})

	t.Run("candidate source set changed after review", func(t *testing.T) {
		if _, err := loadAdvisoryQualityDebt(context.Background(), root, "baseline.json", "current.json", baseSHA, headSHA, strings.Repeat("e", 64)); err == nil || !strings.Contains(err.Error(), "candidate") {
			t.Fatalf("STALE_ADVISORY_EVIDENCE_ACCEPTED: candidate digest drift: %v", err)
		}
		if err := validateQualityDebtCandidateBinding(&proof, baseSHA, headSHA, strings.Repeat("e", 64)); err == nil {
			t.Fatal("review-packet candidate revalidation accepted stale advisory proof")
		}
	})
}

func semanticEvidenceAttestation(t *testing.T, headSHA, sourcePath, content string, change *advisorcore.SemanticChangeIdentity) advisorcore.SemanticReviewAttestation {
	t.Helper()
	request, err := advisorcore.NewSemanticReviewRequest(headSHA, []advisorcore.SemanticSource{{Path: sourcePath, Content: content}}, change)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := advisorcore.ValidateSemanticReviewResponse(request, advisorcore.SemanticReviewResponse{
		SchemaVersion: advisorcore.SemanticReviewSchemaVersion,
		Authority:     advisorcore.AuthorityAdvisoryOnly,
		RequestDigest: request.RequestDigest,
		Findings:      []advisorcore.SemanticFinding{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return attestation
}

func writeSemanticEvidenceAttestation(t *testing.T, root, name string, attestation advisorcore.SemanticReviewAttestation) {
	t.Helper()
	contents, err := advisorcore.MarshalSemanticReviewAttestation(attestation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func runEvidenceGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
