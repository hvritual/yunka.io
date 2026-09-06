package projectflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

// OperationContractContext is a read-only project-relative projection of the
// canonical compiler's source evidence. It does not grant mutation authority.
type OperationContractContext struct {
	OperationID     string   `json:"operationId"`
	Service         string   `json:"service"`
	SourceFiles     []string `json:"sourceFiles"`
	ExternalImports []string `json:"externalImports,omitempty"`
}

func DescribeOperationContractContext(ctx context.Context, options Options, operationID string) (OperationContractContext, error) {
	project, err := resolveProject(options)
	if err != nil {
		return OperationContractContext{}, err
	}
	result, err := compileContract(ctx, project)
	if err != nil {
		return OperationContractContext{}, fmt.Errorf("contract context: compile canonical contract: %w", err)
	}
	value, err := contractcore.ResolveOperationContractContext(result.Manifest, operationID)
	if err != nil {
		return OperationContractContext{}, err
	}
	return projectContractContext(project, value)
}

func DescribeOperationContractContexts(ctx context.Context, options Options) ([]OperationContractContext, error) {
	project, err := resolveProject(options)
	if err != nil {
		return nil, err
	}
	result, err := compileContract(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("contract contexts: compile canonical contract: %w", err)
	}
	values, err := contractcore.OperationContractContexts(result.Manifest)
	if err != nil {
		return nil, err
	}
	contexts := make([]OperationContractContext, 0, len(values))
	for _, value := range values {
		item, err := projectContractContext(project, value)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, item)
	}
	return contexts, nil
}

func projectContractContext(project resolvedProject, value contractcore.OperationContractContext) (OperationContractContext, error) {
	paths := make([]string, 0, len(value.SourceFiles))
	for _, source := range value.SourceFiles {
		absolute, err := sourcePathForProject(project, source)
		if err != nil {
			return OperationContractContext{}, err
		}
		paths = append(paths, relative(project.Root, absolute))
	}
	sort.Strings(paths)
	return OperationContractContext{OperationID: value.OperationID, Service: value.Service, SourceFiles: paths, ExternalImports: append([]string(nil), value.ExternalImports...)}, nil
}

func sourcePathForProject(project resolvedProject, source string) (string, error) {
	if source == "" || source == "." || strings.ContainsAny(source, "\\:\x00") || filepath.IsAbs(source) || filepath.ToSlash(filepath.Clean(source)) != source || source == ".." || strings.HasPrefix(source, "../") {
		return "", fmt.Errorf("contract context: invalid source path %q", source)
	}
	// CompileInventory already rebases every declaration and import to the
	// project namespace. Plain Compile uses its exact proto root. No fallback.
	base := project.Root
	if project.InventoryPath == "" {
		base = project.ProtoDir
	}
	candidate := filepath.Join(base, filepath.FromSlash(source))
	physical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("contract context: resolve source %s: %w", source, err)
	}
	root, err := filepath.EvalSymlinks(project.Root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, physical)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("contract context: source %s escapes project", source)
	}
	info, err := os.Stat(physical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("contract context: source %s is not a regular file", source)
	}
	return candidate, nil
}
