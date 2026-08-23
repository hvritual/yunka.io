package role

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"yunka.io/gateway/dispatcher/intercept/role/db"
	"yunka.io/gateway/rpc/meta"
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

func TestRoleInterceptTypedDatabaseCommitsRequestOwnedMutation(t *testing.T) {
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
		OrgUUID:       "tenant-1",
		RoleUUID:      "role-1",
		ModuleBtnUUID: []string{"button-1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := database.Model(&db.RoleModuleButton{}).
		Where("org_uuid = ? AND role_uuid = ? AND module_button_uuid = ?", "tenant-1", "role-1", "button-1").
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("committed role rows=%d want 1", count)
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
		OrgUUID:       "tenant-cancel",
		RoleUUID:      "role-cancel",
		ModuleBtnUUID: []string{"button-cancel"},
	})
	if err == nil {
		t.Fatal("canceled request unexpectedly succeeded")
	}

	var count int64
	if err := database.Model(&db.RoleModuleButton{}).
		Where("org_uuid = ?", "tenant-cancel").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("canceled request committed %d rows", count)
	}
}
