package identity

import (
	"context"
	"testing"
)

func TestPrincipalContextRoundTripIsIsolated(t *testing.T) {
	principal := Principal{
		Subject:       "user-1",
		TenantID:      "tenant-1",
		UserID:        "user-1",
		Roles:         []string{"admin"},
		AuthMethod:    AuthMethodJWT,
		Authenticated: true,
	}
	ctx := WithPrincipal(context.Background(), principal)

	got, ok := FromContext(ctx)
	if !ok || !got.Authenticated || got.TenantID != "tenant-1" || !got.HasRole("admin") {
		t.Fatalf("unexpected principal: %#v ok=%v", got, ok)
	}
	got.Roles[0] = "mutated"

	again, _ := FromContext(ctx)
	if again.Roles[0] != "admin" {
		t.Fatalf("context principal was mutated: %#v", again)
	}
}
