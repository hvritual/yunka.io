package contract

import (
	"fmt"
	"sort"
	"strings"

	"yunka.io/pkg/operationplan"
)

type operationPlanOwner struct {
	plan           operationplan.Plan
	applicationKey string
}

func CompileOperationPlans(manifest Manifest) (operationplan.Set, error) {
	manifest.Normalize()
	applications := make(map[string]Service)
	appDeps := make(map[string][]string)
	for _, service := range manifest.Services {
		if service.Application == nil {
			continue
		}
		if strings.TrimSpace(service.Domain) == "" || strings.TrimSpace(service.Application.Name) == "" {
			return operationplan.Set{}, fmt.Errorf("contract operation plan: typed application %s requires domain and application name", service.FullName)
		}
		key := service.Domain + "/" + service.Application.Name
		if owner, exists := applications[key]; exists && owner.FullName != service.FullName {
			return operationplan.Set{}, fmt.Errorf("contract operation plan: duplicate application capability %s", key)
		}
		applications[key] = service
	}
	for key, service := range applications {
		deps := stableStrings(service.Application.Requires)
		for _, dependency := range deps {
			if !validApplicationDependency(dependency) {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: application %s has invalid dependency %s", key, dependency)
			}
			if dependency == key {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: application %s cannot depend on itself", key)
			}
			if _, ok := applications[dependency]; !ok {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: application %s requires unknown application %s", key, dependency)
			}
		}
		appDeps[key] = deps
	}
	if cycles := dependencyCycles(appDeps); len(cycles) > 0 {
		return operationplan.Set{}, fmt.Errorf("contract operation plan: application dependency cycle: %s", strings.Join(cycles[0], " -> "))
	}

	owners := make(map[string]operationPlanOwner)
	for _, service := range manifest.Services {
		if service.Application == nil {
			for _, method := range service.Methods {
				if method.Operation != nil {
					return operationplan.Set{}, fmt.Errorf("contract operation plan: operation %s belongs to service without application declaration", method.Operation.ID)
				}
			}
			continue
		}
		applicationKey := service.Domain + "/" + service.Application.Name
		for _, method := range service.Methods {
			if method.Operation == nil {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: typed application method %s has no operation", method.FullName)
			}
			operation := method.Operation
			id := strings.TrimSpace(operation.ID)
			if id == "" {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: method %s has empty operation id", method.FullName)
			}
			if owner, exists := owners[id]; exists {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: duplicate operation id %s (%s and %s)", id, owner.plan.Bindings.RPC, method.FullName)
			}
			httpBindings := make([]operationplan.HTTPBinding, 0, len(method.HTTP))
			for _, binding := range method.HTTP {
				httpBindings = append(httpBindings, operationplan.HTTPBinding{
					Method:       binding.Method,
					Path:         binding.Path,
					Body:         binding.Body,
					ResponseBody: binding.ResponseBody,
				})
			}
			plan := operationplan.Plan{
				OperationID:  id,
				Domain:       service.Domain,
				Application:  service.Application.Name,
				UseCase:      operation.UseCase,
				RequestType:  method.Request,
				ResponseType: method.Response,
				Security: operationplan.Security{
					Public:         operation.Public,
					TenantRequired: operation.TenantRequired,
					Authentication: append([]string(nil), operation.Authentication...),
					Permissions:    append([]string(nil), operation.Permissions...),
					PermissionMode: operation.PermissionMode,
				},
				Composition: operationplan.Composition{
					Boundary:           operation.Composition,
					RequiresOperations: append([]string(nil), operation.RequiresOperations...),
				},
				ApplicationRequires: append([]string(nil), service.Application.Requires...),
				Bindings: operationplan.Bindings{
					RPC:  "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name,
					HTTP: httpBindings,
				},
			}
			owners[id] = operationPlanOwner{plan: plan, applicationKey: applicationKey}
		}
	}

	for id, owner := range owners {
		declaredApplications := make(map[string]struct{}, len(owner.plan.ApplicationRequires))
		for _, dependency := range owner.plan.ApplicationRequires {
			declaredApplications[dependency] = struct{}{}
		}
		for _, required := range owner.plan.Composition.RequiresOperations {
			target, ok := owners[required]
			if !ok {
				return operationplan.Set{}, fmt.Errorf("contract operation plan: operation %s requires unknown operation %s", id, required)
			}
			if target.applicationKey != owner.applicationKey {
				if _, ok := declaredApplications[target.applicationKey]; !ok {
					return operationplan.Set{}, fmt.Errorf("contract operation plan: operation %s requires undeclared application capability %s", id, target.applicationKey)
				}
			}
		}
	}

	set := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: make([]operationplan.Plan, 0, len(owners))}
	ids := make([]string, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		owner := owners[id]
		closure := make(map[string]struct{})
		collectOperationPlanPermissions(id, owners, map[string]bool{}, closure)
		owner.plan.Composition.PermissionClosure = make([]string, 0, len(closure))
		for permission := range closure {
			owner.plan.Composition.PermissionClosure = append(owner.plan.Composition.PermissionClosure, permission)
		}
		sort.Strings(owner.plan.Composition.PermissionClosure)
		set.Operations = append(set.Operations, owner.plan)
	}
	set = operationplan.Normalize(set)
	if err := operationplan.Validate(set); err != nil {
		return operationplan.Set{}, fmt.Errorf("contract operation plan: %w", err)
	}
	return set, nil
}

func collectOperationPlanPermissions(id string, owners map[string]operationPlanOwner, active map[string]bool, result map[string]struct{}) {
	if active[id] {
		return
	}
	active[id] = true
	owner, ok := owners[id]
	if !ok {
		delete(active, id)
		return
	}
	for _, required := range owner.plan.Composition.RequiresOperations {
		target, ok := owners[required]
		if !ok {
			continue
		}
		for _, permission := range target.plan.Security.Permissions {
			result[permission] = struct{}{}
		}
		collectOperationPlanPermissions(required, owners, active, result)
	}
	delete(active, id)
}
