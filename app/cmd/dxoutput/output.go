package dxoutput

import (
	"fmt"
	"strings"

	"yunka.io/app/cmd/projectflow"
	"yunka.io/pkg/diagnostic"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

type Result struct {
	Output   string
	ExitCode int
}

func Build(command, format string, report projectflow.Report, workflowErr error) (Result, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = FormatText
	}
	if format != FormatText && format != FormatJSON {
		item := diagnostic.Diagnostic{
			Code:     "YUNKA-DX-DEV-001",
			Severity: diagnostic.SeverityError,
			Stage:    "cli",
			Summary:  "unsupported output format",
			Detail:   fmt.Sprintf("format %q is unsupported; use text or json", format),
		}
		text, err := diagnostic.RenderText([]diagnostic.Diagnostic{item})
		if err != nil {
			return Result{}, err
		}
		return Result{Output: text, ExitCode: 2}, nil
	}

	if workflowErr != nil {
		item := projectflow.Diagnose(workflowErr)
		if format == FormatJSON {
			contents, err := diagnostic.RenderJSON(command, []diagnostic.Diagnostic{item})
			if err != nil {
				return Result{}, err
			}
			return Result{Output: string(contents), ExitCode: 1}, nil
		}
		text, err := diagnostic.RenderText([]diagnostic.Diagnostic{item})
		if err != nil {
			return Result{}, err
		}
		return Result{Output: text, ExitCode: 1}, nil
	}

	if format == FormatJSON {
		contents, err := projectflow.FormatJSON(command, report)
		if err != nil {
			return Result{}, err
		}
		return Result{Output: string(contents)}, nil
	}
	return Result{Output: projectflow.Format(report)}, nil
}
