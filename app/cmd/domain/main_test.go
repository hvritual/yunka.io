package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/project"
)

func TestGenerateAndCheck(t *testing.T) {
	root := newTestProject(t)
	if _, err := project.Initialize(root, "iot"); err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{
		Name:       "device",
		Object:     "machine",
		Root:       internal,
		RESTPrefix: "/v1",
		Fields:     []string{"serial:string", "enabled:bool"},
	}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(internal, "device")
	poBase, err := os.ReadFile(filepath.Join(domainRoot, "infrastructure", "persistence", "zz_yunka_po_base_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(poBase), `const machineTableName = "iot_device_machine"`) {
		t.Fatalf("PO table naming was not generated: %s", poBase)
	}
	po, err := os.ReadFile(filepath.Join(domainRoot, "infrastructure", "persistence", "po.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(po), "Application-owned; safe to edit") {
		t.Fatalf("editable PO scaffold was not generated: %s", po)
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
	if !strings.Contains(string(wire), "persistence.AutoMigrate") || !strings.Contains(string(wire), "persistence.NewScopeFactory") || !strings.Contains(string(wire), "application.NewService") {
		t.Fatalf("wire bundle does not auto-migrate/compose requestscope/service: %s", wire)
	}
	if err := parseGeneratedGo(domainRoot); err != nil {
		t.Fatal(err)
	}
	if err := Check(internal); err != nil {
		t.Fatal(err)
	}
}

func TestUninitializedProjectDefaultsToYK(t *testing.T) {
	root := newTestProject(t)
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{Name: "device", Root: internal, NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	config, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.TablePrefix != "yk" {
		t.Fatalf("project prefix=%q want yk", config.Database.TablePrefix)
	}
	spec, err := readManifest(filepath.Join(internal, "device"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.TableName != "yk_device_device" {
		t.Fatalf("table=%q want yk_device_device", spec.TableName)
	}
}

func TestEditablePOIsPreservedAcrossRegeneration(t *testing.T) {
	root := newTestProject(t)
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{Name: "device", Object: "machine", Root: internal, NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(internal, "device")
	poPath := filepath.Join(domainRoot, "infrastructure", "persistence", "po.go")
	contents, err := os.ReadFile(poPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(contents), "\t// Add application-owned PO fields below. Example:\n", "\tExternalCode string `gorm:\"column:external_code;type:varchar(64);index\"`\n\t// Add application-owned PO fields below. Example:\n", 1)
	if err := os.WriteFile(poPath, []byte(updated), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(poPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "ExternalCode string") {
		t.Fatalf("developer-owned PO field was overwritten: %s", after)
	}
	repository, err := os.ReadFile(filepath.Join(domainRoot, "infrastructure", "persistence", "zz_yunka_repository_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repository), "AutoMigrate(&MachinePO{})") {
		t.Fatalf("final editable PO is not the migration target: %s", repository)
	}
}

func TestCheckDetectsGeneratedDrift(t *testing.T) {
	root := newTestProject(t)
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
	_, _, err := normalizeOptions(Options{Name: "device", TablePrefix: "yk", Fields: []string{"tenant_id:string"}})
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved field error, got %v", err)
	}
}

func TestDomainCannotOverrideProjectTablePrefix(t *testing.T) {
	root := newTestProject(t)
	if _, err := project.Initialize(root, "biz"); err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{Name: "device", Root: internal, TablePrefix: "iot", NoRPC: true}); err == nil || !strings.Contains(err.Error(), "differs from project database prefix") {
		t.Fatalf("expected project table prefix error, got %v", err)
	}
}

func TestRegenerateRemovesStaleGeneratedTransport(t *testing.T) {
	root := newTestProject(t)
	internal := filepath.Join(root, "internal")
	if err := Generate(Options{Name: "device", Root: internal}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(internal, "device")
	manifestPath := filepath.Join(domainRoot, ManifestName)
	spec, err := readManifest(domainRoot)
	if err != nil {
		t.Fatal(err)
	}
	spec.REST.Enabled = false
	spec.RPC.Enabled = false
	if err := writeManifest(domainRoot, spec); err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(domainRoot, "transport", "rest", "zz_yunka_rest_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("expected stale REST generated file to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(domainRoot, "transport", "rpc", "device.proto")); !os.IsNotExist(err) {
		t.Fatalf("expected stale RPC proto to be removed, got %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest must be preserved: %v", err)
	}
}

func newTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/biz\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	return root
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
