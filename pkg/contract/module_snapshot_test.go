package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverModuleSnapshotReadsGeneratedDescriptorFacts(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "not-the-module-name")
	if err := os.MkdirAll(filepath.Join(moduleRoot, "autoload"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(moduleRoot, "module.go"), "package device\nconst ModuleName = \"device\"\n")
	write(filepath.Join(moduleRoot, "autoload", "register.go"), `package autoload
import (
    "yunka.io/framework/core/modulecatalog"
    module "example.com/product/modules/device"
)
func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
`)
	write(filepath.Join(moduleRoot, "zz_yunka_module_gen.go"), `package device
import "yunka.io/framework/core/modulecatalog"
func GeneratedDescriptor() modulecatalog.Descriptor {
    return modulecatalog.Descriptor{
        Name: ModuleName,
        Version: "v0.1.0",
        DependsOn: []string{"site"},
        Requirements: modulecatalog.Requirements{
            ConfigKey: "modules.device",
            Logger: true,
            Databases: []modulecatalog.DatabaseRequirement{{Name:"primary"}},
            EventBus: true,
            RPC: []modulecatalog.RPCRequirement{{Name:"inventory"}},
        },
        Build: generatedBuild,
    }
}
`)
	modules, bindings, err := DiscoverModuleSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 || len(bindings) != 1 {
		t.Fatalf("modules=%#v bindings=%#v", modules, bindings)
	}
	module := modules[0]
	if module.Name != "device" || module.Version != "v0.1.0" || len(module.DependsOn) != 1 || module.DependsOn[0] != "site" {
		t.Fatalf("descriptor identity/dependency lost: %#v", module)
	}
	if module.Requirements.ConfigKey != "modules.device" || !module.Requirements.Logger || !module.Requirements.EventBus || len(module.Requirements.Databases) != 1 || module.Requirements.Databases[0] != "primary" || len(module.Requirements.RPC) != 1 || module.Requirements.RPC[0] != "inventory" {
		t.Fatalf("descriptor requirements lost: %#v", module.Requirements)
	}
	if bindings[0].ImportPath != "example.com/product/modules/device" {
		t.Fatalf("binding import lost: %#v", bindings[0])
	}
}

func TestParseGeneratedDescriptorFailsClosedOnUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zz_yunka_module_gen.go")
	if err := os.WriteFile(path, []byte(`package device
import "yunka.io/framework/core/modulecatalog"
func GeneratedDescriptor() modulecatalog.Descriptor {
    return modulecatalog.Descriptor{
        Name: ModuleName,
        Version: "v0.1.0",
        Requirements: modulecatalog.Requirements{},
        Build: generatedBuild,
        FutureCapability: true,
    }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parseGeneratedDescriptor(path, "device")
	if err == nil || !strings.Contains(err.Error(), "unsupported field FutureCapability") {
		t.Fatalf("expected fail-closed descriptor schema error, got %v", err)
	}
}
