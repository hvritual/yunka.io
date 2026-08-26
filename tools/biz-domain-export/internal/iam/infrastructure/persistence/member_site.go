package persistence

type MemberSitePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID        string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	SiteID        string `gorm:"column:site_id;type:varchar(64);not null;index" yunka:"-"`
}
