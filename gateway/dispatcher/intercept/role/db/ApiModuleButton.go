package db

/**
 * @BelongProject yunka
 * @BelongPackage db
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/20 1:53 下午
 * @Version V1.0
 */

const (
	// APiModuleButtonTableName represent at api_module_button store table
	ApiModuleButtonTableName = `api_module_button`
)

// APiModuleButton is api_module_button domain
type ApiModuleButton struct {
	ModuleButtonUUID string `gorm:"column:module_button_uuid;type:varchar(32);uniqueIndex:module_api"`
	ApiUUID          string `gorm:"column:api_uuid;type:varchar(32);uniqueIndex:module_api"`
}

// TableName implement sync domain interface
func (ApiModuleButton) TableName() string {
	return ApiModuleButtonTableName
}
