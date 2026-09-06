package projectflow

import (
	"context"
	"fmt"
	"path/filepath"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

// OperationContractContext is a project-relative projection of canonical
// protobuf provenance for one Operation. It is derived on demand from the
// canonical compiler result and is not persisted as another source of truth.
type OperationContractContext struct {
	OperationID string   `json:"operationId"`
	Service     string   `json:"service"`
	SourceFiles []string `json:"sourceFiles"`
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
	contextValue, err := contractcore.ResolveOperationContractContext(result.Manifest, operationID)
	if err != nil {
		return OperationContractContext{}, err
	}
	paths := make([]string, 0, len(contextValue.SourceFiles))
	for _, source := range contextValue.SourceFiles {
		absolute := sourcePathForProject(project, source)
		paths = append(paths, relative(project.Root, absolute))
	}
	return OperationContractContext{OperationID: contextValue.OperationID, Service: contextValue.Service, SourceFiles: paths}, nil
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
		paths := make([]string, 0, len(value.SourceFiles))
		for _, source := range value.SourceFiles {
			paths = append(paths, relative(project.Root, sourcePathForProject(project, source)))
		}
		contexts = append(contexts, OperationContractContext{OperationID: value.OperationID, Service: value.Service, SourceFiles: paths})
	}
	return contexts, nil
}

func sourcePathForProject(project resolvedProject, source string) string {
	if project.InventoryPath == "" {
		return filepath.Join(project.ProtoDir, filepath.FromSlash(source))
	}
	// Inventory compilation preserves descriptor file names relative to each
	// source-set root. For project-level provenance, resolve against the project
	// root when the source already names a project-relative path; otherwise fall
	// back to the inventory directory. Multi-root inventories that cannot be
	// uniquely resolved remain a future pressure case rather than inventing a
	// hand-maintained ownership taxonomy here.
	candidate := filepath.Join(project.Root, filepath.FromSlash(source))
	if _, err := filepath.Abs(candidate); err == nil {
		return candidate
	}
	return filepath.Join(filepath.Dir(project.InventoryPath), filepath.FromSlash(source))
}
