package module

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateCreatesTypedCapabilityModule(t *testing.T) {
	root := t.TempDir()
	writeTestGoMod(t, root, "example.com/service")
	withWorkingDirectory(t, root)
	options := Options{
		Name:      "orders",
		Root:      "modules",
		Logger:    true,
		Databases: []string{"analytics", "primary"},
		EventBus:  true,
		RPC:       []string{"inventory"},
		DependsOn: []string{"tenant"},
	}
	if err := GenerateWithOptions(options); err != nil {
		t.Fatal(err)
	}
	for _, relative := range requiredModuleFiles {
		path := filepath.Join(root, "modules", "orders", relative)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			t.Fatalf("%s parse: %v", relative, err)
		}
	}
	dependencies := readTestFile(t, root, "modules/orders/dependencies.go")
	normalizedDependencies := strings.Join(strings.Fields(dependencies), " ")
	for _, expected := range []string{
		"Config Config",
		"Logger logExt.Logger",
		"AnalyticsDatabase *gorm.DB",
		"PrimaryDatabase *gorm.DB",
		"EventBus eventBus.EventBus",
		"InventoryRPC grpc.ClientConnInterface",
	} {
		if !strings.Contains(normalizedDependencies, expected) {
			t.Fatalf("dependencies missing %q:\n%s", expected, dependencies)
		}
	}
	wiring := readTestFile(t, root, "modules/orders/zz_yunka_module_gen.go")
	normalizedWiring := strings.Join(strings.Fields(wiring), " ")
	for _, expected := range []string{
		`ConfigKey: "modules.orders"`,
		`{Name: "analytics"}`,
		`{Name: "primary"}`,
		`EventBus: true`,
		`{Name: "inventory"}`,
		`DependsOn: []string{"tenant"}`,
	} {
		if !strings.Contains(normalizedWiring, expected) {
			t.Fatalf("wiring missing %q:\n%s", expected, wiring)
		}
	}
	if err := Check("modules"); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for _, root := range []string{first, second} {
		writeTestGoMod(t, root, "example.com/service")
		working, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		err = GenerateWithOptions(Options{
			Name: "orders", Root: "modules", Logger: true,
			Databases: []string{"primary", "analytics", "primary"},
			RPC:       []string{"inventory", "inventory"},
			EventBus:  true, DependsOn: []string{"tenant"},
		})
		if restoreErr := os.Chdir(working); restoreErr != nil {
			t.Fatal(restoreErr)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if left, right := snapshotTree(t, filepath.Join(first, "modules", "orders")), snapshotTree(t, filepath.Join(second, "modules", "orders")); !reflect.DeepEqual(left, right) {
		t.Fatalf("generated trees differ\nleft=%v\nright=%v", left, right)
	}
}

func TestGenerateRejectsUnsafeExistingAndCollidingTargets(t *testing.T) {
	root := t.TempDir()
	writeTestGoMod(t, root, "example.com/service")
	withWorkingDirectory(t, root)
	if err := Generate("../escape"); err == nil {
		t.Fatal("unsafe name accepted")
	}
	if err := Generate("orders"); err != nil {
		t.Fatal(err)
	}
	if err := Generate("orders"); err == nil {
		t.Fatal("existing target overwritten")
	}
	err := GenerateWithOptions(Options{
		Name: "collision", Root: "modules", Logger: true,
		Databases: []string{"foo-bar", "foo.bar"},
	})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("collision error=%v", err)
	}
}

func TestCheckRejectsBrokenAutoload(t *testing.T) {
	root := t.TempDir()
	writeTestGoMod(t, root, "example.com/service")
	withWorkingDirectory(t, root)
	if err := Generate("orders"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "modules", "orders", "autoload", "register.go")
	if err := os.WriteFile(path, []byte("package autoload\n\nfunc init() { go func() {}() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check("modules"); err == nil {
		t.Fatal("broken autoload accepted")
	}
}

func withWorkingDirectory(t *testing.T, root string) {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(working); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func writeTestGoMod(t *testing.T, root, modulePath string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = readTestFile(t, root, relative)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
