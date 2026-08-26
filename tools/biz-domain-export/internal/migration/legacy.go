package migration

import (
	"context"
	"fmt"

	devicepersistence "github.com/hvritual/biz/internal/device/infrastructure/persistence"
	iampersistence "github.com/hvritual/biz/internal/iam/infrastructure/persistence"
	sitepersistence "github.com/hvritual/biz/internal/site/infrastructure/persistence"
	"gorm.io/gorm"
)

func Migrate(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("migration: database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := iampersistence.AutoMigrate(ctx, database); err != nil {
		return fmt.Errorf("migration: iam schema: %w", err)
	}
	if err := sitepersistence.AutoMigrate(ctx, database); err != nil {
		return fmt.Errorf("migration: site schema: %w", err)
	}
	if err := devicepersistence.AutoMigrate(ctx, database); err != nil {
		return fmt.Errorf("migration: device schema: %w", err)
	}
	if !database.Migrator().HasIndex(&devicepersistence.DevicePORecord{}, "uniq_device_tenant_serial") {
		if err := database.WithContext(ctx).Exec("CREATE UNIQUE INDEX uniq_device_tenant_serial ON biz_device_device (tenant_id, serial)").Error; err != nil {
			return fmt.Errorf("migration: device tenant/serial unique index: %w", err)
		}
	}
	return copyLegacy(ctx, database)
}

func copyLegacy(ctx context.Context, database *gorm.DB) error {
	copies := []struct {
		source string
		sql    string
	}{
		{"biz_tenants", `INSERT IGNORE INTO biz_iam_tenant (id,name,status,version,created_at,updated_at,deleted_at)
SELECT id,name,status,1,created_at,created_at,NULL FROM biz_tenants`},
		{"biz_users", `INSERT IGNORE INTO biz_iam_user (id,email,status,version,created_at,updated_at,deleted_at)
SELECT id,email,status,1,created_at,created_at,NULL FROM biz_users`},
		{"biz_memberships", `INSERT IGNORE INTO biz_iam_membership (id,tenant_id,user_id,status,version,created_at,updated_at,deleted_at)
SELECT LOWER(SHA2(CONCAT('membership:',tenant_id,':',user_id),256)),tenant_id,user_id,status,1,created_at,created_at,NULL FROM biz_memberships`},
		{"biz_roles", `INSERT IGNORE INTO biz_iam_role (id,tenant_id,name,status,version,created_at,updated_at,deleted_at)
SELECT LOWER(SHA2(CONCAT('role:',id),256)),tenant_id,name,status,1,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),NULL FROM biz_roles`},
		{"biz_member_roles", `INSERT IGNORE INTO biz_iam_member_role (id,tenant_id,user_id,role_id,version,created_at,updated_at,deleted_at)
SELECT LOWER(SHA2(CONCAT('member_role:',tenant_id,':',user_id,':',role_id),256)),tenant_id,user_id,LOWER(SHA2(CONCAT('role:',role_id),256)),1,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),NULL FROM biz_member_roles`},
		{"biz_role_permissions", `INSERT IGNORE INTO biz_iam_role_permission (id,tenant_id,role_id,permission,data_scope,version,created_at,updated_at,deleted_at)
SELECT LOWER(SHA2(CONCAT('role_permission:',tenant_id,':',role_id,':',permission),256)),tenant_id,LOWER(SHA2(CONCAT('role:',role_id),256)),permission,data_scope,1,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),NULL FROM biz_role_permissions`},
		{"biz_member_sites", `INSERT IGNORE INTO biz_iam_member_site (id,tenant_id,user_id,site_id,version,created_at,updated_at,deleted_at)
SELECT LOWER(SHA2(CONCAT('member_site:',tenant_id,':',user_id,':',site_id),256)),tenant_id,user_id,site_id,1,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),NULL FROM biz_member_sites`},
		{"biz_api_tokens", `INSERT IGNORE INTO biz_iam_api_token (id,token_hash,tenant_id,user_id,expires_at,disabled,version,created_at,updated_at,deleted_at)
SELECT token_hash,token_hash,tenant_id,user_id,expires_at,disabled,1,created_at,created_at,NULL FROM biz_api_tokens`},
		{"biz_sites", `INSERT IGNORE INTO biz_site_site (id,tenant_id,name,version,created_at,updated_at,deleted_at)
SELECT id,tenant_id,name,1,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3),NULL FROM biz_sites`},
		{"biz_devices", `INSERT IGNORE INTO biz_device_device (id,tenant_id,site_id,name,serial,created_by,version,created_at,updated_at,deleted_at)
SELECT id,tenant_id,site_id,name,serial,created_by,version,created_at,updated_at,NULL FROM biz_devices`},
	}
	for _, item := range copies {
		if !database.Migrator().HasTable(item.source) {
			continue
		}
		if err := database.WithContext(ctx).Exec(item.sql).Error; err != nil {
			return fmt.Errorf("migration: copy legacy %s: %w", item.source, err)
		}
	}
	return nil
}
