package auditcore

import "testing"

func TestCompareProvenFindingsClassifiesExistingNewAndFixed(t *testing.T) {
	proven := func(id string) Finding {
		return Finding{
			ID:        id,
			Rule:      "AUDIT-TEST-001",
			Class:     FindingProvenViolation,
			Subject:   "tenant",
			Summary:   "test",
			Invariant: "test invariant",
			Evidence:  []Evidence{{Kind: EvidenceSource, Source: "test"}},
		}
	}
	observation := Finding{
		ID:       "observation",
		Rule:     "AUDIT-OBS-001",
		Class:    FindingEvidenceObservation,
		Subject:  "tenant",
		Summary:  "observation",
		Evidence: []Evidence{{Kind: EvidenceSource, Source: "test"}},
	}

	result := CompareProvenFindings(
		[]Finding{proven("fixed"), proven("existing"), observation},
		[]Finding{proven("existing"), proven("new"), observation},
	)
	if len(result.Existing) != 1 || result.Existing[0].ID != "existing" {
		t.Fatalf("existing=%#v", result.Existing)
	}
	if len(result.New) != 1 || result.New[0].ID != "new" {
		t.Fatalf("new=%#v", result.New)
	}
	if len(result.Fixed) != 1 || result.Fixed[0].ID != "fixed" {
		t.Fatalf("fixed=%#v", result.Fixed)
	}
}
