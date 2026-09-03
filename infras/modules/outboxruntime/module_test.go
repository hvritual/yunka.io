package outboxruntime

import (
	"reflect"
	"testing"

	canonical "github.com/hvritual/yunka.io/framework/modules/outboxruntime"
)

func TestGeneratedDescriptorMatchesCanonicalOutboxRuntime(t *testing.T) {
	got := GeneratedDescriptor()
	want := canonical.GeneratedDescriptor()
	if got.Name != want.Name || got.Version != want.Version {
		t.Fatalf("descriptor identity got=%s@%s want=%s@%s", got.Name, got.Version, want.Name, want.Version)
	}
	if !reflect.DeepEqual(got.Requirements, want.Requirements) {
		t.Fatalf("requirements=%#v want=%#v", got.Requirements, want.Requirements)
	}
	if got.Build == nil {
		t.Fatal("facade descriptor lost canonical build function")
	}
}
