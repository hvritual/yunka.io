package persistence

type RolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	Name          string `gorm:"column:name;type:varchar(100);not null"`
	Status        string `gorm:"column:status;type:varchar(32);not null"`
}
