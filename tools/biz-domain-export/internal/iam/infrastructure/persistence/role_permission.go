package persistence

type RolePermissionPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID        string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
	Permission    string `gorm:"column:permission;type:varchar(120);not null;index"`
	DataScope     string `gorm:"column:data_scope;type:varchar(16);not null"`
}
