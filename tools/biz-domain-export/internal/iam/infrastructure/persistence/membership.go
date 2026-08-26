package persistence

type MembershipPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID        string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	Status        string `gorm:"column:status;type:varchar(32);not null;index"`
}
