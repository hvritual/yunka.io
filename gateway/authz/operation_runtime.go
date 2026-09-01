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

// GrantChecker is the legacy tenant-bound grant seam. It is intentionally
// tenant-specific and cannot authorize permission-bearing Operations whose
// Policy does not require a trusted tenant context.
type GrantChecker interface {
	ResolveGrants(context.Context, string, []string, []PermissionKey) ([]Grant, error)
}

// GrantRequest is the canonical permission-resolution input. Principal and
// Operation are trusted runtime facts; TenantBound records whether the Policy
// requires authorization to be evaluated inside the Principal tenant boundary.
type GrantRequest struct {
	Principal   identity.Principal
	Operation   OperationID
	Permissions []PermissionKey
	TenantBound bool
}

// GrantResolver is the principal-aware IAM seam used by the canonical grant
// authorizer. Implementations own the authority model for tenant-bound and
// non-tenant-bound principals; the framework does not infer authority from
// permission prefixes, role names, package names, or routes.
type GrantResolver interface {
	ResolveGrants(context.Context, GrantRequest) ([]Grant, error)
}

var (
	// ErrGrantResolverUnavailable means the configured resolver cannot safely
	// resolve the requested authority boundary. The authorizer maps it to a
	// fail-closed Decision rather than silently falling back to another scope.
	ErrGrantResolverUnavailable = errors.New("gateway authz: grant resolver unavailable")
	errTenantGrantRolesRequired  = errors.New("gateway authz: tenant grant roles required")
)

type tenantBoundGrantResolver struct{ checker GrantChecker }

// NewTenantBoundGrantResolver adapts the legacy GrantChecker into the new
// principal-aware seam. It deliberately fails closed for non-tenant-bound
// permission resolution so an empty TenantID never becomes a hidden platform
// or global-authority protocol.
func NewTenantBoundGrantResolver(checker GrantChecker) (GrantResolver, error) {
	if checker == nil {
		return nil, errors.New("gateway authz: grant checker is required")
	}
	return tenantBoundGrantResolver{checker: checker}, nil
}

func (resolver tenantBoundGrantResolver) ResolveGrants(ctx context.Context, request GrantRequest) ([]Grant, error) {
	if !request.TenantBound {
		return nil, ErrGrantResolverUnavailable
	}
	if strings.TrimSpace(request.Principal.TenantID) == "" {
		return nil, ErrGrantResolverUnavailable
	}
	if len(request.Principal.Roles) == 0 {
		return nil, errTenantGrantRolesRequired
	}
	return resolver.checker.ResolveGrants(
		ctx,
		request.Principal.TenantID,
		request.Principal.Roles,
		request.Permissions,
	)
}

type GrantAuthorizer struct{ resolver GrantResolver }

// NewGrantAuthorizer preserves the existing tenant-bound constructor and
// behavior. Consumers that need permission authorization without a tenant
// boundary must opt into NewGrantAuthorizerWithResolver with an explicit
// principal-aware GrantResolver.
func NewGrantAuthorizer(checker GrantChecker) (*GrantAuthorizer, error) {
	resolver, err := NewTenantBoundGrantResolver(checker)
	if err != nil {
		return nil, err
	}
	return NewGrantAuthorizerWithResolver(resolver)
}

func NewGrantAuthorizerWithResolver(resolver GrantResolver) (*GrantAuthorizer, error) {
	if resolver == nil {
		return nil, errors.New("gateway authz: grant resolver is required")
	}
	return &GrantAuthorizer{resolver: resolver}, nil
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
	if a == nil || a.resolver == nil {
		decision.Reason = ReasonGrantResolverUnavailable
		return decision, nil
	}
	grants, err := a.resolver.ResolveGrants(ctx, GrantRequest{
		Principal:   principal,
		Operation:   policy.Operation,
		Permissions: append([]PermissionKey(nil), policy.Permissions...),
		TenantBound: policy.TenantRequired,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrGrantResolverUnavailable):
			decision.Reason = ReasonGrantResolverUnavailable
			return decision, nil
		case errors.Is(err, errTenantGrantRolesRequired):
			decision.Reason = ReasonRoleRequired
			return decision, nil
		default:
			return decision, fmt.Errorf("gateway authz: grant resolution: %w", err)
		}
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
