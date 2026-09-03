package autoload

import (
	"testing"

	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
	module "github.com/hvritual/yunka.io/infras/modules/outboxruntime"
)

func TestAutoloadRegistersCanonicalDescriptor(t *testing.T) {
	plan, err := modulecatalog.Default().Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range plan.Descriptors {
		if descriptor.Name == module.ModuleName {
			return
		}
	}
	t.Fatalf("autoload did not register %q: names=%v", module.ModuleName, plan.Names())
}
