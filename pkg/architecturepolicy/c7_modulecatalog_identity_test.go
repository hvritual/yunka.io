package architecturepolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoloadAcceptsProvenModuleCatalogCompatibilityIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "register.go")
	source := `package autoload
import (
  module "github.com/hvritual/biz/modules/access"
  "yunka.io/framework/core/modulecatalog"
)
func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := CheckAutoloadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("compatibility modulecatalog identity rejected: %#v", diagnostics)
	}
}

func TestAutoloadRejectsUnprovenModuleCatalogSuffixIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "register.go")
	source := `package autoload
import (
  module "github.com/hvritual/biz/modules/access"
  modulecatalog "example.com/framework/core/modulecatalog"
)
func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
`
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := CheckAutoloadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("unproven modulecatalog suffix identity unexpectedly accepted")
	}
}
