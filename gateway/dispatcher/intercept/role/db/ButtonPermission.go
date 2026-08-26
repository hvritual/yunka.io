package db

const ButtonPermissionTableName = "button_permission"

// ButtonPermission maps a UI button to the permissions it represents. The
// mapping drives visibility and compatibility translation only; it is never a
// role grant.
type ButtonPermission struct {
	ID               uint64 `gorm:"primaryKey;autoIncrement"`
	ModuleButtonUUID string `gorm:"column:module_button_uuid;size:191;not null;uniqueIndex:ux_button_permission,priority:1;index:idx_button_permission_lookup,priority:1"`
	Permission       string `gorm:"column:permission;size:191;not null;uniqueIndex:ux_button_permission,priority:2;index:idx_button_permission_lookup,priority:2"`
}

func (*ButtonPermission) TableName() string { return ButtonPermissionTableName }

type ButtonPermissionBinding struct {
	ModuleButtonUUID string
	Permissions      []string
}
