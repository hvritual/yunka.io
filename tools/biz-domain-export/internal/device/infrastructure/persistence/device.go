package persistence

type DevicePO struct {
	SiteID    string `gorm:"column:site_id;type:varchar(64);not null;index"`
	Name      string `gorm:"column:name;type:varchar(200);not null"`
	Serial    string `gorm:"column:serial;type:varchar(128);not null;index"`
	CreatedBy string `gorm:"column:created_by;type:varchar(64);not null;index"`
}
