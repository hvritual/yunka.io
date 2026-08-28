package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"yunka.io/framework/core/identity"
	frameworkoperation "yunka.io/framework/operation"
	"yunka.io/pkg/operationplan"
)

var (
	ErrExecutionSecurityUnavailable = errors.New("gateway authz: execution security unavailable")
	ErrExecutionAdapterUnsupported  = errors.New("gateway authz: unsupported execution adapter")
)

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

// ExecutorFromOperationRuntime is a bounded C8 compatibility adapter. It lets
// generated C9 transports enter the unified Executor while an older caller
// still supplies OperationRuntime. Policy lookup remains keyed by the canonical
// RPC binding only inside this compatibility seam and must not be used by new
// composition code.
func ExecutorFromOperationRuntime(runtime OperationRuntime) (frameworkoperation.Executor, error) {
	if runtime == nil {
		return nil, ErrOperationRuntimeUnavailable
	}
	return frameworkoperation.NewExecutor(operationRuntimeSecurity{runtime: runtime}), nil
}

type operationRuntimeSecurity struct{ runtime OperationRuntime }

func (security operationRuntimeSecurity) Prepare(ctx context.Context, plan operationplan.Plan, input any) (context.Context, error) {
	if security.runtime == nil {
		return nil, ErrOperationRuntimeUnavailable
	}
	binding := strings.TrimSpace(plan.Bindings.RPC)
	if binding == "" {
		return nil, fmt.Errorf("gateway authz: operation %s has no canonical RPC binding", plan.OperationID)
	}
	return security.runtime.Prepare(ctx, binding, input)
}

// PreauthorizedExecutor is a compatibility path for gRPC servers that still
// run the C8 SecuredUnaryServerInterceptor. The interceptor performs the only
// authorization decision; the Executor verifies the resulting operation marker
// and never authorizes a second time.
func PreauthorizedExecutor() frameworkoperation.Executor {
	return frameworkoperation.NewExecutor(preauthorizedSecurity{})
}

type preauthorizedSecurity struct{}

func (preauthorizedSecurity) Prepare(ctx context.Context, plan operationplan.Plan, _ any) (context.Context, error) {
	if _, err := RequireAuthorizedOperation(ctx, OperationID(strings.TrimSpace(plan.OperationID))); err != nil {
		return nil, err
	}
	return ctx, nil
}

// ResolveExecutor accepts the canonical C9 Executor or the bounded C8
// OperationRuntime compatibility input used by generated REST registration.
func ResolveExecutor(value any) (frameworkoperation.Executor, error) {
	switch runtime := value.(type) {
	case frameworkoperation.Executor:
		if runtime == nil {
			return nil, frameworkoperation.ErrExecutorUnavailable
		}
		return runtime, nil
	case OperationRuntime:
		return ExecutorFromOperationRuntime(runtime)
	default:
		return nil, fmt.Errorf("%w: %T", ErrExecutionAdapterUnsupported, value)
	}
}
