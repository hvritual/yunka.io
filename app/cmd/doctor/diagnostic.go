package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"yunka.io/pkg/devruntime"
	"yunka.io/pkg/diagnostic"
)

const doctorDiagnosticSchemaVersion = 1

type doctorMapping struct {
	Code       string
	ActionKind diagnostic.ActionKind
}

var doctorMappings = map[string]doctorMapping{
	"workspace.root":              {Code: diagnostic.CodeDoctorWorkspaceRoot, ActionKind: diagnostic.ActionCommand},
	"workspace.go_work":           {Code: diagnostic.CodeDoctorGoWork, ActionKind: diagnostic.ActionEdit},
	"toolchain.lock":              {Code: diagnostic.CodeDoctorToolchainLock, ActionKind: diagnostic.ActionEdit},
	"tool.go":                     {Code: diagnostic.CodeDoctorGo, ActionKind: diagnostic.ActionCommand},
	"tool.protoc":                 {Code: diagnostic.CodeDoctorProtoc, ActionKind: diagnostic.ActionCommand},
	"tool.protoc-gen-go":          {Code: diagnostic.CodeDoctorProtocGenGo, ActionKind: diagnostic.ActionCommand},
	"tool.protoc-gen-go-grpc":     {Code: diagnostic.CodeDoctorProtocGenGoGRPC, ActionKind: diagnostic.ActionCommand},
	"tool.gcc":                    {Code: diagnostic.CodeDoctorGCC, ActionKind: diagnostic.ActionCommand},
	"tool.git":                    {Code: diagnostic.CodeDoctorGit, ActionKind: diagnostic.ActionCommand},
	"contract.manifest":           {Code: diagnostic.CodeDoctorContractManifest, ActionKind: diagnostic.ActionCommand},
	"application_graph.contract": {Code: diagnostic.CodeDoctorContractGraph, ActionKind: diagnostic.ActionEdit},
	"git.status":                  {Code: diagnostic.CodeDoctorGitStatus, ActionKind: diagnostic.ActionCommand},
	"dev.manifest":                {Code: diagnostic.CodeDoctorDevManifest, ActionKind: diagnostic.ActionEdit},
}

type doctorEnvelope struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Command       string                  `json:"command"`
	OK            bool                    `json:"ok"`
	Strict        bool                    `json:"strict"`
	Diagnostics  []diagnostic.Diagnostic `json:"diagnostics"`
}

func adaptDoctorReport(report devruntime.DoctorReport) ([]diagnostic.Diagnostic, error) {
	items := make([]diagnostic.Diagnostic, 0, len(report.Checks))
	for _, check := range report.Checks {
		mapping, ok := doctorMappings[check.Name]
		if !ok {
			return nil, fmt.Errorf("doctor diagnostics: unmapped check %q", check.Name)
		}
		definition, ok := diagnostic.LookupDefinition(mapping.Code)
		if !ok {
			return nil, fmt.Errorf("doctor diagnostics: code %q has no canonical definition", mapping.Code)
		}
		severity, err := doctorSeverity(check.Status)
		if err != nil {
			return nil, fmt.Errorf("doctor diagnostics: %s: %w", check.Name, err)
		}
		item := diagnostic.Diagnostic{
			Code:     definition.Code,
			Severity: severity,
			Stage:    definition.Stage,
			Summary:  check.Name,
			Detail:   sanitizeDoctorDetail(report.Root, check.Detail),
		}
		if definition.Location != "" {
			item.Location = &diagnostic.Location{Path: definition.Location}
		}
		if strings.TrimSpace(check.Action) != "" {
			item.Actions = []diagnostic.Action{{
				Kind:  mapping.ActionKind,
				Label: "Remediation",
				Value: sanitizeDoctorDetail(report.Root, check.Action),
			}}
		}
		items = append(items, item)
	}
	return diagnostic.Normalize(items)
}

func renderDoctorJSON(report devruntime.DoctorReport, strict bool) ([]byte, error) {
	items, err := adaptDoctorReport(report)
	if err != nil {
		return nil, err
	}
	envelope := doctorEnvelope{
		SchemaVersion: doctorDiagnosticSchemaVersion,
		Command:       "yunka doctor",
		OK:            !report.Failed(strict),
		Strict:        strict,
		Diagnostics:  items,
	}
	contents, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func renderDoctorText(report devruntime.DoctorReport) (string, error) {
	items, err := adaptDoctorReport(report)
	if err != nil {
		return "", err
	}
	return diagnostic.RenderText(items)
}

func doctorSeverity(status devruntime.CheckStatus) (diagnostic.Severity, error) {
	switch status {
	case devruntime.CheckPass:
		return diagnostic.SeverityInfo, nil
	case devruntime.CheckWarn:
		return diagnostic.SeverityWarning, nil
	case devruntime.CheckFail:
		return diagnostic.SeverityError, nil
	default:
		return "", errors.New("unsupported doctor check status")
	}
}

func sanitizeDoctorDetail(root, value string) string {
	value = strings.TrimSpace(value)
	root = strings.TrimSpace(root)
	if root == "" {
		return value
	}
	root = filepath.Clean(root)
	variants := []string{root, filepath.ToSlash(root), strings.ReplaceAll(root, "/", "\\")}
	for _, variant := range variants {
		if variant != "" {
			value = strings.ReplaceAll(value, variant, ".")
		}
	}
	return value
}
