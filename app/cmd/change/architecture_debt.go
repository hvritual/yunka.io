package change

import (
	"fmt"
	"strings"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/projectflow"
)

func collectArchitectureDebt(root, baseSHA string) (auditcore.DebtDelta, error) {
	return collectArchitectureDebtWithOptions(projectflow.Options{Root: root}, baseSHA)
}

func collectArchitectureDebtWithOptions(options projectflow.Options, baseSHA string) (auditcore.DebtDelta, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: base SHA is required")
	}
	report, err := audit.BuildWithBaseOptions(options, baseSHA)
	if err != nil {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: %w", err)
	}
	if report.Debt == nil || report.Debt.BaseSHA != baseSHA {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: audit did not resolve change base %s", baseSHA)
	}
	return *report.Debt, nil
}

// recordArchitectureDebt preserves the canonical deterministic debt delta as
// evidence. It no longer treats every new deterministic finding as blocking;
// accepted blocking policy and exact waivers are evaluated by recordQualityDebt.
func recordArchitectureDebt(attestation *ChangeAttestation, debt auditcore.DebtDelta) {
	if attestation == nil {
		return
	}
	attestation.ArchitectureDebt = &debt
	detail := fmt.Sprintf("existing=%d new=%d fixed=%d blocking_new=%d", len(debt.Existing), len(debt.New), len(debt.Fixed), len(blockingNewFindings(debt.New)))
	attestation.Gates = append(attestation.Gates, GateResult{Name: "architecture-debt", Status: "pass", Detail: detail})
}

func recordQualityDebt(attestation *ChangeAttestation, proof QualityDebtProof) {
	if attestation == nil {
		return
	}
	if err := ValidateQualityDebtProof(proof); err != nil {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "quality-debt", Status: "fail", Detail: err.Error()})
		attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic("quality-debt", "", err.Error()))
		return
	}
	attestation.QualityDebt = &proof
	advisoryExisting, advisoryNew, advisoryResolved := 0, 0, 0
	if proof.Advisory != nil {
		advisoryExisting = len(proof.Advisory.Existing)
		advisoryNew = len(proof.Advisory.New)
		advisoryResolved = len(proof.Advisory.Resolved)
	}
	detail := fmt.Sprintf(
		"deterministic(existing=%d new=%d fixed=%d blocking=%d waived=%d unwaived=%d) advisory(existing=%d new=%d resolved=%d)",
		len(proof.Deterministic.Existing), len(proof.Deterministic.New), len(proof.Deterministic.Fixed),
		len(proof.BlockingNew), len(proof.WaivedBlocking), len(proof.UnwaivedBlocking),
		advisoryExisting, advisoryNew, advisoryResolved,
	)
	if len(proof.UnwaivedBlocking) == 0 {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "quality-debt", Status: "pass", Detail: detail})
		return
	}
	attestation.Gates = append(attestation.Gates, GateResult{Name: "quality-debt", Status: "fail", Detail: detail})
	for _, finding := range proof.UnwaivedBlocking {
		attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic(
			"quality-debt",
			qualityFindingPath(finding),
			fmt.Sprintf("new blocking engineering-quality debt %s %s: %s", finding.Rule, finding.Subject, finding.Summary),
		))
	}
}

func architectureDebtFindingPath(finding auditcore.Finding) string {
	for _, evidence := range finding.Evidence {
		if path := strings.TrimSpace(evidence.Path); path != "" {
			return path
		}
	}
	return ""
}
