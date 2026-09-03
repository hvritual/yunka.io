package auditcore

import "sort"

// CompareProvenFindings reconciles deterministic architecture debt by stable
// Finding ID. Evidence observations are deliberately excluded: only facts the
// framework can prove may participate in existing/new/fixed debt accounting.
func CompareProvenFindings(base, current []Finding) DebtDelta {
	baseIndex := provenFindingIndex(base)
	currentIndex := provenFindingIndex(current)
	ids := make(map[string]struct{}, len(baseIndex)+len(currentIndex))
	for id := range baseIndex {
		ids[id] = struct{}{}
	}
	for id := range currentIndex {
		ids[id] = struct{}{}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)

	result := DebtDelta{
		Existing: []Finding{},
		New:      []Finding{},
		Fixed:    []Finding{},
	}
	for _, id := range ordered {
		before, beforeOK := baseIndex[id]
		after, afterOK := currentIndex[id]
		switch {
		case beforeOK && afterOK:
			result.Existing = append(result.Existing, cloneFinding(after))
		case afterOK:
			result.New = append(result.New, cloneFinding(after))
		case beforeOK:
			result.Fixed = append(result.Fixed, cloneFinding(before))
		}
	}
	return result
}

func provenFindingIndex(values []Finding) map[string]Finding {
	result := map[string]Finding{}
	for _, finding := range values {
		if finding.Class != FindingProvenViolation || finding.ID == "" {
			continue
		}
		result[finding.ID] = finding
	}
	return result
}
