package autoload

import (
	"github.com/hvritual/biz/modules/deviceops"
	"yunka.io/framework/core/modulecatalog"
)

func init() { modulecatalog.MustRegister(deviceops.GeneratedDescriptor()) }
