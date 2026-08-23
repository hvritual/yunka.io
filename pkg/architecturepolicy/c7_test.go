package architecturepolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoloadDescriptorOnlyPolicy(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid.go")
	if err := os.WriteFile(valid, []byte(`package autoload
import (
  "yunka.io/framework/core/modulecatalog"
  orders "yunka.io/modules/orders"
)
func init() { modulecatalog.MustRegister(orders.GeneratedDescriptor()) }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := CheckAutoloadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}

	invalid := filepath.Join(root, "invalid.go")
	if err := os.WriteFile(invalid, []byte(`package autoload
import (
  "os"
  "yunka.io/framework/core/modulecatalog"
  orders "yunka.io/modules/orders"
)
func init() {
  _ = os.Getenv("SECRET")
  modulecatalog.MustRegister(orders.GeneratedDescriptor())
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err = CheckAutoloadFile(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("unsafe autoload accepted")
	}
}

func TestRepositoryC7Architecture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work")); os.IsNotExist(err) {
		t.Skip("repository root not available")
	}
	diagnostics, err := CheckC7(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("C7 architecture diagnostics=%#v", diagnostics)
	}
}

func TestAutoloadRejectsSpoofedCatalogAndSideEffectDescriptorCall(t *testing.T) {
	root := t.TempDir()
	cases := map[string]string{
		"spoof.go": `package autoload
import (
  modulecatalog "example.com/fake/catalog"
  orders "yunka.io/modules/orders"
)
func init() { modulecatalog.MustRegister(orders.GeneratedDescriptor()) }
`,
		"side_effect.go": `package autoload
import (
  "yunka.io/framework/core/modulecatalog"
  orders "yunka.io/modules/orders"
)
func init() { modulecatalog.MustRegister(orders.GeneratedDescriptor(readEnvironment())) }
`,
	}
	for name, source := range cases {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		diagnostics, err := CheckAutoloadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(diagnostics) == 0 {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestTypedCompositionRejectsSelectorServiceLocatorAndAliasedPool(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"framework/core/modulecatalog", "framework/kernel", "framework/platform", "framework/requestscope"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "framework/core/modulecatalog/bad.go"), []byte(`package modulecatalog
import (
  core "yunka.io/framework/core"
  synchronization "sync"
)
var pool synchronization.Pool
func bad() { _ = core.GetApp() }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := checkTypedComposition(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) < 2 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestTypedCompositionRejectsLegacyContainerImportsInC72Paths(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		"framework/core/modulecatalog",
		"framework/kernel",
		"framework/platform",
		"framework/requestscope",
	} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "framework/requestscope/bad.go"), []byte(`package requestscope
import (
  "yunka.io/framework/core/module"
  "yunka.io/framework/core/request"
  "yunka.io/pkg/di"
)
var _ = module.NewModule
var _ request.Runtime
var _ = di.Fill
`), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := checkTypedComposition(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) < 3 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}
