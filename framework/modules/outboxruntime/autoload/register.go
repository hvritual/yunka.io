package autoload

import (
	"yunka.io/framework/core/modulecatalog"
	module "yunka.io/framework/modules/outboxruntime"
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
