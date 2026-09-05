package projectflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

func TestModuleIdentityFailureUsesStableDiagnostic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "service.go")
	writeModuleIdentityTestFile(t, path, "package internal\nimport _ \"yunka.io/framework/core/modulecatalog\"\n")

	err := requireCanonicalModuleIdentity(root)
	if err == nil {
		t.Fatal("expected legacy module identity failure")
	}
	item := Diagnose(err)
	if item.Code != diagnostic.CodeModuleIdentity || item.Stage != "module-identity" {
		t.Fatalf("diagnostic=%#v", item)
	}
	if item.Location == nil || item.Location.Path != "internal/service.go" {
		t.Fatalf("location=%#v", item.Location)
	}
	if len(item.Actions) != 2 || item.Actions[1].Value != "yunka dependency module-identity migrate" {
		t.Fatalf("actions=%#v", item.Actions)
	}
}

func TestGenerateRejectsLegacyIdentityBeforeMutation(t *testing.T) {
	root := t.TempDir()
	writeModuleIdentityTestFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeModuleIdentityTestFile(t, filepath.Join(root, "internal", "service.go"), "package internal\nimport _ \"yunka.io/gateway/authz\"\n")

	_, err := Generate(nil, Options{Root: root})
	if err == nil {
		t.Fatal("expected generate to fail before contract compilation")
	}
	item := Diagnose(err)
	if item.Code != diagnostic.CodeModuleIdentity {
		t.Fatalf("diagnostic=%#v", item)
	}
	if _, statErr := os.Stat(filepath.Join(root, "contracts", "generated")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy identity preflight must not create generated output: stat=%v", statErr)
	}
}

func TestCanonicalModuleIdentityPassesPreflight(t *testing.T) {
	root := t.TempDir()
	writeModuleIdentityTestFile(t, filepath.Join(root, "service.go"), "package demo\nimport _ \"github.com/hvritual/yunka.io/framework/core/modulecatalog\"\n")
	if err := requireCanonicalModuleIdentity(root); err != nil {
		t.Fatal(err)
	}
}

func writeModuleIdentityTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
