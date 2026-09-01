package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScaffoldCreatesIdempotentFourCommandProjectShape(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scaffold(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if report.BootstrapContract != "contracts/proto/"+bootstrapContractName {
		t.Fatalf("bootstrap contract=%q", report.BootstrapContract)
	}
	if report.BootstrapEntrypoint != "cmd/yunka-bootstrap/main.go" {
		t.Fatalf("bootstrap entrypoint=%q", report.BootstrapEntrypoint)
	}
	if report.DevManifest != ".yunka/dev.json" {
		t.Fatalf("dev manifest=%q", report.DevManifest)
	}
	tracked := []string{
		filepath.Join(root, "contracts", "proto", bootstrapContractName),
		filepath.Join(root, "cmd", "yunka-bootstrap", "main.go"),
		filepath.Join(root, ".yunka", "dev.json"),
	}
	before := make(map[string][]byte, len(tracked))
	for _, path := range tracked {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = contents
	}
	if _, err := Scaffold(root, config); err != nil {
		t.Fatal(err)
	}
	for _, path := range tracked {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before[path], after) {
			t.Fatalf("second scaffold mutated %s", path)
		}
	}
	manifest := string(before[filepath.Join(root, ".yunka", "dev.json")])
	if !strings.Contains(manifest, `"go"`) || !strings.Contains(manifest, `"./cmd/yunka-bootstrap"`) || !strings.Contains(manifest, `"http://127.0.0.1:8080/ready"`) {
		t.Fatalf("unexpected dev manifest: %s", manifest)
	}
}

func TestScaffoldUsesSingleExistingMainWithoutOverwritingIt(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	mainPath := filepath.Join(root, "cmd", "api", "main.go")
	writeProjectTestFile(t, mainPath, "package main\n\nfunc main() {}\n")
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scaffold(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if report.BootstrapEntrypoint != "" {
		t.Fatalf("unexpected bootstrap entrypoint %q", report.BootstrapEntrypoint)
	}
	contents, err := os.ReadFile(filepath.Join(root, ".yunka", "dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"./cmd/api"`) {
		t.Fatalf("existing main was not selected: %s", contents)
	}
	original, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "package main\n\nfunc main() {}\n" {
		t.Fatal("existing main was mutated")
	}
}

func TestScaffoldWithoutGoModuleLeavesDevManifestDeveloperOwned(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scaffold(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if report.DevManifest != "" || !strings.Contains(report.DevSkipped, "go.mod") {
		t.Fatalf("unexpected dev result: %#v", report)
	}
	if _, err := os.Stat(filepath.Join(root, "contracts", "proto", bootstrapContractName)); err != nil {
		t.Fatal(err)
	}
}

func writeProjectTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}
