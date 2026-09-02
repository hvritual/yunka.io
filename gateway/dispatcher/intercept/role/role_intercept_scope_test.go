package role

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/gateway/dispatcher/intercept/role/db"
	"github.com/hvritual/yunka.io/gateway/rpc/meta"
)

type captureGatewayRegistrar struct {
	service meta.GatewayServiceServer
}

func (registrar *captureGatewayRegistrar) RegisterGatewayService(service meta.GatewayServiceServer) error {
	registrar.service = service
	return nil
}

func openRoleScopeTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(t.TempDir()+"/gateway.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestRoleInterceptTypedDatabaseCommitsPermissionMutation(t *testing.T) {
	database := openRoleScopeTestDatabase(t)
	registrar := &captureGatewayRegistrar{}
	_, err := NewRoleInterceptWithDatabase(nil, registrar, database)
	if err != nil {
		t.Fatal(err)
	}
	if registrar.service == nil {
		t.Fatal("GatewayService was not registered")
	}

	_, err = registrar.service.OperateRoleAPI(context.Background(), &meta.RoleModuleBtn{
		OrgUUID:     "tenant-1",
		RoleUUID:    "role-1",
		Permissions: []string{"drink.execute"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := database.Model(&db.RolePermission{}).
		Where("org_uuid = ? AND role_uuid = ? AND permission = ?", "tenant-1", "role-1", "drink.execute").
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("committed permission rows=%d want 1", count)
	}
}

func TestRoleInterceptLegacyButtonInputTranslatesToPermission(t *testing.T) {
	database := openRoleScopeTestDatabase(t)
	registrar := &captureGatewayRegistrar{}
	_, err := NewRoleInterceptWithDatabase(nil, registrar, database)
	if err != nil {
		t.Fatal(err)
	}
	_, err = registrar.service.BatchAddRuntimeApi(context.Background(), &meta.BatchRuntimeApiRequest{
		Apis: []*meta.RuntimeApi{{
			Uuid: "api-1",
			Authorization: &meta.RuntimeAuthorization{Permissions: []string{"drink.execute"}},
		}},
		Buttons: []*meta.RuntimeApiModuleButton{{ApiUUID: "api-1", ModuleBtnUUID: "button-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registrar.service.OperateRoleAPI(context.Background(), &meta.RoleModuleBtn{
		OrgUUID:       "tenant-legacy",
		RoleUUID:      "role-legacy",
		ModuleBtnUUID: []string{"button-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.Model(&db.RolePermission{}).
		Where("org_uuid = ? AND role_uuid = ? AND permission = ?", "tenant-legacy", "role-legacy", "drink.execute").
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("translated permission rows=%d want 1", count)
	}
}

func TestRoleInterceptCanceledRequestDoesNotMutateDatabase(t *testing.T) {
	database := openRoleScopeTestDatabase(t)
	registrar := &captureGatewayRegistrar{}
	_, err := NewRoleInterceptWithDatabase(nil, registrar, database)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = registrar.service.OperateRoleAPI(ctx, &meta.RoleModuleBtn{
		OrgUUID:     "tenant-cancel",
		RoleUUID:    "role-cancel",
		Permissions: []string{"cancel.execute"},
	})
	if err == nil {
		t.Fatal("canceled request unexpectedly succeeded")
	}

	var count int64
	if err := database.Model(&db.RolePermission{}).
		Where("org_uuid = ?", "tenant-cancel").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("canceled request committed %d rows", count)
	}
}
