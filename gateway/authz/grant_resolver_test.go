package authz

import (
	"context"
	"testing"

	"yunka.io/framework/core/identity"
)

type grantResolverFunc func(context.Context, GrantRequest) ([]Grant, error)

func (f grantResolverFunc) ResolveGrants(ctx context.Context, request GrantRequest) ([]Grant, error) {
	return f(ctx, request)
}

func TestGrantAuthorizerPrincipalAwareResolverAllowsNonTenantAuthority(t *testing.T) {
	calls := 0
	resolver := grantResolverFunc(func(_ context.Context, request GrantRequest) ([]Grant, error) {
		calls++
		if request.TenantBound {
			t.Fatal("non-tenant operation unexpectedly marked tenant-bound")
		}
		if request.Operation != "tenant.create" {
			t.Fatalf("operation=%q", request.Operation)
		}
		if request.Principal.TenantID != "" {
			t.Fatalf("tenant=%q", request.Principal.TenantID)
		}
		if request.Principal.Subject != "platform-admin:root" {
			t.Fatalf("subject=%q", request.Principal.Subject)
		}
		if len(request.Permissions) != 1 || request.Permissions[0] != "platform.tenant.create" {
			t.Fatalf("permissions=%v", request.Permissions)
		}
		return []Grant{{Permission: "platform.tenant.create", RoleID: "platform-admin"}}, nil
	})
	authorizer, err := NewGrantAuthorizerWithResolver(resolver)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{
		Subject:       "platform-admin:root",
		AuthMethod:    identity.AuthMethodAPIKey,
		Authenticated: true,
	}
	decision, err := authorizer.Authorize(context.Background(), principal, Policy{
		Operation:      "tenant.create",
		Permissions:    []PermissionKey{"platform.tenant.create"},
		Mode:           PermissionAll,
		TenantRequired: false,
		Authentication: []string{identity.AuthMethodAPIKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.Reason != ReasonAllowed {
		t.Fatalf("decision=%#v", decision)
	}
	if calls != 1 {
		t.Fatalf("resolver calls=%d want=1", calls)
	}
}

func TestLegacyTenantGrantCheckerFailsClosedForNonTenantAuthority(t *testing.T) {
	calls := 0
	checker := grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) {
		calls++
		return []Grant{{Permission: "platform.tenant.create"}}, nil
	})
	authorizer, err := NewGrantAuthorizer(checker)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{
		Subject:       "platform-admin:root",
		Roles:         []string{"platform-admin"},
		AuthMethod:    identity.AuthMethodAPIKey,
		Authenticated: true,
	}
	decision, err := authorizer.Authorize(context.Background(), principal, Policy{
		Operation:      "tenant.create",
		Permissions:    []PermissionKey{"platform.tenant.create"},
		Mode:           PermissionAll,
		TenantRequired: false,
		Authentication: []string{identity.AuthMethodAPIKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Reason != ReasonGrantResolverUnavailable {
		t.Fatalf("decision=%#v", decision)
	}
	if calls != 0 {
		t.Fatalf("legacy tenant checker calls=%d want=0", calls)
	}
}

func TestTenantRequiredStillFailsBeforeGrantResolution(t *testing.T) {
	calls := 0
	resolver := grantResolverFunc(func(context.Context, GrantRequest) ([]Grant, error) {
		calls++
		return []Grant{{Permission: "device.read"}}, nil
	})
	authorizer, err := NewGrantAuthorizerWithResolver(resolver)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{Authenticated: true, AuthMethod: identity.AuthMethodAPIKey}
	decision, err := authorizer.Authorize(context.Background(), principal, Policy{
		Operation:      "device.list",
		Permissions:    []PermissionKey{"device.read"},
		TenantRequired: true,
		Authentication: []string{identity.AuthMethodAPIKey},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.Reason != ReasonTenantRequired {
		t.Fatalf("decision=%#v", decision)
	}
	if calls != 0 {
		t.Fatalf("resolver calls=%d want=0", calls)
	}
}

func TestPrincipalAwareResolverOwnsRoleModel(t *testing.T) {
	resolver := grantResolverFunc(func(_ context.Context, request GrantRequest) ([]Grant, error) {
		if len(request.Principal.Roles) != 0 {
			t.Fatalf("roles=%v want none", request.Principal.Roles)
		}
		return []Grant{{Permission: "system.read", RoleID: "direct-grant"}}, nil
	})
	authorizer, err := NewGrantAuthorizerWithResolver(resolver)
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{Subject: "service:control-plane", Authenticated: true, AuthMethod: identity.AuthMethodService}
	decision, err := authorizer.Authorize(context.Background(), principal, Policy{
		Operation:      "system.inspect",
		Permissions:    []PermissionKey{"system.read"},
		Authentication: []string{identity.AuthMethodService},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("decision=%#v", decision)
	}
}
