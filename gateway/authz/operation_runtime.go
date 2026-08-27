package authz

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"yunka.io/framework/core/identity"
)

// Grant is an IAM-owned permission grant projected into the framework security runtime.
// Scope is opaque to the framework and is interpreted only by a domain OperationGuard.
type Grant struct {
	Permission PermissionKey
	RoleID     string
	Scope      string
}

// GrantChecker returns grants that actually authorize the requested permissions.
// Implementations MUST bind Scope to the same role/permission grant; unrelated role scope
// rows must never be returned.
type GrantChecker interface {
	ResolveGrants(context.Context, string, []string, []PermissionKey) ([]Grant, error)
}

type GrantAuthorizer struct{ checker GrantChecker }

func NewGrantAuthorizer(checker GrantChecker) (*GrantAuthorizer, error) {
	if checker == nil {
		return nil, errors.New("gateway authz: grant checker is required")
	}
	return &GrantAuthorizer{checker: checker}, nil
}

func (a *GrantAuthorizer) Authorize(ctx context.Context, principal identity.Principal, policy Policy) (Decision, error) {
	policy = Normalize(policy)
	decision := Decision{Operation: policy.Operation, Permissions: append([]PermissionKey(nil), policy.Permissions...)}
	if len(policy.Authentication) > 0 {
		if !principal.Authenticated {
			decision.Reason = ReasonUnauthenticated
			return decision, nil
		}
		if !containsString(policy.Authentication, principal.AuthMethod) {
			decision.Reason = ReasonAuthenticationMethod
			return decision, nil
		}
	}
	if len(policy.Permissions) > 0 && !principal.Authenticated {
		decision.Reason = ReasonUnauthenticated
		return decision, nil
	}
	if policy.TenantRequired && strings.TrimSpace(principal.TenantID) == "" {
		decision.Reason = ReasonTenantRequired
		return decision, nil
	}
	if len(policy.Permissions) == 0 {
		decision.Allowed, decision.Reason = true, ReasonAllowed
		return decision, nil
	}
	if strings.TrimSpace(principal.TenantID) == "" {
		decision.Reason = ReasonTenantRequired
		return decision, nil
	}
	if len(principal.Roles) == 0 {
		decision.Reason = ReasonRoleRequired
		return decision, nil
	}
	grants, err := a.checker.ResolveGrants(ctx, principal.TenantID, principal.Roles, policy.Permissions)
	if err != nil {
		return decision, fmt.Errorf("gateway authz: grant resolution: %w", err)
	}
	requested := make(map[PermissionKey]struct{}, len(policy.Permissions))
	for _, permission := range policy.Permissions {
		requested[permission] = struct{}{}
	}
	present := make(map[PermissionKey]struct{}, len(policy.Permissions))
	normalized := make([]Grant, 0, len(grants))
	for _, grant := range grants {
		grant.Permission = PermissionKey(strings.TrimSpace(string(grant.Permission)))
		grant.RoleID = strings.TrimSpace(grant.RoleID)
		grant.Scope = strings.TrimSpace(grant.Scope)
		if _, ok := requested[grant.Permission]; !ok {
			continue
		}
		present[grant.Permission] = struct{}{}
		normalized = append(normalized, grant)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Permission != normalized[j].Permission {
			return normalized[i].Permission < normalized[j].Permission
		}
		if normalized[i].RoleID != normalized[j].RoleID {
			return normalized[i].RoleID < normalized[j].RoleID
		}
		return normalized[i].Scope < normalized[j].Scope
	})
	decision.Grants = normalized
	if policy.Mode == PermissionAny {
		decision.Allowed = len(present) > 0
	} else {
		decision.Allowed = len(present) == len(policy.Permissions)
	}
	if decision.Allowed {
		decision.Reason = ReasonAllowed
	} else {
		decision.Reason = ReasonPermissionDenied
	}
	return decision, nil
}

// AuthorizedOperation is written by OperationRuntime immediately before the
// Application boundary. Application/domain code may inspect actor metadata for
// business/audit purposes, but MUST NOT repeat permission evaluation.
type AuthorizedOperation struct {
	Principal identity.Principal
	Policy    Policy
	Decision  Decision
}

type authorizedOperationKey struct{}

func WithAuthorizedOperation(ctx context.Context, value AuthorizedOperation) context.Context {
	return context.WithValue(ctx, authorizedOperationKey{}, value)
}

func AuthorizedOperationFromContext(ctx context.Context) (AuthorizedOperation, bool) {
	if ctx == nil {
		return AuthorizedOperation{}, false
	}
	value, ok := ctx.Value(authorizedOperationKey{}).(AuthorizedOperation)
	return value, ok
}

var (
	ErrOperationRuntimeUnavailable = errors.New("gateway authz: operation runtime unavailable")
	ErrOperationPolicyNotFound     = errors.New("gateway authz: operation policy not found")
	ErrAuthorizedOperationMissing  = errors.New("gateway authz: authorized operation missing")
)

// OperationGuard performs domain-specific resource-scope preparation after the
// IAM decision but before Application code is invoked. The framework does not
// interpret Grant.Scope.
type OperationGuard interface {
	Prepare(context.Context, AuthorizedOperation, any) (context.Context, error)
}

type GuardResolver interface {
	ResolveGuard(OperationID) (OperationGuard, bool)
}

type StaticGuardResolver map[OperationID]OperationGuard

func NewStaticGuardResolver(values map[OperationID]OperationGuard) StaticGuardResolver {
	result := make(StaticGuardResolver, len(values))
	for operation, guard := range values {
		operation = OperationID(strings.TrimSpace(string(operation)))
		if operation == "" || guard == nil {
			continue
		}
		result[operation] = guard
	}
	return result
}

func (resolver StaticGuardResolver) ResolveGuard(operation OperationID) (OperationGuard, bool) {
	guard, ok := resolver[OperationID(strings.TrimSpace(string(operation)))]
	return guard, ok
}

// OperationRuntime is the single pre-Application security boundary shared by
// REST and gRPC generated transports.
type OperationRuntime interface {
	Prepare(context.Context, string, any) (context.Context, error)
}

type operationRuntime struct {
	resolver   PolicyResolver
	authorizer Authorizer
	guards     GuardResolver
}

func NewOperationRuntime(resolver PolicyResolver, authorizer Authorizer, guards GuardResolver) (OperationRuntime, error) {
	if resolver == nil {
		return nil, errors.New("gateway authz: policy resolver is required")
	}
	if authorizer == nil {
		return nil, errors.New("gateway authz: authorizer is required")
	}
	return &operationRuntime{resolver: resolver, authorizer: authorizer, guards: guards}, nil
}

func (runtime *operationRuntime) Prepare(ctx context.Context, key string, input any) (context.Context, error) {
	if runtime == nil || runtime.resolver == nil || runtime.authorizer == nil {
		return nil, ErrOperationRuntimeUnavailable
	}
	policy, ok := runtime.resolver.ResolvePolicy(ctx, key)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrOperationPolicyNotFound, strings.TrimSpace(key))
	}
	principal, _ := identity.FromContext(ctx)
	decision, err := runtime.authorizer.Authorize(ctx, principal, policy)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		return nil, Denied(decision)
	}
	authorized := AuthorizedOperation{Principal: principal, Policy: policy, Decision: decision}
	secured := WithAuthorizedOperation(ctx, authorized)
	if runtime.guards != nil {
		if guard, exists := runtime.guards.ResolveGuard(policy.Operation); exists {
			secured, err = guard.Prepare(secured, authorized, input)
			if err != nil {
				return nil, err
			}
			if secured == nil {
				return nil, errors.New("gateway authz: operation guard returned nil context")
			}
		}
	}
	return secured, nil
}

func RequireAuthorizedOperation(ctx context.Context, operation OperationID) (AuthorizedOperation, error) {
	value, ok := AuthorizedOperationFromContext(ctx)
	if !ok || value.Decision.Allowed == false {
		return AuthorizedOperation{}, ErrAuthorizedOperationMissing
	}
	if operation != "" && value.Policy.Operation != operation {
		return AuthorizedOperation{}, fmt.Errorf("%w: expected=%s actual=%s", ErrAuthorizedOperationMissing, operation, value.Policy.Operation)
	}
	return value, nil
}
