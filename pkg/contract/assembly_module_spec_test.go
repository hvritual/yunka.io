package contract

import (
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
	"yunka.io/pkg/modulespec"
)

func TestRenderAssemblyModuleCodeInlinesDeclarativeModuleWithoutBuild(t *testing.T) {
	plan, err := assemblyplan.Compile(assemblyplan.Input{
		Identity: "root",
		Modules: []assemblyplan.ModuleInput{
			{
				Name:     "identity",
				Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipCanonical, Source: modulespec.EvidenceSource, Ref: "identity/module.yunka.json"},
			},
			{
				Name:      "access",
				Version:   "v0.1.0",
				DependsOn: []string{"identity"},
				Requirements: assemblyplan.ModuleRequirements{
					Logger:    true,
					Databases: []string{"primary"},
				},
				Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipCanonical, Source: modulespec.EvidenceSource, Ref: "access/module.yunka.json"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderAssemblyModuleCode(plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	source := string(files[0].Content)
	for _, expected := range []string{
		`Name: "access"`,
		`DependsOn: []string{"identity"}`,
		`Logger: true`,
		`{Name: "primary"}`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("generated source missing %q:\n%s", expected, source)
		}
	}
	for _, forbidden := range []string{"accessmodule", "GeneratedDescriptor", "Build:"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("declarative module unexpectedly uses runtime binding %q:\n%s", forbidden, source)
		}
	}
}

func TestRenderAssemblyModuleCodePreservesLegacyBindingBesideDeclarativeModule(t *testing.T) {
	plan, err := assemblyplan.Compile(assemblyplan.Input{
		Identity: "root",
		Modules: []assemblyplan.ModuleInput{
			{Name: "access", Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipCanonical, Source: modulespec.EvidenceSource, Ref: "access/module.yunka.json"}},
			{Name: "worker", Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "generated-module-source", Ref: "worker/zz_yunka_module_gen.go"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderAssemblyModuleCode(plan, []ModuleBinding{{Name: "worker", ImportPath: "example.com/product/modules/worker", DescriptorSymbol: "GeneratedDescriptor"}})
	if err != nil {
		t.Fatal(err)
	}
	source := string(files[0].Content)
	if !strings.Contains(source, "workermodule.GeneratedDescriptor()") || !strings.Contains(source, `Name: "access"`) {
		t.Fatalf("mixed module registration lost:\n%s", source)
	}
}
