package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"

	frameworkoperation "yunka.io/framework/operation"
	"yunka.io/framework/core/identity"
	"yunka.io/pkg/operationplan"
)

var ErrExecutionSecurityUnavailable = errors.New("gateway authz: execution security unavailable")

type executionSecurity struct {
	authorizer Authorizer
	guards     GuardResolver
}

func NewExecutionSecurity(authorizer Authorizer, guards GuardResolver) (frameworkoperation.SecurityPhase, error) {
	if authorizer == nil {
		return nil, errors.New("gateway authz: authorizer is required")
	}
	return &executionSecurity{authorizer: authorizer, guards: guards}, nil
}

func (security *executionSecurity) Prepare(ctx context.Context, plan operationplan.Plan, input any) (context.Context, error) {
	if security == nil || security.authorizer == nil {
		return nil, ErrExecutionSecurityUnavailable
	}
	policy := PolicyFromOperationPlan(plan)
	return prepareAuthorized(ctx, policy, input, security.authorizer, security.guards)
}

func PolicyFromOperationPlan(plan operationplan.Plan) Policy {
	permissions := make([]PermissionKey, 0, len(plan.Security.Permissions))
	for _, permission := range plan.Security.Permissions {
		if permission = strings.TrimSpace(permission); permission != "" {
			permissions = append(permissions, PermissionKey(permission))
		}
	}
	mode := PermissionAll
	if strings.EqualFold(strings.TrimSpace(plan.Security.PermissionMode), "any") {
		mode = PermissionAny
	}
	return Normalize(Policy{
		Operation:      OperationID(strings.TrimSpace(plan.OperationID)),
		Permissions:    permissions,
		Mode:           mode,
		TenantRequired: plan.Security.TenantRequired,
		Authentication: append([]string(nil), plan.Security.Authentication...),
	})
}

func prepareAuthorized(ctx context.Context, policy Policy, input any, authorizer Authorizer, guards GuardResolver) (context.Context, error) {
	if authorizer == nil {
		return nil, ErrExecutionSecurityUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	principal, _ := identity.FromContext(ctx)
	decision, err := authorizer.Authorize(ctx, principal, policy)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		return nil, Denied(decision)
	}
	authorized := AuthorizedOperation{Principal: principal, Policy: policy, Decision: decision}
	secured := WithAuthorizedOperation(ctx, authorized)
	if guards != nil {
		if guard, exists := guards.ResolveGuard(policy.Operation); exists {
			secured, err = guard.Prepare(secured, authorized, input)
			if err != nil {
				return nil, err
			}
			if secured == nil {
				return nil, fmt.Errorf("gateway authz: operation guard returned nil context for %s", policy.Operation)
			}
		}
	}
	return secured, nil
}
