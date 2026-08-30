package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverModuleBindingsUsesExplicitGeneratedFacts(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "arbitrary-directory")
	if err := os.MkdirAll(filepath.Join(moduleRoot, "autoload"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "module.go"), []byte("package device\n\nconst ModuleName = \"device\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "autoload", "register.go"), []byte(`package autoload
import (
    "yunka.io/framework/core/modulecatalog"
    module "example.com/product/modules/device"
)
func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	bindings, err := DiscoverModuleBindings(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
	binding := bindings[0]
	if binding.Name != "device" || binding.ImportPath != "example.com/product/modules/device" || binding.DescriptorSymbol != "GeneratedDescriptor" {
		t.Fatalf("binding lost explicit generated facts: %#v", binding)
	}
	if strings.Contains(binding.Name, "arbitrary-directory") {
		t.Fatalf("module identity was inferred from directory name: %#v", binding)
	}
}

func TestDiscoverModuleBindingsRejectsAutoloadWithoutGeneratedDescriptorCall(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "device")
	if err := os.MkdirAll(filepath.Join(moduleRoot, "autoload"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(moduleRoot, "module.go"), []byte("package device\nconst ModuleName = \"device\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(moduleRoot, "autoload", "register.go"), []byte(`package autoload
import module "example.com/product/modules/device"
var _ = module.GeneratedDescriptor
`), 0o644)
	_, err := DiscoverModuleBindings(root)
	if err == nil || !strings.Contains(err.Error(), "GeneratedDescriptor") {
		t.Fatalf("expected explicit registration failure, got %v", err)
	}
}

func TestValidateModuleBindingsRequiresExactAssemblySelection(t *testing.T) {
	bindings := []ModuleBinding{{Name: "device", ImportPath: "example.com/product/modules/device", DescriptorSymbol: "GeneratedDescriptor"}}
	if err := ValidateModuleBindings([]string{"device"}, bindings); err != nil {
		t.Fatal(err)
	}
	if err := ValidateModuleBindings([]string{"device", "site"}, bindings); err == nil || !strings.Contains(err.Error(), "site") {
		t.Fatalf("expected missing module binding failure, got %v", err)
	}
	if err := ValidateModuleBindings(nil, bindings); err == nil || !strings.Contains(err.Error(), "not selected") {
		t.Fatalf("expected extra module binding failure, got %v", err)
	}
}
