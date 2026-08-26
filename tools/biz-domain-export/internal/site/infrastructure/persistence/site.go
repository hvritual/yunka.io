package persistence

type SitePO struct {
	Name string `gorm:"column:name;type:varchar(200);not null"`
}
