package policy

import (
	"context"
	"errors"
	"testing"

	"yunka.io/framework/core/identity"
)

func TestAllMatcherDoesNotUpgradeSiteGrant(t *testing.T) {
	principal := identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true}
	resolver := ResolverFunc(func(context.Context, identity.Principal, string) (Grant, error) {
		return Grant{Allowed: true, SiteIDs: []string{"site-a"}}, nil
	})
	rule := Permission("device.read", Any(
		All[resource](),
		Site(func(value resource) string { return value.SiteID }),
	))
	if err := rule.Authorize(context.Background(), resolver, principal, resource{SiteID: "site-b"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("site-scoped grant was upgraded to all: %v", err)
	}
	filter, err := rule.Scope(context.Background(), resolver, principal)
	if err != nil {
		t.Fatal(err)
	}
	if filter.All || !filter.UseSites || len(filter.SiteIDs) != 1 || filter.SiteIDs[0] != "site-a" {
		t.Fatalf("unexpected filter: %+v", filter)
	}
}
