package middleware

import (
	"reflect"
	"testing"

	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/pkg/define"
)

func TestPrincipalFromClaims(t *testing.T) {
	principal := principalFromClaims(map[string]interface{}{
		"sub":           "subject-1",
		define.OrgUUID:  "tenant-1",
		define.UserUUID: "user-1",
		define.RoleUUID: "admin|operator",
	})
	if !principal.Authenticated || principal.AuthMethod != identity.AuthMethodJWT {
		t.Fatalf("principal=%#v", principal)
	}
	if principal.Subject != "subject-1" || principal.TenantID != "tenant-1" || principal.UserID != "user-1" {
		t.Fatalf("principal=%#v", principal)
	}
	if !reflect.DeepEqual(principal.Roles, []string{"admin", "operator"}) {
		t.Fatalf("roles=%v", principal.Roles)
	}
}

func TestPrincipalFromClaimsFallsBackToSubject(t *testing.T) {
	principal := principalFromClaims(map[string]interface{}{"sub": "subject-1"})
	if principal.UserID != "subject-1" || principal.Subject != "subject-1" {
		t.Fatalf("principal=%#v", principal)
	}
}
