package contract

import (
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

func TestRenderAssemblyModuleCodeUsesExplicitBindingsOnly(t *testing.T) {
	plan, err := assemblyplan.Compile(assemblyplan.Input{
		Identity: "root",
		Modules: []assemblyplan.ModuleInput{{
			Name: "device",
			Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "generated-module-source", Ref: "device/zz_yunka_module_gen.go"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderAssemblyModuleCode(plan, []ModuleBinding{{Name: "device", ImportPath: "example.com/product/modules/device", DescriptorSymbol: "GeneratedDescriptor"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != AssemblyModuleCodePath {
		t.Fatalf("unexpected files: %#v", files)
	}
	source := string(files[0].Content)
	for _, required := range []string{
		`devicemodule "example.com/product/modules/device"`,
		"func NewCatalog(additional ...modulecatalog.Descriptor) (*modulecatalog.Catalog, error)",
		"catalog.Register(devicemodule.GeneratedDescriptor())",
		"for _, descriptor := range additional",
		"catalog.Register(descriptor)",
		"catalog.Seal()",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("generated module catalog missing %q:\n%s", required, source)
		}
	}
	for _, forbidden := range []string{"module.Name", "reflect.", "plugin.Open", "ServiceLocator", "modulecatalog.Default()"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("generated module catalog contains forbidden inference/runtime lookup %q:\n%s", forbidden, source)
		}
	}
}

func TestRenderAssemblyModuleCodeFailsWithoutExactBinding(t *testing.T) {
	plan, err := assemblyplan.Compile(assemblyplan.Input{
		Identity: "root",
		Modules: []assemblyplan.ModuleInput{{
			Name: "device",
			Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "generated-module-source", Ref: "device/zz_yunka_module_gen.go"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderAssemblyModuleCode(plan, nil)
	if err == nil || !strings.Contains(err.Error(), "no explicit generated Go binding") {
		t.Fatalf("expected missing binding failure, got %v", err)
	}
}
