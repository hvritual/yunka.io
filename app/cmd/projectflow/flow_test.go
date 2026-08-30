package projectflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectConventionalDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	project, err := resolveProject(Options{Root: root, ProtoPaths: []string{"third_party/proto"}})
	if err != nil {
		t.Fatal(err)
	}
	if project.GoModule != "example.com/demo" {
		t.Fatalf("GoModule=%q", project.GoModule)
	}
	if project.CodeImport != "example.com/demo/internal" {
		t.Fatalf("CodeImport=%q", project.CodeImport)
	}
	if project.InventoryPath != "" {
		t.Fatalf("InventoryPath=%q", project.InventoryPath)
	}
	if project.ContractOut != filepath.Join(root, "contracts", "generated") {
		t.Fatalf("ContractOut=%q", project.ContractOut)
	}
	if project.ModuleRoot != filepath.Join(root, "modules") {
		t.Fatalf("ModuleRoot=%q", project.ModuleRoot)
	}
	if project.CodeOut != filepath.Join(root, "internal") {
		t.Fatalf("CodeOut=%q", project.CodeOut)
	}
	if len(project.AdditionalProtoPaths) != 1 || project.AdditionalProtoPaths[0] != filepath.Join(root, "third_party", "proto") {
		t.Fatalf("AdditionalProtoPaths=%v", project.AdditionalProtoPaths)
	}
}

func TestResolveProjectPrefersSourceInventory(t *testing.T) {
	root := t.TempDir()
	inventory := filepath.Join(root, "contracts", "sources.json")
	if err := os.MkdirAll(filepath.Dir(inventory), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventory, []byte("{\"schemaVersion\":1,\"sourceSets\":[]}"), 0o640); err != nil {
		t.Fatal(err)
	}

	project, err := resolveProject(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if project.InventoryPath != inventory {
		t.Fatalf("InventoryPath=%q want %q", project.InventoryPath, inventory)
	}
}

func TestResolveProjectRequiresContractSource(t *testing.T) {
	_, err := resolveProject(Options{Root: t.TempDir()})
	if err == nil {
		t.Fatal("expected missing contract source error")
	}
}

func TestFormatStageOrder(t *testing.T) {
	report := Report{Stages: []Stage{
		{Name: "contract", Status: "generated", Detail: "ok"},
		{Name: "modules", Status: "skipped", Detail: "none"},
	}}
	got := Format(report)
	want := "GENERATED contract  ok\nSKIPPED   modules   none\n"
	if got != want {
		t.Fatalf("Format()=%q want %q", got, want)
	}
}
