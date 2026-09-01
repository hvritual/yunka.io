package authz

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"yunka.io/framework/core/identity"
)

type OperationID string
type PermissionKey string
type PermissionMode uint8

const (
	PermissionAll PermissionMode = iota
	PermissionAny
)

type Policy struct {
	Operation      OperationID
	Permissions    []PermissionKey
	Mode           PermissionMode
	TenantRequired bool
	Authentication []string
}

func (policy Policy) Protected() bool {
	policy = Normalize(policy)
	return len(policy.Authentication) > 0 || len(policy.Permissions) > 0 || policy.TenantRequired
}

func (policy Policy) AcceptsAuthentication(method string) bool {
	policy = Normalize(policy)
	return containsString(policy.Authentication, strings.TrimSpace(method))
}

type Reason string

const (
	ReasonAllowed                  Reason = "allowed"
	ReasonUnauthenticated          Reason = "unauthenticated"
	ReasonTenantRequired           Reason = "tenant_required"
	ReasonRoleRequired             Reason = "role_required"
	ReasonPermissionDenied         Reason = "permission_denied"
	ReasonAuthenticationMethod     Reason = "authentication_method_denied"
	ReasonGrantResolverUnavailable Reason = "grant_resolver_unavailable"
)

type Decision struct {
	Allowed     bool
	Operation   OperationID
	Permissions []PermissionKey
	Grants      []Grant
	Reason      Reason
}

type PermissionChecker interface {
	HasPermissions(context.Context, string, []string, []PermissionKey, PermissionMode) (bool, error)
}

type Authorizer interface {
	Authorize(context.Context, identity.Principal, Policy) (Decision, error)
}

type RBACAuthorizer struct{ checker PermissionChecker }

func NewRBACAuthorizer(checker PermissionChecker) (*RBACAuthorizer, error) {
	if checker == nil {
		return nil, errors.New("gateway authz: permission checker is required")
	}
	return &RBACAuthorizer{checker: checker}, nil
}

func (a *RBACAuthorizer) Authorize(ctx context.Context, principal identity.Principal, policy Policy) (Decision, error) {
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
	allowed, err := a.checker.HasPermissions(ctx, principal.TenantID, principal.Roles, policy.Permissions, policy.Mode)
	if err != nil {
		return decision, fmt.Errorf("gateway authz: permission check: %w", err)
	}
	decision.Allowed = allowed
	if allowed {
		decision.Reason = ReasonAllowed
	} else {
		decision.Reason = ReasonPermissionDenied
	}
	return decision, nil
}

func Normalize(policy Policy) Policy {
	policy.Operation = OperationID(strings.TrimSpace(string(policy.Operation)))
	seen := map[PermissionKey]struct{}{}
	permissions := make([]PermissionKey, 0, len(policy.Permissions))
	for _, permission := range policy.Permissions {
		permission = PermissionKey(strings.TrimSpace(string(permission)))
		if permission == "" {
			continue
		}
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool { return permissions[i] < permissions[j] })
	policy.Permissions = permissions
	auth := make([]string, 0, len(policy.Authentication))
	seenAuth := map[string]struct{}{}
	for _, method := range policy.Authentication {
		method = strings.TrimSpace(method)
		if method == "" {
			continue
		}
		if _, exists := seenAuth[method]; exists {
			continue
		}
		seenAuth[method] = struct{}{}
		auth = append(auth, method)
	}
	sort.Strings(auth)
	policy.Authentication = auth
	return policy
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
