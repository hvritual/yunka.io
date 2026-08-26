package db

const RolePermissionTableName = "role_permission"

// RolePermission is the only persistent authorization grant owned by a role.
// UI buttons are intentionally not part of this relationship.
type RolePermission struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	OrgUUID    string `gorm:"column:org_uuid;size:191;not null;uniqueIndex:ux_role_permission,priority:1;index:idx_role_permission_lookup,priority:1"`
	RoleUUID   string `gorm:"column:role_uuid;size:191;not null;uniqueIndex:ux_role_permission,priority:2;index:idx_role_permission_lookup,priority:2"`
	Permission string `gorm:"column:permission;size:191;not null;uniqueIndex:ux_role_permission,priority:3;index:idx_role_permission_lookup,priority:3"`
}

func (*RolePermission) TableName() string { return RolePermissionTableName }
