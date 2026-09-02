package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/pkg/modulespec"
)

func TestDeclarativeModuleCLIPathCreatesOnlyCanonicalSpec(t *testing.T) {
	root := t.TempDir()
	modules := filepath.Join(root, "modules")
	if err := AddSpec(SpecOptions{Name: "access", Root: modules, Databases: []string{"primary"}}); err != nil {
		t.Fatal(err)
	}
	moduleRoot := filepath.Join(modules, "access")
	entries, err := os.ReadDir(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != modulespec.Filename {
		t.Fatalf("unexpected declarative module files: %v", entries)
	}
	if err := RequireSpec(modules, "access", "logger", ""); err != nil {
		t.Fatal(err)
	}
	if err := RequireSpec(modules, "access", "dependency", "identity"); err != nil {
		t.Fatal(err)
	}
	if err := Check(modules); err != nil {
		t.Fatal(err)
	}
	output, err := ShowSpec(modules, "access")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Module: access", "database.primary", "logger", "identity", "Runtime build: none"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("show output missing %q:\n%s", expected, output)
		}
	}
}

func TestDeclarativeModuleRejectsLegacySourceMixing(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "access")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "module.go"), []byte("package access\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddSpec(SpecOptions{Name: "access", Root: filepath.Join(root, "modules")}); err == nil || !strings.Contains(err.Error(), "legacy") {
		t.Fatalf("mixed source err=%v", err)
	}
}
