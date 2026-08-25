package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndCheck(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/biz\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{
		Name:        "device",
		Object:      "machine",
		Root:        internal,
		TablePrefix: "iot",
		RESTPrefix:  "/v1",
		Fields:      []string{"serial:string", "enabled:bool"},
	}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(internal, "device")
	po, err := os.ReadFile(filepath.Join(domainRoot, "infrastructure", "persistence", "zz_yunka_po_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(po), `return "iot_device_machine"`) {
		t.Fatalf("PO table naming was not generated: %s", po)
	}
	rest, err := os.ReadFile(filepath.Join(domainRoot, "transport", "rest", "zz_yunka_rest_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rest), "handler.service.CreateMachine") {
		t.Fatalf("REST adapter does not delegate to service: %s", rest)
	}
	rpc, err := os.ReadFile(filepath.Join(domainRoot, "transport", "rpc", "zz_yunka_rpc_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rpc), "server.service.CreateMachine") {
		t.Fatalf("RPC adapter does not delegate to service: %s", rpc)
	}
	wire, err := os.ReadFile(filepath.Join(domainRoot, "wire", "zz_yunka_wiring_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), "persistence.NewScopeFactory") || !strings.Contains(string(wire), "application.NewService") {
		t.Fatalf("wire bundle does not compose requestscope/service: %s", wire)
	}
	if err := parseGeneratedGo(domainRoot); err != nil {
		t.Fatal(err)
	}
	if err := Check(internal); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDetectsGeneratedDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/biz\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{Name: "tenant", Root: internal, NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(internal, "tenant", "application", "zz_yunka_contract_gen.go")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\n// drift\n")
	_ = file.Close()
	if err := Check(internal); err == nil || !strings.Contains(err.Error(), "generated drift") {
		t.Fatalf("expected generated drift error, got %v", err)
	}
}

func TestReservedFieldRejected(t *testing.T) {
	_, _, err := normalizeOptions(Options{Name: "device", Fields: []string{"tenant_id:string"}})
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved field error, got %v", err)
	}
}

func parseGeneratedGo(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		_, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
		return err
	})
}
