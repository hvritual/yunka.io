package deviceops

import (
	"gorm.io/gorm"
	"yunka.io/pkg/logExt"
)

type Dependencies struct {
	Config          Config
	Logger          logExt.Logger
	PrimaryDatabase *gorm.DB
}
