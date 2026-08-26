package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/gateway/authz"
)

const legacyRoleModuleButtonTableName = "role_module_button"

type Store struct {
	*gorm.DB
	dirName string
}

func (s *Store) BindButtonPermissions(bindings []ButtonPermissionBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	normalized := make(map[string][]string, len(bindings))
	for _, binding := range bindings {
		button := strings.TrimSpace(binding.ModuleButtonUUID)
		if button == "" {
			continue
		}
		normalized[button] = mergeStrings(normalized[button], binding.Permissions)
	}
	if len(normalized) == 0 {
		return nil
	}
	buttons := make([]string, 0, len(normalized))
	for button := range normalized {
		buttons = append(buttons, button)
	}
	sort.Strings(buttons)
	return s.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("module_button_uuid IN ?", buttons).Delete(&ButtonPermission{}).Error; err != nil {
			return err
		}
		rows := make([]ButtonPermission, 0)
		for _, button := range buttons {
			for _, permission := range normalized[button] {
				rows = append(rows, ButtonPermission{ModuleButtonUUID: button, Permission: permission})
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (s *Store) PermissionsForButtons(buttonUUIDs []string) ([]string, error) {
	buttons := normalizeStrings(buttonUUIDs)
	if len(buttons) == 0 {
		return nil, nil
	}
	var permissions []string
	err := s.Model(&ButtonPermission{}).
		Where("module_button_uuid IN ?", buttons).
		Distinct("permission").
		Pluck("permission", &permissions).Error
	if err != nil {
		return nil, err
	}
	return normalizeStrings(permissions), nil
}

// BackfillLegacyRolePermissionsForButtons converts historical Role->Button
// grants to Role->Permission after the corresponding Button->Permission
// mapping is known. The legacy table is read-only and is never migrated or
// written by C8.3.
func (s *Store) BackfillLegacyRolePermissionsForButtons(buttonUUIDs []string) error {
	buttons := normalizeStrings(buttonUUIDs)
	if len(buttons) == 0 || !s.Migrator().HasTable(legacyRoleModuleButtonTableName) {
		return nil
	}
	type legacyGrant struct {
		OrgUUID    string `gorm:"column:org_uuid"`
		RoleUUID   string `gorm:"column:role_uuid"`
		Permission string `gorm:"column:permission"`
	}
	var grants []legacyGrant
	if err := s.Table(legacyRoleModuleButtonTableName+" AS role_button").
		Select("role_button.org_uuid AS org_uuid, role_button.role_uuid AS role_uuid, button_permission.permission AS permission").
		Joins("JOIN "+ButtonPermissionTableName+" AS button_permission ON button_permission.module_button_uuid = role_button.module_button_uuid").
		Where("role_button.module_button_uuid IN ?", buttons).
		Scan(&grants).Error; err != nil {
		return err
	}
	rows := make([]RolePermission, 0, len(grants))
	for _, grant := range grants {
		org := strings.TrimSpace(grant.OrgUUID)
		role := strings.TrimSpace(grant.RoleUUID)
		permission := strings.TrimSpace(grant.Permission)
		if org == "" || role == "" || permission == "" {
			continue
		}
		rows = append(rows, RolePermission{OrgUUID: org, RoleUUID: role, Permission: permission})
	}
	if len(rows) == 0 {
		return nil
	}
	return s.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (s *Store) GrantRolePermissions(orgUUID, roleUUID string, addPermissions, deletePermissions []string) error {
	orgUUID = strings.TrimSpace(orgUUID)
	roleUUID = strings.TrimSpace(roleUUID)
	addPermissions = normalizeStrings(addPermissions)
	deletePermissions = normalizeStrings(deletePermissions)
	if len(addPermissions) == 0 && len(deletePermissions) == 0 {
		return nil
	}
	if orgUUID == "" || roleUUID == "" {
		return errors.New("role db: organization and role are required for permission mutation")
	}
	rows := make([]RolePermission, 0, len(addPermissions))
	for _, permission := range addPermissions {
		rows = append(rows, RolePermission{OrgUUID: orgUUID, RoleUUID: roleUUID, Permission: permission})
	}
	return s.Transaction(func(tx *gorm.DB) error {
		if len(rows) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
				return err
			}
		}
		if len(deletePermissions) == 0 {
			return nil
		}
		return tx.Where("org_uuid = ? AND role_uuid = ? AND permission IN ?", orgUUID, roleUUID, deletePermissions).
			Delete(&RolePermission{}).Error
	})
}

func (s *Store) HasPermissions(ctx context.Context, tenantID string, roleIDs []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	roles := normalizeStrings(roleIDs)
	required := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		required = append(required, string(permission))
	}
	required = normalizeStrings(required)
	if len(required) == 0 {
		return true, nil
	}
	if tenantID == "" || len(roles) == 0 {
		return false, nil
	}
	if mode != authz.PermissionAll && mode != authz.PermissionAny {
		return false, errors.New("role db: unsupported permission match mode")
	}
	var count int64
	err := s.WithContext(ctx).Model(&RolePermission{}).
		Where("org_uuid = ? AND role_uuid IN ? AND permission IN ?", tenantID, roles, required).
		Distinct("permission").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if mode == authz.PermissionAny {
		return count > 0, nil
	}
	return count == int64(len(required)), nil
}

// NewStoreFromDB binds the role repository to an existing GORM handle. In the
// typed path this handle is the request-owned transaction supplied by
// requestscope; the Store never owns or closes the App-level connection pool.
func NewStoreFromDB(database *gorm.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("role db: GORM database is required")
	}
	return &Store{DB: database}, nil
}

func Migrate(database *gorm.DB) error {
	if database == nil {
		return errors.New("role db: GORM database is required")
	}
	return database.AutoMigrate(&RolePermission{}, &ButtonPermission{})
}

func NewStore(dirName, dbName string) (*Store, error) {
	if err := os.MkdirAll(dirName, 0750); err != nil {
		return nil, err
	}
	database, err := gorm.Open(sqlite.Open(filepath.Join(dirName, dbName)), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	store, err := NewStoreFromDB(database)
	if err != nil {
		return nil, err
	}
	store.dirName = dirName
	return store, Migrate(database)
}

func mergeStrings(left, right []string) []string {
	values := append(append([]string(nil), left...), right...)
	return normalizeStrings(values)
}

func normalizeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
