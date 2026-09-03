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

type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Project       ProjectIdentity `json:"project"`
	Source        SourceSnapshot  `json:"source"`
	Findings      []Finding       `json:"findings"`
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
	for index := range report.Findings {
		finding := &report.Findings[index]
		finding.ID = strings.TrimSpace(finding.ID)
		finding.Rule = strings.TrimSpace(finding.Rule)
		finding.Subject = strings.TrimSpace(finding.Subject)
		finding.Summary = strings.TrimSpace(finding.Summary)
		finding.Invariant = strings.TrimSpace(finding.Invariant)
		finding.Evidence = normalizeEvidence(finding.Evidence)
	}
	sort.Slice(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Subject < right.Subject
	})
	if report.Findings == nil {
		report.Findings = []Finding{}
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
	seen := make(map[string]struct{}, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.ID == "" {
			return fmt.Errorf("audit: finding id is required")
		}
		if _, duplicate := seen[finding.ID]; duplicate {
			return fmt.Errorf("audit: duplicate finding id %q", finding.ID)
		}
		seen[finding.ID] = struct{}{}
		if finding.Rule == "" {
			return fmt.Errorf("audit: finding %s rule is required", finding.ID)
		}
		if finding.Subject == "" {
			return fmt.Errorf("audit: finding %s subject is required", finding.ID)
		}
		if finding.Summary == "" {
			return fmt.Errorf("audit: finding %s summary is required", finding.ID)
		}
		switch finding.Class {
		case FindingProvenViolation:
			if finding.Invariant == "" {
				return fmt.Errorf("audit: proven finding %s invariant is required", finding.ID)
			}
		case FindingEvidenceObservation:
		default:
			return fmt.Errorf("audit: finding %s class %q is unsupported", finding.ID, finding.Class)
		}
		if len(finding.Evidence) == 0 {
			return fmt.Errorf("audit: finding %s evidence is required", finding.ID)
		}
		for _, evidence := range finding.Evidence {
			if !supportedEvidenceKind(evidence.Kind) {
				return fmt.Errorf("audit: finding %s evidence kind %q is unsupported", finding.ID, evidence.Kind)
			}
			if evidence.Source == "" {
				return fmt.Errorf("audit: finding %s evidence source is required", finding.ID)
			}
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

func cloneReport(report Report) Report {
	result := report
	result.Source.Files = make([]GoSourceFile, len(report.Source.Files))
	for index, file := range report.Source.Files {
		result.Source.Files[index] = file
		result.Source.Files[index].Imports = append([]string(nil), file.Imports...)
	}
	result.Findings = make([]Finding, len(report.Findings))
	for index, finding := range report.Findings {
		result.Findings[index] = finding
		result.Findings[index].Evidence = append([]Evidence(nil), finding.Evidence...)
	}
	return result
}
