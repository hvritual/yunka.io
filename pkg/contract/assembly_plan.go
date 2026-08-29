package contract

import (
	"fmt"
	"sort"
	"strings"

	"yunka.io/pkg/assemblyplan"
	"yunka.io/pkg/operationplan"
)

// CompileAssemblyPlan projects existing canonical Contract/OperationPlan facts
// plus a qualified static module descriptor snapshot into the C10 leaf-safe
// AssemblyPlan IR. It does not invent transport bindings or business behavior.
// A nil module slice is rejected so callers cannot accidentally publish a
// partial plan. Pass an explicit empty slice only for an intentionally empty
// resolved catalog.
func CompileAssemblyPlan(manifest Manifest, operations operationplan.Set, modules []assemblyplan.ModuleInput) (assemblyplan.Plan, error) {
	if modules == nil {
		return assemblyplan.Plan{}, fmt.Errorf("contract assembly plan: qualified module snapshot is required; pass an explicit empty slice only for an intentionally empty catalog")
	}
	manifest.Normalize()
	operations = operationplan.Normalize(operations)
	if err := operationplan.Validate(operations); err != nil {
		return assemblyplan.Plan{}, fmt.Errorf("contract assembly plan: operation plan: %w", err)
	}

	applications := map[string]assemblyplan.ApplicationInput{}
	for _, service := range manifest.Services {
		if service.Application == nil {
			continue
		}
		domain := strings.TrimSpace(service.Domain)
		name := strings.TrimSpace(service.Application.Name)
		if domain == "" || name == "" {
			return assemblyplan.Plan{}, fmt.Errorf("contract assembly plan: typed application %s requires domain and application name", service.FullName)
		}
		id := domain + "/" + name
		candidate := assemblyplan.ApplicationInput{
			ID:        id,
			Domain:    domain,
			Name:      name,
			DependsOn: append([]string(nil), service.Application.Requires...),
			Evidence: assemblyplan.Evidence{
				Ownership: assemblyplan.OwnershipReused,
				Source:    ManifestFilename,
				Ref:       "applications/" + id,
			},
		}
		if current, exists := applications[id]; exists {
			if strings.Join(current.DependsOn, "\x00") != strings.Join(candidate.DependsOn, "\x00") {
				return assemblyplan.Plan{}, fmt.Errorf("contract assembly plan: application %s has inconsistent dependency declarations", id)
			}
			continue
		}
		applications[id] = candidate
	}

	input := assemblyplan.Input{Identity: assemblyplan.RootTarget, Modules: append([]assemblyplan.ModuleInput(nil), modules...)}
	applicationIDs := make([]string, 0, len(applications))
	for id := range applications {
		applicationIDs = append(applicationIDs, id)
	}
	sort.Strings(applicationIDs)
	for _, id := range applicationIDs {
		input.Applications = append(input.Applications, applications[id])
	}

	for _, operation := range operations.Operations {
		applicationID := strings.TrimSpace(operation.Domain) + "/" + strings.TrimSpace(operation.Application)
		item := assemblyplan.OperationInput{
			ID:                 operation.OperationID,
			Application:        applicationID,
			RequiresOperations: append([]string(nil), operation.Composition.RequiresOperations...),
			Evidence: assemblyplan.Evidence{
				Ownership: assemblyplan.OwnershipReused,
				Source:    OperationPlansFilename,
				Ref:       "operations/" + operation.OperationID,
			},
		}
		if strings.TrimSpace(operation.Bindings.RPC) != "" {
			item.Bindings = append(item.Bindings, assemblyplan.BindingInput{
				Transport: "rpc",
				Index:     0,
				Evidence: assemblyplan.Evidence{
					Ownership: assemblyplan.OwnershipReused,
					Source:    OperationPlansFilename,
					Ref:       "operations/" + operation.OperationID + "/bindings/rpc",
				},
			})
		}
		for index := range operation.Bindings.HTTP {
			item.Bindings = append(item.Bindings, assemblyplan.BindingInput{
				Transport: "http",
				Index:     index,
				Evidence: assemblyplan.Evidence{
					Ownership: assemblyplan.OwnershipReused,
					Source:    OperationPlansFilename,
					Ref:       fmt.Sprintf("operations/%s/bindings/http/%d", operation.OperationID, index),
				},
			})
		}
		input.Operations = append(input.Operations, item)
	}

	plan, err := assemblyplan.Compile(input)
	if err != nil {
		return assemblyplan.Plan{}, fmt.Errorf("contract assembly plan: %w", err)
	}
	return plan, nil
}
