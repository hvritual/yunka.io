package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC83GatewayAuthorizationUsesPermissionAsOnlyGrantPrimitive(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(content)
	}

	intercept := read("gateway/dispatcher/intercept/intercept.go")
	roleHandler := read("gateway/dispatcher/intercept/role/role_intercept.go")
	roleDB := read("gateway/dispatcher/intercept/role/db/sqlite_db.go")
	roleMiddleware := read("gateway/dispatcher/middleware/role_middleware.go")
	proto := read("contracts/proto/gateway/gateway.proto")
	authz := read("gateway/authz/types.go")

	for path, content := range map[string]string{
		"gateway/dispatcher/intercept/intercept.go":                  intercept,
		"gateway/dispatcher/intercept/role/role_intercept.go":       roleHandler,
		"gateway/dispatcher/middleware/role_middleware.go":           roleMiddleware,
	} {
		for _, forbidden := range []string{"VerifyRoleApiRight", "VerifyRoleAPIRight"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s retains legacy API->Button->Role authorization token %q", path, forbidden)
			}
		}
	}
	if strings.Contains(roleHandler, ".OperateRole(") || strings.Contains(roleHandler, "db.ApiModuleButton") {
		t.Error("role handler still writes legacy Button authorization relationships")
	}
	if !strings.Contains(roleHandler, "GrantRolePermissions") || !strings.Contains(roleHandler, "PermissionsForButtons") {
		t.Error("role handler must mutate Permission grants and translate legacy Button inputs through Button->Permission")
	}
	if !strings.Contains(roleDB, "AutoMigrate(&RolePermission{}, &ButtonPermission{})") {
		t.Error("C8.3 migration must own only RolePermission and ButtonPermission tables")
	}
	if strings.Contains(roleDB, "AutoMigrate(&ApiModuleButton") || strings.Contains(roleDB, "AutoMigrate(&RoleModuleButton") {
		t.Error("legacy API/Role Button tables must not be created by C8.3")
	}
	if !strings.Contains(proto, "repeated string permissions = 5") || !strings.Contains(proto, "repeated string deletePermissions = 6") {
		t.Error("RoleModuleBtn protobuf must expose direct Permission mutation")
	}
	if !strings.Contains(proto, "repeated string permissions = 3") {
		t.Error("RuntimeApiModuleButton protobuf must reference Permission")
	}
	if !strings.Contains(authz, "type PermissionChecker interface") || !strings.Contains(authz, "type Authorizer interface") {
		t.Error("authorization complexity must remain owned by gateway/authz")
	}
}
