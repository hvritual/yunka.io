package persistence

type TenantPO struct {
	Name   string `gorm:"column:name;type:varchar(200);not null"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
