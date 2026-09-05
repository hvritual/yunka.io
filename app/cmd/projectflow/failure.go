package projectflow

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

type FailureKind string

const (
	FailureProject        FailureKind = "project"
	FailureContract       FailureKind = "contract"
	FailureContractDrift  FailureKind = "contract_drift"
	FailureModule         FailureKind = "module"
	FailureModuleIdentity FailureKind = "module_identity"
	FailureAssembly       FailureKind = "assembly"
	FailureAssemblyDrift  FailureKind = "assembly_drift"
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
		item := diagnostic.MustDefinition(diagnostic.CodeDeveloperWorkflowFailed).Diagnostic(diagnostic.SeverityError)
		item.Detail = sanitizeDetail("", err)
		return item
	}

	definition := diagnostic.MustDefinition(failureDefinitionCode(failure.Kind))
	item := definition.Diagnostic(diagnostic.SeverityError)
	item.Detail = sanitizeDetail(failure.Root, failure.Err)
	if failure.Location != "" {
		item.Location = &diagnostic.Location{Path: failure.Location}
	}
	return item
}

func failureDefinitionCode(kind FailureKind) string {
	switch kind {
	case FailureProject:
		return diagnostic.CodeProjectResolve
	case FailureContract:
		return diagnostic.CodeContractFailure
	case FailureContractDrift:
		return diagnostic.CodeContractDrift
	case FailureModule:
		return diagnostic.CodeModuleFailure
	case FailureModuleIdentity:
		return diagnostic.CodeModuleIdentity
	case FailureAssembly:
		return diagnostic.CodeAssemblyFailure
	case FailureAssemblyDrift:
		return diagnostic.CodeAssemblyDrift
	default:
		return diagnostic.CodeDeveloperWorkflowFailed
	}
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
