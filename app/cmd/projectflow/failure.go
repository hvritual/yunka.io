package projectflow

import (
	"errors"
	"path/filepath"
	"strings"

	"yunka.io/pkg/diagnostic"
)

type FailureKind string

const (
	FailureProject       FailureKind = "project"
	FailureContract      FailureKind = "contract"
	FailureContractDrift FailureKind = "contract_drift"
	FailureModule        FailureKind = "module"
	FailureAssembly      FailureKind = "assembly"
	FailureAssemblyDrift FailureKind = "assembly_drift"
)

type Failure struct {
	Kind     FailureKind
	Root     string
	Location string
	Err      error
}

func (failure *Failure) Error() string {
	if failure == nil || failure.Err == nil {
		return "projectflow failure"
	}
	return failure.Err.Error()
}

func (failure *Failure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Err
}

func wrapFailure(kind FailureKind, root, location string, err error) error {
	if err == nil {
		return nil
	}
	return &Failure{
		Kind:     kind,
		Root:     strings.TrimSpace(root),
		Location: cleanLocation(location),
		Err:      err,
	}
}

func Diagnose(err error) diagnostic.Diagnostic {
	var failure *Failure
	if !errors.As(err, &failure) {
		return diagnostic.Diagnostic{
			Code:     "YUNKA-DX-DEV-999",
			Severity: diagnostic.SeverityError,
			Stage:    "developer-workflow",
			Summary:  "developer workflow failed",
			Detail:   sanitizeDetail("", err),
		}
	}

	item := diagnostic.Diagnostic{
		Severity: diagnostic.SeverityError,
		Detail:   sanitizeDetail(failure.Root, failure.Err),
	}
	if failure.Location != "" {
		item.Location = &diagnostic.Location{Path: failure.Location}
	}

	switch failure.Kind {
	case FailureProject:
		item.Code = "YUNKA-DX-PROJECT-001"
		item.Stage = "project"
		item.Summary = "project configuration could not be resolved"
		item.Actions = []diagnostic.Action{{
			Kind:  diagnostic.ActionEdit,
			Label: "Review project profile",
			Value: ".yunka/project.json",
		}}
	case FailureContract:
		item.Code = "YUNKA-DX-CONTRACT-001"
		item.Stage = "contract"
		item.Summary = "contract generation or validation failed"
	case FailureContractDrift:
		item.Code = "YUNKA-DX-CONTRACT-002"
		item.Stage = "contract"
		item.Summary = "generated contract artifacts are stale"
		item.Actions = []diagnostic.Action{{
			Kind:  diagnostic.ActionCommand,
			Label: "Regenerate",
			Value: "yunka generate",
		}}
	case FailureModule:
		item.Code = "YUNKA-DX-MODULE-001"
		item.Stage = "module"
		item.Summary = "module validation failed"
		item.Actions = []diagnostic.Action{{
			Kind:  diagnostic.ActionCommand,
			Label: "Inspect modules",
			Value: "yunka module check",
		}}
	case FailureAssembly:
		item.Code = "YUNKA-DX-ASSEMBLY-001"
		item.Stage = "assembly"
		item.Summary = "runtime assembly generation or validation failed"
	case FailureAssemblyDrift:
		item.Code = "YUNKA-DX-ASSEMBLY-002"
		item.Stage = "assembly"
		item.Summary = "generated runtime assembly artifacts are stale"
		item.Actions = []diagnostic.Action{{
			Kind:  diagnostic.ActionCommand,
			Label: "Regenerate",
			Value: "yunka generate",
		}}
	default:
		item.Code = "YUNKA-DX-DEV-999"
		item.Stage = "developer-workflow"
		item.Summary = "developer workflow failed"
	}
	return item
}

func cleanLocation(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	if filepath.IsAbs(filepath.FromSlash(value)) {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func sanitizeDetail(root string, err error) string {
	if err == nil {
		return ""
	}
	value := strings.TrimSpace(err.Error())
	root = strings.TrimSpace(root)
	if root != "" {
		root = filepath.Clean(root)
		for _, variant := range []string{root, filepath.ToSlash(root), strings.ReplaceAll(root, "/", "\\")} {
			if variant != "" {
				value = strings.ReplaceAll(value, variant, ".")
			}
		}
	}
	return value
}
