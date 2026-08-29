package assemblyplan

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON enforces the committed-plan boundary. Runtime-local evidence is
// a classification available to assembly tooling, but it must never be
// serialized into the canonical committed AssemblyPlan artifact.
func (plan Plan) MarshalJSON() ([]byte, error) {
	if err := validateCommittedPlan(plan); err != nil {
		return nil, err
	}
	type planAlias Plan
	return json.Marshal(planAlias(plan))
}

func validateCommittedPlan(plan Plan) error {
	check := func(evidence Evidence, owner string) error {
		if evidence.Ownership == OwnershipRuntimeLocal {
			return fmt.Errorf("assemblyplan: runtime-local evidence cannot enter committed plan: %s", owner)
		}
		return nil
	}
	for _, item := range plan.Applications {
		if err := check(item.Evidence, "application "+item.ID); err != nil {
			return err
		}
	}
	for _, item := range plan.ApplicationDependencies {
		if err := check(item.Evidence, "application dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	for _, item := range plan.ApplicationDependencyClosure {
		if err := check(item.Evidence, "application dependency closure "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	for _, item := range plan.Operations {
		if err := check(item.Evidence, "operation "+item.ID); err != nil {
			return err
		}
		for _, binding := range item.Bindings {
			if err := check(binding.Evidence, "binding "+item.ID+"/"+binding.Transport); err != nil {
				return err
			}
		}
	}
	for _, item := range plan.OperationDependencies {
		if err := check(item.Evidence, "operation dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	for _, item := range plan.Modules {
		if err := check(item.Evidence, "module "+item.Name); err != nil {
			return err
		}
	}
	for _, item := range plan.ModuleDependencies {
		if err := check(item.Evidence, "module dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	for _, item := range plan.Requirements {
		if err := check(item.Evidence, "requirement "+item.Module+"/"+item.Kind); err != nil {
			return err
		}
	}
	for _, item := range plan.Targets {
		if err := check(item.Evidence, "target "+item.Name); err != nil {
			return err
		}
	}
	return nil
}
