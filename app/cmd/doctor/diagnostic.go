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
	Stage      string
	Location   string
	ActionKind diagnostic.ActionKind
}

var doctorMappings = map[string]doctorMapping{
	"workspace.root":              {Code: "YUNKA-DX-PROJECT-101", Stage: "project", ActionKind: diagnostic.ActionCommand},
	"workspace.go_work":           {Code: "YUNKA-DX-TOOLCHAIN-101", Stage: "toolchain", Location: "go.work", ActionKind: diagnostic.ActionEdit},
	"toolchain.lock":              {Code: "YUNKA-DX-TOOLCHAIN-102", Stage: "toolchain", Location: "tools/toolchain.env", ActionKind: diagnostic.ActionEdit},
	"tool.go":                     {Code: "YUNKA-DX-TOOLCHAIN-110", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"tool.protoc":                 {Code: "YUNKA-DX-TOOLCHAIN-111", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"tool.protoc-gen-go":          {Code: "YUNKA-DX-TOOLCHAIN-112", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"tool.protoc-gen-go-grpc":     {Code: "YUNKA-DX-TOOLCHAIN-113", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"tool.gcc":                    {Code: "YUNKA-DX-TOOLCHAIN-114", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"tool.git":                    {Code: "YUNKA-DX-TOOLCHAIN-115", Stage: "toolchain", ActionKind: diagnostic.ActionCommand},
	"contract.manifest":           {Code: "YUNKA-DX-CONTRACT-101", Stage: "contract", Location: "contracts/generated/manifest.json", ActionKind: diagnostic.ActionCommand},
	"application_graph.contract": {Code: "YUNKA-DX-CONTRACT-102", Stage: "contract", Location: "contracts/generated/manifest.json", ActionKind: diagnostic.ActionEdit},
	"git.status":                  {Code: "YUNKA-DX-DEV-101", Stage: "developer-environment", ActionKind: diagnostic.ActionCommand},
	"dev.manifest":                {Code: "YUNKA-DX-DEV-102", Stage: "developer-environment", Location: ".yunka/dev.json", ActionKind: diagnostic.ActionEdit},
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
		severity, err := doctorSeverity(check.Status)
		if err != nil {
			return nil, fmt.Errorf("doctor diagnostics: %s: %w", check.Name, err)
		}
		item := diagnostic.Diagnostic{
			Code:     mapping.Code,
			Severity: severity,
			Stage:    mapping.Stage,
			Summary:  check.Name,
			Detail:   sanitizeDoctorDetail(report.Root, check.Detail),
		}
		if mapping.Location != "" {
			item.Location = &diagnostic.Location{Path: mapping.Location}
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
