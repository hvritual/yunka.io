package auditcore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const SchemaVersion = 1

type FindingClass string

const (
	FindingProvenViolation     FindingClass = "proven_violation"
	FindingEvidenceObservation FindingClass = "evidence_observation"
)

type EvidenceKind string

const (
	EvidenceCanonical EvidenceKind = "canonical"
	EvidenceSource    EvidenceKind = "source"
	EvidenceGenerated EvidenceKind = "generated"
	EvidenceGit       EvidenceKind = "git"
	EvidenceRuntime   EvidenceKind = "runtime"
)

type ProjectIdentity struct {
	GoModule string `json:"goModule,omitempty"`
	Profiled bool   `json:"profiled"`
}

type Evidence struct {
	Kind   EvidenceKind `json:"kind"`
	Source string       `json:"source"`
	Path   string       `json:"path,omitempty"`
	Detail string       `json:"detail,omitempty"`
}

type Finding struct {
	ID        string       `json:"id"`
	Rule      string       `json:"rule"`
	Class     FindingClass `json:"class"`
	Subject   string       `json:"subject"`
	Summary   string       `json:"summary"`
	Invariant string       `json:"invariant,omitempty"`
	Evidence  []Evidence   `json:"evidence"`
}

type DebtDelta struct {
	BaseRef  string    `json:"baseRef"`
	BaseSHA  string    `json:"baseSha"`
	Existing []Finding `json:"existing"`
	New      []Finding `json:"new"`
	Fixed    []Finding `json:"fixed"`
}

type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Project       ProjectIdentity `json:"project"`
	Source        SourceSnapshot  `json:"source"`
	Findings      []Finding       `json:"findings"`
	Debt          *DebtDelta      `json:"debt,omitempty"`
}

func NewReport(project ProjectIdentity) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Project:       project,
		Source:        SourceSnapshot{Files: []GoSourceFile{}},
		Findings:      []Finding{},
	}
}

func Normalize(report *Report) {
	if report == nil {
		return
	}
	report.Project.GoModule = strings.TrimSpace(report.Project.GoModule)
	NormalizeSource(&report.Source)
	normalizeFindings(report.Findings)
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	if report.Debt != nil {
		report.Debt.BaseRef = strings.TrimSpace(report.Debt.BaseRef)
		report.Debt.BaseSHA = strings.TrimSpace(report.Debt.BaseSHA)
		normalizeFindings(report.Debt.Existing)
		normalizeFindings(report.Debt.New)
		normalizeFindings(report.Debt.Fixed)
		if report.Debt.Existing == nil {
			report.Debt.Existing = []Finding{}
		}
		if report.Debt.New == nil {
			report.Debt.New = []Finding{}
		}
		if report.Debt.Fixed == nil {
			report.Debt.Fixed = []Finding{}
		}
	}
}

func Validate(report Report) error {
	if report.SchemaVersion != SchemaVersion {
		return fmt.Errorf("audit: unsupported schemaVersion %d", report.SchemaVersion)
	}
	for _, file := range report.Source.Files {
		if file.Path == "" {
			return fmt.Errorf("audit: source file path is required")
		}
		if file.Package == "" {
			return fmt.Errorf("audit: source file %s package is required", file.Path)
		}
	}
	if err := validateFindings(report.Findings, false); err != nil {
		return err
	}
	if report.Debt != nil {
		if report.Debt.BaseRef == "" || report.Debt.BaseSHA == "" {
			return fmt.Errorf("audit: debt baseline ref and SHA are required")
		}
		if err := validateFindings(report.Debt.Existing, true); err != nil {
			return fmt.Errorf("audit: existing debt: %w", err)
		}
		if err := validateFindings(report.Debt.New, true); err != nil {
			return fmt.Errorf("audit: new debt: %w", err)
		}
		if err := validateFindings(report.Debt.Fixed, true); err != nil {
			return fmt.Errorf("audit: fixed debt: %w", err)
		}
	}
	return nil
}

func Marshal(report Report) ([]byte, error) {
	normalized := cloneReport(report)
	Normalize(&normalized)
	if err := Validate(normalized); err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func normalizeFindings(values []Finding) {
	for index := range values {
		finding := &values[index]
		finding.ID = strings.TrimSpace(finding.ID)
		finding.Rule = strings.TrimSpace(finding.Rule)
		finding.Subject = strings.TrimSpace(finding.Subject)
		finding.Summary = strings.TrimSpace(finding.Summary)
		finding.Invariant = strings.TrimSpace(finding.Invariant)
		finding.Evidence = normalizeEvidence(finding.Evidence)
	}
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Subject < right.Subject
	})
}

func validateFindings(values []Finding, provenOnly bool) error {
	seen := make(map[string]struct{}, len(values))
	for _, finding := range values {
		if finding.ID == "" {
			return fmt.Errorf("finding id is required")
		}
		if _, duplicate := seen[finding.ID]; duplicate {
			return fmt.Errorf("duplicate finding id %q", finding.ID)
		}
		seen[finding.ID] = struct{}{}
		if finding.Rule == "" {
			return fmt.Errorf("finding %s rule is required", finding.ID)
		}
		if finding.Subject == "" {
			return fmt.Errorf("finding %s subject is required", finding.ID)
		}
		if finding.Summary == "" {
			return fmt.Errorf("finding %s summary is required", finding.ID)
		}
		switch finding.Class {
		case FindingProvenViolation:
			if finding.Invariant == "" {
				return fmt.Errorf("proven finding %s invariant is required", finding.ID)
			}
		case FindingEvidenceObservation:
			if provenOnly {
				return fmt.Errorf("finding %s class %q cannot participate in debt delta", finding.ID, finding.Class)
			}
		default:
			return fmt.Errorf("finding %s class %q is unsupported", finding.ID, finding.Class)
		}
		if len(finding.Evidence) == 0 {
			return fmt.Errorf("finding %s evidence is required", finding.ID)
		}
		for _, evidence := range finding.Evidence {
			if !supportedEvidenceKind(evidence.Kind) {
				return fmt.Errorf("finding %s evidence kind %q is unsupported", finding.ID, evidence.Kind)
			}
			if evidence.Source == "" {
				return fmt.Errorf("finding %s evidence source is required", finding.ID)
			}
		}
	}
	return nil
}

func normalizeEvidence(values []Evidence) []Evidence {
	seen := make(map[string]Evidence, len(values))
	for _, value := range values {
		value.Source = strings.TrimSpace(value.Source)
		value.Path = strings.TrimSpace(value.Path)
		value.Detail = strings.TrimSpace(value.Detail)
		key := string(value.Kind) + "\x00" + value.Source + "\x00" + value.Path + "\x00" + value.Detail
		seen[key] = value
	}
	result := make([]Evidence, 0, len(seen))
	for _, value := range seen {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Detail < right.Detail
	})
	if result == nil {
		return []Evidence{}
	}
	return result
}

func supportedEvidenceKind(kind EvidenceKind) bool {
	switch kind {
	case EvidenceCanonical, EvidenceSource, EvidenceGenerated, EvidenceGit, EvidenceRuntime:
		return true
	default:
		return false
	}
}

func cloneFinding(value Finding) Finding {
	result := value
	result.Evidence = append([]Evidence(nil), value.Evidence...)
	return result
}

func cloneFindings(values []Finding) []Finding {
	result := make([]Finding, len(values))
	for index, finding := range values {
		result[index] = cloneFinding(finding)
	}
	return result
}

func cloneReport(report Report) Report {
	result := report
	result.Source.Files = make([]GoSourceFile, len(report.Source.Files))
	for index, file := range report.Source.Files {
		result.Source.Files[index] = file
		result.Source.Files[index].Imports = append([]string(nil), file.Imports...)
	}
	result.Findings = cloneFindings(report.Findings)
	if report.Debt != nil {
		debt := *report.Debt
		debt.Existing = cloneFindings(report.Debt.Existing)
		debt.New = cloneFindings(report.Debt.New)
		debt.Fixed = cloneFindings(report.Debt.Fixed)
		result.Debt = &debt
	}
	return result
}
