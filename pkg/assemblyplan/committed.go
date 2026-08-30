package assemblyplan

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	applications := make(map[string]Evidence, len(plan.Applications))
	for _, item := range plan.Applications {
		if err := check(item.Evidence, "application "+item.ID); err != nil {
			return err
		}
		applications[item.ID] = item.Evidence
	}
	for _, item := range plan.ApplicationDependencies {
		if err := check(item.Evidence, "application dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
		owner, ok := applications[item.From]
		if !ok {
			continue
		}
		if err := requireReusedEvidence(item.Evidence, owner.Source, owner.Ref+"/requires/"+item.To, "application dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	for _, item := range plan.ApplicationDependencyClosure {
		if err := check(item.Evidence, "application dependency closure "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	operations := make(map[string]Evidence, len(plan.Operations))
	for _, item := range plan.Operations {
		if err := check(item.Evidence, "operation "+item.ID); err != nil {
			return err
		}
		operations[item.ID] = item.Evidence
		for _, binding := range item.Bindings {
			owner := "binding " + item.ID + "/" + binding.Transport + "/" + strconv.Itoa(binding.Index)
			if err := check(binding.Evidence, owner); err != nil {
				return err
			}
			ref := item.Evidence.Ref + "/bindings/" + binding.Transport
			if binding.Transport == "http" {
				ref += "/" + strconv.Itoa(binding.Index)
			}
			if err := requireReusedEvidence(binding.Evidence, item.Evidence.Source, ref, owner); err != nil {
				return err
			}
		}
	}
	for _, item := range plan.OperationDependencies {
		if err := check(item.Evidence, "operation dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
		owner, ok := operations[item.From]
		if !ok {
			continue
		}
		if err := requireReusedEvidence(item.Evidence, owner.Source, owner.Ref+"/requiresOperations/"+item.To, "operation dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
	}
	modules := make(map[string]Evidence, len(plan.Modules))
	for _, item := range plan.Modules {
		if err := check(item.Evidence, "module "+item.Name); err != nil {
			return err
		}
		modules[item.Name] = item.Evidence
	}
	for _, item := range plan.ModuleDependencies {
		if err := check(item.Evidence, "module dependency "+item.From+" -> "+item.To); err != nil {
			return err
		}
		owner, ok := modules[item.From]
		if !ok {
			continue
		}
		if err := requireReusedEvidence(item.Evidence, owner.Source, owner.Ref+"/dependsOn/"+item.To, "module dependency "+item.From+" -> "+item.To); err != nil {
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

func requireReusedEvidence(evidence Evidence, source, ref, owner string) error {
	if evidence.Ownership != OwnershipReused || evidence.Source != source || evidence.Ref != ref {
		return fmt.Errorf("assemblyplan: %s has stale or inconsistent canonical evidence", owner)
	}
	return nil
}
