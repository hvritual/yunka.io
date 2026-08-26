package authz

import (
	"context"
	"testing"

	"yunka.io/framework/core/identity"
)

type checkerFunc func(context.Context, string, []string, []PermissionKey, PermissionMode) (bool, error)

func (fn checkerFunc) HasPermissions(ctx context.Context, tenant string, roles []string, permissions []PermissionKey, mode PermissionMode) (bool, error) {
	return fn(ctx, tenant, roles, permissions, mode)
}

func TestRBACAuthorizerUsesStablePermissions(t *testing.T) {
	var gotTenant string
	var gotPermissions []PermissionKey
	authorizer, err := NewRBACAuthorizer(checkerFunc(func(_ context.Context, tenant string, _ []string, permissions []PermissionKey, mode PermissionMode) (bool, error) {
		gotTenant = tenant
		gotPermissions = append([]PermissionKey(nil), permissions...)
		if mode != PermissionAll {
			t.Fatalf("mode=%v", mode)
		}
		return true, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := authorizer.Authorize(context.Background(), identity.Principal{Authenticated: true, TenantID: "tenant-a", Roles: []string{"operator"}, AuthMethod: identity.AuthMethodJWT}, Policy{Operation: "device.machine.get", Permissions: []PermissionKey{"device.machine.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodJWT}})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.Reason != ReasonAllowed {
		t.Fatalf("decision=%+v", decision)
	}
	if gotTenant != "tenant-a" || len(gotPermissions) != 1 || gotPermissions[0] != "device.machine.read" {
		t.Fatalf("tenant=%q permissions=%v", gotTenant, gotPermissions)
	}
}

func TestRBACAuthorizerFailsClosed(t *testing.T) {
	authorizer, err := NewRBACAuthorizer(checkerFunc(func(context.Context, string, []string, []PermissionKey, PermissionMode) (bool, error) {
		return false, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	decision, err := authorizer.Authorize(context.Background(), identity.Principal{}, Policy{Operation: "device.machine.get", Permissions: []PermissionKey{"device.machine.read"}, TenantRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Reason != ReasonUnauthenticated {
		t.Fatalf("decision=%+v", decision)
	}
}
