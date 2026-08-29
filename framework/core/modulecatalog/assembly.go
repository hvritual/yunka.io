package modulecatalog

import "yunka.io/pkg/assemblyplan"

// AssemblyInputs converts an already-resolved static module plan into the
// leaf-safe C10 AssemblyPlan input. Build functions and runtime providers are
// intentionally excluded; only immutable descriptor facts cross the boundary.
func AssemblyInputs(plan Plan) []assemblyplan.ModuleInput {
	result := make([]assemblyplan.ModuleInput, 0, len(plan.Descriptors))
	for _, descriptor := range plan.Descriptors {
		requirements := assemblyplan.ModuleRequirements{
			ConfigKey: descriptor.Requirements.ConfigKey,
			Logger:    descriptor.Requirements.Logger,
			EventBus:  descriptor.Requirements.EventBus,
		}
		for _, database := range descriptor.Requirements.Databases {
			requirements.Databases = append(requirements.Databases, database.Name)
		}
		for _, rpc := range descriptor.Requirements.RPC {
			requirements.RPC = append(requirements.RPC, rpc.Name)
		}
		result = append(result, assemblyplan.ModuleInput{
			Name:         descriptor.Name,
			Version:      descriptor.Version,
			DependsOn:    append([]string(nil), descriptor.DependsOn...),
			Requirements: requirements,
			Evidence: assemblyplan.Evidence{
				Ownership: assemblyplan.OwnershipReused,
				Source:    "modulecatalog",
				Ref:       "modules/" + descriptor.Name,
			},
		})
	}
	return result
}
