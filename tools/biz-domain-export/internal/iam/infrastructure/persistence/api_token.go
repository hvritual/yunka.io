package persistence

import "time"

type APITokenPO struct {
	TokenHash     string     `gorm:"column:token_hash;type:varchar(64);not null;uniqueIndex" yunka:"-"`
	ScopeTenantID string     `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID        string     `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" yunka:"-"`
	Disabled      bool       `gorm:"column:disabled;not null;default:false" yunka:"-"`
}
