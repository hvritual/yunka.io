package change

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
)

const (
	RemediationBindingSchemaVersion = 1
	RemediationCheckSchemaVersion   = 1
	DefaultRemediationBindingPath   = ".git/yunka/change-remediation.json"
)

type RemediationBinding struct {
	SchemaVersion   int      `json:"schemaVersion"`
	BaseSHA         string   `json:"baseSha"`
	ChangeSetDigest string   `json:"changeSetDigest"`
	FindingIDs      []string `json:"findingIds"`
}

type RemediationAuditReport struct {
	BaseSHA    string   `json:"baseSha"`
	Targets    []string `json:"targets"`
	Fixed      []string `json:"fixed"`
	Remaining  []string `json:"remaining"`
	NewDebt    []string `json:"newDebt"`
	Conformant bool     `json:"conformant"`
}

type RemediationCheckReport struct {
	SchemaVersion   int                    `json:"schemaVersion"`
	BaseSHA         string                 `json:"baseSha"`
	ChangeSetDigest string                 `json:"changeSetDigest"`
	ChangeSet       ChangeSetCheckReport   `json:"changeSet"`
	Audit           RemediationAuditReport `json:"audit"`
	Conformant      bool                   `json:"conformant"`
}

func BuildRemediationBinding(root string, value ChangeSet, findingIDs []string) (RemediationBinding, error) {
	if err := ensureCleanWorktree(root); err != nil {
		return RemediationBinding{}, &Failure{Kind: FailureEvidence, Err: err}
	}
	normalizeChangeSet(&value)
	if err := validateChangeSet(value); err != nil {
		return RemediationBinding{}, err
	}
	ids, err := normalizeRemediationFindingIDs(findingIDs)
	if err != nil {
		return RemediationBinding{}, &Failure{Kind: FailureIntent, Err: err}
	}
	digest, err := changeSetDigest(value)
	if err != nil {
		return RemediationBinding{}, &Failure{Kind: FailureEvidence, Err: err}
	}
	report, err := audit.BuildWithBase(root, value.BaseSHA)
	if err != nil {
		return RemediationBinding{}, &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation bind: audit baseline: %w", err)}
	}
	if report.Debt == nil || report.Debt.BaseSHA != value.BaseSHA {
		return RemediationBinding{}, &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation bind: audit did not resolve ChangeSet base %s", value.BaseSHA)}
	}
	existing := findingIDSet(report.Debt.Existing)
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return RemediationBinding{}, &Failure{Kind: FailureIntent, Err: fmt.Errorf("change remediation bind: finding %s must be a proven finding present at both ChangeSet base and current clean worktree", id)}
		}
	}
	binding := RemediationBinding{
		SchemaVersion:   RemediationBindingSchemaVersion,
		BaseSHA:         value.BaseSHA,
		ChangeSetDigest: digest,
		FindingIDs:      ids,
	}
	if err := validateRemediationBinding(binding); err != nil {
		return RemediationBinding{}, err
	}
	return binding, nil
}

func WriteRemediationBinding(root, output string, value RemediationBinding) (string, error) {
	normalizeRemediationBinding(&value)
	if err := validateRemediationBinding(value); err != nil {
		return "", err
	}
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultRemediationBindingPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return "", err
	}
	return display, nil
}

func LoadRemediationBinding(root, input string) (RemediationBinding, string, error) {
	path, display, err := resolveGitPrivateStatePath(root, input, DefaultRemediationBindingPath)
	if err != nil {
		return RemediationBinding{}, "", err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return RemediationBinding{}, "", err
	}
	var value RemediationBinding
	if err := decodeStrictJSON(contents, &value); err != nil {
		return RemediationBinding{}, "", fmt.Errorf("decode remediation binding: %w", err)
	}
	normalizeRemediationBinding(&value)
	if err := validateRemediationBinding(value); err != nil {
		return RemediationBinding{}, "", err
	}
	return value, display, nil
}

func ReconcileRemediation(root string, value ChangeSet, binding RemediationBinding) (RemediationCheckReport, error) {
	normalizeChangeSet(&value)
	if err := validateChangeSet(value); err != nil {
		return RemediationCheckReport{}, err
	}
	normalizeRemediationBinding(&binding)
	if err := validateRemediationBinding(binding); err != nil {
		return RemediationCheckReport{}, err
	}
	digest, err := changeSetDigest(value)
	if err != nil {
		return RemediationCheckReport{}, err
	}
	if binding.BaseSHA != value.BaseSHA {
		return RemediationCheckReport{}, fmt.Errorf("change remediation check: binding base %s differs from ChangeSet base %s", binding.BaseSHA, value.BaseSHA)
	}
	if binding.ChangeSetDigest != digest {
		return RemediationCheckReport{}, fmt.Errorf("change remediation check: ChangeSet digest mismatch; binding=%s current=%s", binding.ChangeSetDigest, digest)
	}

	changeReport, err := ReconcileChangeSet(root, value)
	if err != nil {
		return RemediationCheckReport{}, err
	}
	auditReport, err := audit.BuildWithBase(root, value.BaseSHA)
	if err != nil {
		return RemediationCheckReport{}, fmt.Errorf("change remediation check: audit debt: %w", err)
	}
	if auditReport.Debt == nil || auditReport.Debt.BaseSHA != value.BaseSHA {
		return RemediationCheckReport{}, fmt.Errorf("change remediation check: audit did not resolve ChangeSet base %s", value.BaseSHA)
	}

	baseline := findingIDSet(auditReport.Debt.Existing)
	for _, finding := range auditReport.Debt.Fixed {
		baseline[finding.ID] = struct{}{}
	}
	fixedSet := findingIDSet(auditReport.Debt.Fixed)
	auditResult := RemediationAuditReport{
		BaseSHA: value.BaseSHA,
		Targets: append([]string(nil), binding.FindingIDs...),
	}
	for _, id := range binding.FindingIDs {
		if _, ok := baseline[id]; !ok {
			return RemediationCheckReport{}, fmt.Errorf("change remediation check: finding %s was not present at immutable ChangeSet base", id)
		}
		if _, fixed := fixedSet[id]; fixed {
			auditResult.Fixed = append(auditResult.Fixed, id)
		} else {
			auditResult.Remaining = append(auditResult.Remaining, id)
		}
	}
	for _, finding := range auditReport.Debt.New {
		auditResult.NewDebt = append(auditResult.NewDebt, finding.ID)
	}
	auditResult.Fixed = uniqueSorted(auditResult.Fixed)
	auditResult.Remaining = uniqueSorted(auditResult.Remaining)
	auditResult.NewDebt = uniqueSorted(auditResult.NewDebt)
	if auditResult.Fixed == nil {
		auditResult.Fixed = []string{}
	}
	if auditResult.Remaining == nil {
		auditResult.Remaining = []string{}
	}
	if auditResult.NewDebt == nil {
		auditResult.NewDebt = []string{}
	}
	auditResult.Conformant = len(auditResult.Remaining) == 0 && len(auditResult.NewDebt) == 0

	return RemediationCheckReport{
		SchemaVersion:   RemediationCheckSchemaVersion,
		BaseSHA:         value.BaseSHA,
		ChangeSetDigest: digest,
		ChangeSet:       changeReport,
		Audit:           auditResult,
		Conformant:      changeReport.Conformant && auditResult.Conformant,
	}, nil
}

func RenderRemediationBinding(value RemediationBinding, path, format string) (string, error) {
	normalizeRemediationBinding(&value)
	if err := validateRemediationBinding(value); err != nil {
		return "", err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path        string             `json:"path"`
			Remediation RemediationBinding `json:"remediation"`
		}{Path: path, Remediation: value}
		contents, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change remediation bind: unsupported format %q", format)
	}
	return fmt.Sprintf("remediation %s\nbase        %s\nchange-set  %s\ntargets     %d\n", path, value.BaseSHA, value.ChangeSetDigest, len(value.FindingIDs)), nil
}

func RenderRemediationCheck(report RemediationCheckReport, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change remediation check: unsupported format %q", format)
	}
	return fmt.Sprintf(
		"remediation base %s\nchange-set conformant %t\nfixed       %d\nremaining   %d\nnew debt    %d\nconformant  %t\n",
		report.BaseSHA,
		report.ChangeSet.Conformant,
		len(report.Audit.Fixed),
		len(report.Audit.Remaining),
		len(report.Audit.NewDebt),
		report.Conformant,
	), nil
}

func changeSetDigest(value ChangeSet) (string, error) {
	normalizeChangeSet(&value)
	if err := validateChangeSet(value); err != nil {
		return "", err
	}
	contents, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeRemediationBinding(value *RemediationBinding) {
	if value == nil {
		return
	}
	value.BaseSHA = strings.TrimSpace(value.BaseSHA)
	value.ChangeSetDigest = strings.TrimSpace(value.ChangeSetDigest)
	ids := make([]string, 0, len(value.FindingIDs))
	for _, id := range value.FindingIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	value.FindingIDs = uniqueSorted(ids)
	if value.FindingIDs == nil {
		value.FindingIDs = []string{}
	}
}

func normalizeRemediationFindingIDs(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	result = uniqueSorted(result)
	if len(result) == 0 {
		return nil, fmt.Errorf("change remediation bind: at least one --finding is required")
	}
	return result, nil
}

func validateRemediationBinding(value RemediationBinding) error {
	if value.SchemaVersion != RemediationBindingSchemaVersion {
		return fmt.Errorf("unsupported remediation binding schemaVersion %d", value.SchemaVersion)
	}
	if value.BaseSHA == "" {
		return fmt.Errorf("change remediation: baseSha is required")
	}
	if value.ChangeSetDigest == "" {
		return fmt.Errorf("change remediation: changeSetDigest is required")
	}
	if len(value.FindingIDs) == 0 {
		return fmt.Errorf("change remediation: findingIds are required")
	}
	for _, id := range value.FindingIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("change remediation: finding id is required")
		}
	}
	return nil
}

func findingIDSet(values []auditcore.Finding) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, finding := range values {
		if id := strings.TrimSpace(finding.ID); id != "" {
			result[id] = struct{}{}
		}
	}
	return result
}
