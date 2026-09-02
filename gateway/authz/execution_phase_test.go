package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/identity"
	frameworkoperation "github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

type phaseChecker struct{ calls int }

func (checker *phaseChecker) HasPermissions(_ context.Context, tenant string, roles []string, permissions []PermissionKey, mode PermissionMode) (bool, error) {
	checker.calls++
	return tenant == "tenant-a" && len(roles) == 1 && roles[0] == "operator" && len(permissions) == 1 && permissions[0] == "device.read" && mode == PermissionAll, nil
}

type phaseGuard struct{ calls int }

func (guard *phaseGuard) Prepare(ctx context.Context, authorized AuthorizedOperation, _ any) (context.Context, error) {
	guard.calls++
	if authorized.Policy.Operation != "device.get" || !authorized.Decision.Allowed {
		return nil, errors.New("unexpected authorization")
	}
	return ctx, nil
}

func TestExecutionSecurityUsesStableOperationPlanIdentityOnce(t *testing.T) {
	checker := &phaseChecker{}
	authorizer, err := NewRBACAuthorizer(checker)
	if err != nil {
		t.Fatal(err)
	}
	guard := &phaseGuard{}
	security, err := NewExecutionSecurity(authorizer, NewStaticGuardResolver(map[OperationID]OperationGuard{"device.get": guard}))
	if err != nil {
		t.Fatal(err)
	}
	executor := frameworkoperation.NewExecutor(security)
	principal := identity.Principal{Subject: "user-1", TenantID: "tenant-a", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT, Authenticated: true}
	ctx := identity.WithPrincipal(context.Background(), principal)
	plan := operationplan.Plan{
		OperationID: "device.get",
		Security: operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.read"}, PermissionMode: "all"},
		Bindings: operationplan.Bindings{RPC: "/device.v1.DeviceApplication/GetDevice"},
	}
	called := 0
	_, err = executor.Execute(ctx, plan, nil, func(callContext context.Context) (any, error) {
		called++
		authorized, err := RequireAuthorizedOperation(callContext, "device.get")
		if err != nil {
			return nil, err
		}
		if authorized.Policy.Operation != "device.get" {
			t.Fatalf("operation=%s", authorized.Policy.Operation)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 || checker.calls != 1 || guard.calls != 1 {
		t.Fatalf("application=%d authorizer=%d guard=%d", called, checker.calls, guard.calls)
	}
}

func TestPreauthorizedExecutorDoesNotAuthorizeAgain(t *testing.T) {
	plan := operationplan.Plan{OperationID: "device.get", Security: operationplan.Security{Permissions: []string{"device.read"}, PermissionMode: "all"}, Bindings: operationplan.Bindings{RPC: "/device.v1.DeviceApplication/GetDevice"}}
	policy := PolicyFromOperationPlan(plan)
	ctx := WithAuthorizedOperation(context.Background(), AuthorizedOperation{Policy: policy, Decision: Decision{Allowed: true, Operation: policy.Operation}})
	called := 0
	_, err := PreauthorizedExecutor().Execute(ctx, plan, nil, func(context.Context) (any, error) {
		called++
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("calls=%d", called)
	}
}

func TestResolveExecutorRejectsUnsupportedValue(t *testing.T) {
	if _, err := ResolveExecutor("bad"); !errors.Is(err, ErrExecutionAdapterUnsupported) {
		t.Fatalf("err=%v", err)
	}
}
