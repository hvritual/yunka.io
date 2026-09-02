package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/modulespec"
)

func TestDiscoverModuleSnapshotReadsCanonicalModuleSpecWithoutGoBinding(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "access")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := modulespec.Default()
	spec.DependsOn = []string{"identity"}
	spec.Requirements = modulespec.Requirements{Logger: true, Databases: []string{"primary"}}
	data, err := modulespec.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, modulespec.Filename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	modules, bindings, err := DiscoverModuleSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 || len(bindings) != 0 {
		t.Fatalf("modules=%#v bindings=%#v", modules, bindings)
	}
	module := modules[0]
	if module.Name != "access" || module.Evidence.Source != modulespec.EvidenceSource || module.Evidence.Ref != "access/module.yunka.json" {
		t.Fatalf("unexpected module evidence: %#v", module)
	}
	if len(module.DependsOn) != 1 || module.DependsOn[0] != "identity" || len(module.Requirements.Databases) != 1 || module.Requirements.Databases[0] != "primary" || !module.Requirements.Logger {
		t.Fatalf("module facts lost: %#v", module)
	}
}

func TestDiscoverModuleSnapshotRejectsDeclarativeAndLegacyFactSourcesTogether(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "access")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := modulespec.Marshal(modulespec.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, modulespec.Filename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "module.go"), []byte("package access\nconst ModuleName = \"access\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = DiscoverModuleSnapshot(root)
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("mixed fact source err=%v", err)
	}
}
