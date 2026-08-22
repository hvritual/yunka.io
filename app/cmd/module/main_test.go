package module

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesTypedAutoloadModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/service\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(working)
	if err := Generate("orders"); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"config.go", "dependencies.go", "module.go", "zz_yunka_module_gen.go", "autoload/register.go"} {
		path := filepath.Join(root, "modules", "orders", relative)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			t.Fatalf("%s parse: %v", relative, err)
		}
	}
	autoload, err := os.ReadFile(filepath.Join(root, "modules", "orders", "autoload", "register.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(autoload)
	if !strings.Contains(text, `module "example.com/service/modules/orders"`) ||
		!strings.Contains(text, "modulecatalog.MustRegister(module.GeneratedDescriptor())") {
		t.Fatalf("autoload=%s", text)
	}
	if strings.Contains(text, "os.Getenv") || strings.Contains(text, "GetApp") || strings.Contains(text, "NewModule(") {
		t.Fatalf("autoload contains runtime composition: %s", text)
	}
}

func TestGenerateRejectsUnsafeOrExistingTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/service\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	working, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(working)
	if err := Generate("../escape"); err == nil {
		t.Fatal("unsafe name accepted")
	}
	if err := Generate("orders"); err != nil {
		t.Fatal(err)
	}
	if err := Generate("orders"); err == nil {
		t.Fatal("existing target overwritten")
	}
}
