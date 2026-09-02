package db

import (
	"context"
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/gateway/authz"
)

func openPermissionTestStore(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(t.TempDir()+"/gateway.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	store, err := NewStoreFromDB(database)
	if err != nil {
		t.Fatal(err)
	}
	return store, database
}

func TestRoleOwnsPermissionsWithTenantIsolation(t *testing.T) {
	store, database := openPermissionTestStore(t)
	if err := store.GrantRolePermissions("tenant-1", "role-a", []string{"drink.read", "drink.write", "drink.write"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.GrantRolePermissions("tenant-1", "role-b", []string{"device.read"}, nil); err != nil {
		t.Fatal(err)
	}

	allowed, err := store.HasPermissions(context.Background(), "tenant-1", []string{"role-a"}, []authz.PermissionKey{"drink.read", "drink.write"}, authz.PermissionAll)
	if err != nil || !allowed {
		t.Fatalf("all permissions allowed=%v err=%v", allowed, err)
	}
	allowed, err = store.HasPermissions(context.Background(), "tenant-1", []string{"role-a", "role-b"}, []authz.PermissionKey{"missing", "device.read"}, authz.PermissionAny)
	if err != nil || !allowed {
		t.Fatalf("any permission allowed=%v err=%v", allowed, err)
	}
	allowed, err = store.HasPermissions(context.Background(), "tenant-2", []string{"role-a"}, []authz.PermissionKey{"drink.read"}, authz.PermissionAll)
	if err != nil || allowed {
		t.Fatalf("cross-tenant permission allowed=%v err=%v", allowed, err)
	}

	if err := store.GrantRolePermissions("tenant-1", "role-a", nil, []string{"drink.write"}); err != nil {
		t.Fatal(err)
	}
	allowed, err = store.HasPermissions(context.Background(), "tenant-1", []string{"role-a"}, []authz.PermissionKey{"drink.write"}, authz.PermissionAll)
	if err != nil || allowed {
		t.Fatalf("revoked permission allowed=%v err=%v", allowed, err)
	}

	var count int64
	if err := database.Model(&RolePermission{}).Where("org_uuid = ? AND role_uuid = ? AND permission = ?", "tenant-1", "role-a", "drink.read").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("idempotent permission rows=%d want=1", count)
	}
}

func TestButtonReferencesPermissionsWithoutBecomingGrant(t *testing.T) {
	store, database := openPermissionTestStore(t)
	if err := store.BindButtonPermissions([]ButtonPermissionBinding{
		{ModuleButtonUUID: "button-1", Permissions: []string{"drink.read", "drink.write"}},
		{ModuleButtonUUID: "button-2", Permissions: []string{"device.read"}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.PermissionsForButtons([]string{"button-1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"drink.read", "drink.write"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("button permissions=%v want=%v", got, want)
	}

	if err := store.BindButtonPermissions([]ButtonPermissionBinding{{ModuleButtonUUID: "button-1", Permissions: []string{"drink.execute"}}}); err != nil {
		t.Fatal(err)
	}
	got, err = store.PermissionsForButtons([]string{"button-1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"drink.execute"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("replaced button permissions=%v want=%v", got, want)
	}

	var grants int64
	if err := database.Model(&RolePermission{}).Count(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatalf("button mapping unexpectedly created %d role grants", grants)
	}
}

func TestLegacyRoleButtonGrantBackfillsToPermission(t *testing.T) {
	store, database := openPermissionTestStore(t)
	if err := database.Exec(`CREATE TABLE role_module_button (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		org_uuid TEXT NOT NULL,
		role_uuid TEXT NOT NULL,
		module_button_uuid TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("INSERT INTO role_module_button(org_uuid, role_uuid, module_button_uuid) VALUES (?, ?, ?)", "tenant-legacy", "role-legacy", "button-legacy").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.BindButtonPermissions([]ButtonPermissionBinding{{ModuleButtonUUID: "button-legacy", Permissions: []string{"legacy.execute"}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.BackfillLegacyRolePermissionsForButtons([]string{"button-legacy"}); err != nil {
		t.Fatal(err)
	}
	allowed, err := store.HasPermissions(context.Background(), "tenant-legacy", []string{"role-legacy"}, []authz.PermissionKey{"legacy.execute"}, authz.PermissionAll)
	if err != nil || !allowed {
		t.Fatalf("legacy backfill allowed=%v err=%v", allowed, err)
	}
}
