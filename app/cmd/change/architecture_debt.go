package change

import (
	"fmt"
	"strings"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
)

func collectArchitectureDebt(root, baseSHA string) (auditcore.DebtDelta, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: base SHA is required")
	}
	report, err := audit.BuildWithBase(root, baseSHA)
	if err != nil {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: %w", err)
	}
	if report.Debt == nil || report.Debt.BaseSHA != baseSHA {
		return auditcore.DebtDelta{}, fmt.Errorf("architecture debt proof: audit did not resolve change base %s", baseSHA)
	}
	return *report.Debt, nil
}

func recordArchitectureDebt(attestation *ChangeAttestation, debt auditcore.DebtDelta) {
	if attestation == nil {
		return
	}
	attestation.ArchitectureDebt = &debt
	detail := fmt.Sprintf("existing=%d new=%d fixed=%d", len(debt.Existing), len(debt.New), len(debt.Fixed))
	if len(debt.New) == 0 {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "architecture-debt", Status: "pass", Detail: detail})
		return
	}
	attestation.Gates = append(attestation.Gates, GateResult{Name: "architecture-debt", Status: "fail", Detail: detail})
	for _, finding := range debt.New {
		attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic(
			"architecture-debt",
			architectureDebtFindingPath(finding),
			fmt.Sprintf("new proven architecture debt %s %s: %s", finding.Rule, finding.Subject, finding.Summary),
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
