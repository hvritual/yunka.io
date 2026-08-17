package db

/**
 * @BelongProject yunka
 * @BelongPackage db
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/20 1:56 下午
 * @Version V1.0
 */

const (
	// RoleModuleButtonTableName represent at role_module_button store table
	RoleModuleButtonTableName = `role_module_button`
)

// RoleModuleButton is role_module_button domain
type RoleModuleButton struct {
	OrgUUID          string `gorm:"column:org_uuid;type:varchar(32);uniqueIndex:org_role_button"`
	RoleUUID         string `gorm:"column:role_uuid;type:varchar(32);uniqueIndex:org_role_button"`
	ModuleButtonUUID string `gorm:"column:module_button_uuid;type:varchar(32);uniqueIndex:org_role_button"`
}

// TableName implement sync domain interface
func (RoleModuleButton) TableName() string {
	return RoleModuleButtonTableName
}
