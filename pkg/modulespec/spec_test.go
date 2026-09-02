package modulespec

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMarshalNormalizesDeclarativeModuleSpec(t *testing.T) {
	spec := Default()
	spec.DependsOn = []string{"tenant", "identity", "tenant"}
	spec.Requirements.Databases = []string{"primary", "analytics", "primary"}
	spec.Requirements.RPC = []string{"gateway", "gateway"}
	data, err := Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), Filename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.DependsOn, []string{"identity", "tenant"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependsOn=%v want=%v", got, want)
	}
	if got, want := loaded.Requirements.Databases, []string{"analytics", "primary"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("databases=%v want=%v", got, want)
	}
}

func TestLoadRejectsUnknownFieldsAndValidateRejectsSelfDependency(t *testing.T) {
	path := filepath.Join(t.TempDir(), Filename)
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"future":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field err=%v", err)
	}
	if err := ValidateForModule("access", Spec{SchemaVersion: 1, Version: "v0.1.0", DependsOn: []string{"access"}}); err == nil || !strings.Contains(err.Error(), "depends on itself") {
		t.Fatalf("self dependency err=%v", err)
	}
}
