package project

import (
	"os"
	"path/filepath"
	"testing"

	"yunka.io/app/cmd/sourceaudit"
)

func TestAG062InitCreatesStableDefaultSourcePolicy(t *testing.T) {
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
	if report.SourcePolicy != SourcePolicyRelativePath || report.SourcePolicySkipped != "" {
		t.Fatalf("source policy report=%#v", report)
	}
	path := filepath.Join(root, filepath.FromSlash(SourcePolicyRelativePath))
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := sourceaudit.ReadPolicy(contents)
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Modules) != 1 || policy.Modules[0].Path != "." || policy.Modules[0].Workspace != "off" {
		t.Fatalf("default modules=%#v", policy.Modules)
	}
	if len(policy.Components) != 1 || policy.Components[0].Path != "." || policy.Components[0].Kind != "production" || !policy.Components[0].AllowExternal {
		t.Fatalf("default components=%#v", policy.Components)
	}
	custom := []byte("{\"developer\":\"owned\"}\n")
	if err := os.WriteFile(path, custom, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := Scaffold(root, config); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(custom) {
		t.Fatal("yunka init overwrote developer source policy")
	}
}

func TestAG062InitWithoutGoModuleDoesNotCreateInvalidPolicy(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Scaffold(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if report.SourcePolicy != "" || report.SourcePolicySkipped == "" {
		t.Fatalf("source policy report=%#v", report)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(SourcePolicyRelativePath))); !os.IsNotExist(err) {
		t.Fatalf("source policy unexpectedly created: %v", err)
	}
}
