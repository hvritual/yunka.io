package persistence

type UserPO struct {
	Email  string `gorm:"column:email;type:varchar(320);not null;uniqueIndex"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
