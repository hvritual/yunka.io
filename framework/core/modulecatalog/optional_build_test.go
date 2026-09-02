package modulecatalog

import "testing"

func TestCatalogAcceptsDescriptorWithoutRuntimeBuild(t *testing.T) {
	catalog := New()
	if err := catalog.Register(Descriptor{Name: "access", Requirements: Requirements{Databases: []DatabaseRequirement{{Name: "primary"}}}}); err != nil {
		t.Fatal(err)
	}
	plan, err := catalog.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Descriptors) != 1 || plan.Descriptors[0].Build != nil {
		t.Fatalf("unexpected declarative descriptor: %#v", plan.Descriptors)
	}
}
