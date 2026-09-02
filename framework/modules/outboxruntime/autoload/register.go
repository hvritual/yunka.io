package autoload

import (
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	module "github.com/hvritual/yunka.io/framework/modules/outboxruntime"
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
