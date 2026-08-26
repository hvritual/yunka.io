package persistence

type MemberRolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID        string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID        string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
}
