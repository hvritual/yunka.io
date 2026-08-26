package policy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"yunka.io/framework/core/identity"
)

type resource struct {
	SiteID  string
	OwnerID string
}

func TestContextResolverAndAnyScope(t *testing.T) {
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true}
	ctx := WithGrants(context.Background(), map[string]Grant{
		"device.read": {Allowed: true, SiteIDs: []string{"site-b", "site-a", "site-a"}, Self: true},
	})
	rule := Permission("device.read", Any(
		Site(func(value resource) string { return value.SiteID }),
		Self(func(value resource) string { return value.OwnerID }),
	))

	filter, err := rule.Scope(ctx, ContextResolver{}, principal)
	if err != nil {
		t.Fatal(err)
	}
	if filter.All || !filter.UseSites || !filter.UseSelf || filter.OwnerID != "user-a" {
		t.Fatalf("unexpected filter: %+v", filter)
	}
	if !reflect.DeepEqual(filter.SiteIDs, []string{"site-a", "site-b"}) {
		t.Fatalf("unexpected sites: %#v", filter.SiteIDs)
	}
	for _, target := range []resource{{SiteID: "site-a"}, {OwnerID: "user-a"}} {
		if err := rule.Authorize(ctx, ContextResolver{}, principal, target); err != nil {
			t.Fatalf("expected target allowed: %v", err)
		}
	}
	if err := rule.Authorize(ctx, ContextResolver{}, principal, resource{SiteID: "site-x", OwnerID: "user-x"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestAllGrantOverridesMatcher(t *testing.T) {
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true}
	resolver := ResolverFunc(func(context.Context, identity.Principal, string) (Grant, error) {
		return Grant{Allowed: true, All: true}, nil
	})
	rule := Permission("device.update", Site(func(value resource) string { return value.SiteID }))
	if err := rule.Authorize(context.Background(), resolver, principal, resource{SiteID: "other"}); err != nil {
		t.Fatal(err)
	}
	filter, err := rule.Scope(context.Background(), resolver, principal)
	if err != nil || !filter.All {
		t.Fatalf("want all filter, got %+v err=%v", filter, err)
	}
}

func TestOpenRuleRemainsBackwardCompatible(t *testing.T) {
	var rule Rule[resource]
	if err := rule.Authorize(context.Background(), nil, identity.Principal{}, resource{}); err != nil {
		t.Fatal(err)
	}
	filter, err := rule.Scope(context.Background(), nil, identity.Principal{})
	if err != nil || !filter.All {
		t.Fatalf("want open all filter, got %+v err=%v", filter, err)
	}
}

func TestPermissionRequiresAuthenticatedPrincipal(t *testing.T) {
	rule := Permission[resource]("device.read", All[resource]())
	err := rule.Authorize(context.Background(), ResolverFunc(func(context.Context, identity.Principal, string) (Grant, error) {
		return Grant{Allowed: true, All: true}, nil
	}), identity.Principal{}, resource{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want unauthorized, got %v", err)
	}
}
