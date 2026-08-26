package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/iam/access"
	persistence "github.com/hvritual/biz/internal/iam/infrastructure/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Config struct {
	TenantID   string
	TenantName string
	UserID     string
	Email      string
	Token      string
}

func Ensure(ctx context.Context, database *gorm.DB, config Config) error {
	if strings.TrimSpace(config.Token) == "" {
		return nil
	}
	now := time.Now().UTC()
	naturalRoleID := config.TenantID + ":owner"
	roleID := hashID("role:" + naturalRoleID)
	tokenDigest := sha256.Sum256([]byte(config.Token))
	tokenHash := hex.EncodeToString(tokenDigest[:])

	tenant := persistence.TenantPORecord{
		TenantPO: persistence.TenantPO{Name: config.TenantName, Status: "active"},
		TenantPOBase: persistence.TenantPOBase{ID: config.TenantID, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	user := persistence.UserPORecord{
		UserPO: persistence.UserPO{Email: config.Email, Status: "active"},
		UserPOBase: persistence.UserPOBase{ID: config.UserID, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	membership := persistence.MembershipPORecord{
		MembershipPO: persistence.MembershipPO{ScopeTenantID: config.TenantID, UserID: config.UserID, Status: "active"},
		MembershipPOBase: persistence.MembershipPOBase{ID: hashID("membership:" + config.TenantID + ":" + config.UserID), Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	role := persistence.RolePORecord{
		RolePO: persistence.RolePO{ScopeTenantID: config.TenantID, Name: "owner", Status: "active"},
		RolePOBase: persistence.RolePOBase{ID: roleID, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	memberRole := persistence.MemberRolePORecord{
		MemberRolePO: persistence.MemberRolePO{ScopeTenantID: config.TenantID, UserID: config.UserID, RoleID: roleID},
		MemberRolePOBase: persistence.MemberRolePOBase{ID: hashID("member_role:" + config.TenantID + ":" + config.UserID + ":" + naturalRoleID), Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	token := persistence.APITokenPORecord{
		APITokenPO: persistence.APITokenPO{TokenHash: tokenHash, ScopeTenantID: config.TenantID, UserID: config.UserID},
		APITokenPOBase: persistence.APITokenPOBase{ID: tokenHash, Version: 1, CreatedAt: now, UpdatedAt: now},
	}

	for _, value := range []any{&tenant, &user, &membership, &role, &memberRole, &token} {
		if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error; err != nil {
			return err
		}
	}
	permissions := []string{
		access.PermissionDeviceRead,
		access.PermissionDeviceCreate,
		access.PermissionDeviceUpdate,
		access.PermissionDeviceDelete,
	}
	for _, permission := range permissions {
		value := persistence.RolePermissionPORecord{
			RolePermissionPO: persistence.RolePermissionPO{ScopeTenantID: config.TenantID, RoleID: roleID, Permission: permission, DataScope: string(access.DataScopeAll)},
			RolePermissionPOBase: persistence.RolePermissionPOBase{ID: hashID("role_permission:" + config.TenantID + ":" + naturalRoleID + ":" + permission), Version: 1, CreatedAt: now, UpdatedAt: now},
		}
		if err := database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&value).Error; err != nil {
			return err
		}
	}
	return nil
}

func hashID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
