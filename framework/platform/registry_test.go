package platform

import (
	"context"
	"testing"
)

func TestTypedRegistryRejectsDuplicateAndMutationAfterSeal(t *testing.T) {
	registry := NewRegistry[DatabaseFactory]()
	factory := DatabaseFactoryFunc(func(_ context.Context, _ string) (DatabaseResource, error) { return DatabaseResource{}, nil })
	if err := registry.Register("primary", factory); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("primary", factory); err == nil {
		t.Fatal("expected duplicate registration failure")
	}
	registry.Seal()
	if err := registry.Register("secondary", factory); err == nil {
		t.Fatal("expected sealed registry failure")
	}
	if names := registry.Names(); len(names) != 1 || names[0] != "primary" {
		t.Fatalf("unexpected names: %v", names)
	}
}
