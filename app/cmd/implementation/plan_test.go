package implementation

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/applicationboundary"
	"yunka.io/app/cmd/projectflow"
)

func fixture(root string) (projectflow.ProjectDescriptor, contract.Manifest) {
	p := projectflow.ProjectDescriptor{Root: root, GoModule: "example.com/library", GeneratedGoRoot: "src/app", GeneratedGoImport: "example.com/library/src/app"}
	m := contract.Manifest{SchemaVersion: contract.ManifestVersion,
		Files:    []contract.File{{Name: "shelf.proto", Package: "shelf.v1", GoPackage: "example.com/library/wire;shelfv1", Domain: &contract.DomainDeclaration{Name: "shelf", Version: "v1"}}},
		Messages: []contract.Message{{Name: "Request", FullName: "shelf.v1.Request"}, {Name: "Response", FullName: "shelf.v1.Response"}},
		Services: []contract.Service{{Name: "ShelfAPI", FullName: "shelf.v1.ShelfAPI", Domain: "shelf", Application: &contract.ApplicationDeclaration{Name: "catalog", Operations: []contract.OperationDeclaration{{ID: "shelf.list", UseCase: "list_books", Public: true, RequestType: "shelf.v1.Request", ResponseType: "shelf.v1.Response", ApplicationMethod: "ListBooks", Execution: &contract.ExecutionPolicy{Transaction: "none", Idempotency: "none"}}}}}},
	}
	return p, m
}
func mustPlan(t *testing.T, root string) Report {
	t.Helper()
	p, m := fixture(root)
	r, err := render(p, m, "shelf/catalog", p.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func put(t *testing.T, root, name, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0640); err != nil {
		t.Fatal(err)
	}
}
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(name string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if e.Type()&os.ModeSymlink != 0 {
			value, err := os.Readlink(name)
			out[rel] = "link:" + value
			return err
		}
		b, err := os.ReadFile(name)
		out[rel] = digest(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAG061CanonicalStarterShape(t *testing.T) {
	root := t.TempDir()
	before := snapshot(t, root)
	r := mustPlan(t, root)
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("render wrote files")
	}
	if r.Mode != "plan" || !strings.HasSuffix(r.Contract, ".ShelfAPI") || len(r.Files) != 7 {
		t.Fatalf("unexpected shape: %+v", r)
	}
	for _, f := range r.Files {
		if f.SHA256 != digest([]byte(f.Content)) || f.Action != "create" {
			t.Fatalf("bad file evidence: %+v", f)
		}
		if strings.Contains(f.Path, "zz_yunka_") || strings.Contains(f.Content, contract.GeneratedApplicationMarker) {
			t.Fatalf("starter masquerades as generated: %s", f.Path)
		}
		if strings.HasSuffix(f.Path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, parser.AllErrors); err != nil {
				t.Fatal(err)
			}
		}
		if strings.HasSuffix(f.Path, "_handler.go") && !strings.Contains(f.Content, ": not implemented") {
			t.Fatal("invented successful behavior")
		}
		if strings.HasSuffix(f.Path, "architecture.types.json") {
			p, err := applicationboundary.ReadPolicy(strings.NewReader(f.Content))
			if err != nil {
				t.Fatal(err)
			}
			if p.Factories[0].Results[0].Contract.Name != "ShelfAPI" || len(p.Factories[0].AllowedCallers) != 1 || p.Factories[0].AllowedCallers[0] != "example.com/library/bootstrap" {
				t.Fatal("wrong canonical policy")
			}
		}
	}
	b, _ := json.Marshal(r)
	again := mustPlan(t, root)
	a, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatal("render not deterministic")
	}
}
func TestAG061MultiApplicationNamingComesFromRenderer(t *testing.T) {
	p, m := fixture(t.TempDir())
	m.Services = append(m.Services, contract.Service{Name: "AuditAPI", FullName: "shelf.v1.AuditAPI", Domain: "shelf", Application: &contract.ApplicationDeclaration{Name: "inspection", Operations: []contract.OperationDeclaration{{ID: "shelf.inspect", UseCase: "inspect", Public: true, RequestType: "shelf.v1.Request", ResponseType: "shelf.v1.Response", ApplicationMethod: "Inspect", Execution: &contract.ExecutionPolicy{Transaction: "none", Idempotency: "none"}}}}})
	r, err := render(p, m, "shelf/catalog", p.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(r.Contract, ".CatalogApplication") {
		t.Fatalf("not canonical multi-application name: %s", r.Contract)
	}
	for _, f := range r.Files {
		if strings.Contains(f.Content, "ShelfAPI") {
			t.Fatalf("stale single-application identity in %s", f.Path)
		}
	}
}
func TestAG061UnsupportedInputsNeverWrite(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(*projectflow.ProjectDescriptor, *contract.Manifest)
		key, caller string
	}{
		{"missing", func(*projectflow.ProjectDescriptor, *contract.Manifest) {}, "shelf/absent", "example.com/library/bootstrap"},
		{"traversal", func(*projectflow.ProjectDescriptor, *contract.Manifest) {}, "shelf/../catalog", "example.com/library/bootstrap"},
		{"external_caller", func(*projectflow.ProjectDescriptor, *contract.Manifest) {}, "shelf/catalog", "outside.com/app"},
		{"invalid_caller", func(*projectflow.ProjectDescriptor, *contract.Manifest) {}, "shelf/catalog", "example.com/library/?bad"},
		{"owner_self", func(*projectflow.ProjectDescriptor, *contract.Manifest) {}, "shelf/catalog", "example.com/library/src/app/shelf/application/catalog"},
		{"wrong_import_root", func(p *projectflow.ProjectDescriptor, _ *contract.Manifest) { p.GeneratedGoImport = "wrong.com/root" }, "shelf/catalog", "example.com/library/bootstrap"},
		{"outside_root", func(p *projectflow.ProjectDescriptor, _ *contract.Manifest) { p.GeneratedGoRoot = "../outside" }, "shelf/catalog", "example.com/library/bootstrap"},
		{"capabilities", func(_ *projectflow.ProjectDescriptor, m *contract.Manifest) {
			m.Services[0].Application.Capabilities = []contract.CapabilityRequirement{{Name: "db", Package: "example.com/db", Type: "DB"}}
		}, "shelf/catalog", "example.com/library/bootstrap"},
		{"requires", func(_ *projectflow.ProjectDescriptor, m *contract.Manifest) {
			m.Services[0].Application.Requires = []string{"shelf/other"}
		}, "shelf/catalog", "example.com/library/bootstrap"},
		{"operation_requires", func(_ *projectflow.ProjectDescriptor, m *contract.Manifest) {
			m.Services[0].Application.Operations[0].RequiresOperations = []string{"other.get"}
		}, "shelf/catalog", "example.com/library/bootstrap"},
		{"empty", func(_ *projectflow.ProjectDescriptor, m *contract.Manifest) {
			m.Services[0].Application.Operations = nil
		}, "shelf/catalog", "example.com/library/bootstrap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p, m := fixture(root)
			tc.change(&p, &m)
			if _, err := render(p, m, tc.key, tc.caller); err == nil {
				t.Fatal("unsupported input accepted")
			}
			if len(snapshot(t, root)) != 0 {
				t.Fatal("validation wrote files")
			}
		})
	}
	if _, err := Run(context.Background(), Options{Application: "bad"}, true); err == nil {
		t.Fatal("invalid public request accepted")
	}
}
func TestAG061PreflightProtectsEveryTarget(t *testing.T) {
	for _, kind := range []string{"conflict", "symlink-file", "symlink-parent", "directory", "parent-file"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			r := mustPlan(t, root)
			target := r.Files[len(r.Files)-1].Path
			external := t.TempDir()
			put(t, external, "keep", "external")
			switch kind {
			case "conflict":
				put(t, root, target, "user changes")
			case "directory":
				if err := os.MkdirAll(filepath.Join(root, target), 0750); err != nil {
					t.Fatal(err)
				}
			case "parent-file":
				put(t, root, "src/app", "user file")
			case "symlink-file":
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, target)), 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(external, "keep"), filepath.Join(root, target)); err != nil {
					t.Fatal(err)
				}
			case "symlink-parent":
				if err := os.Symlink(external, filepath.Join(root, "src")); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, root)
			outside := snapshot(t, external)
			opened, err := os.OpenRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.Close()
			if err := preflight(opened, &r); err == nil {
				t.Fatal("unsafe target accepted")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outside, snapshot(t, external)) {
				t.Fatal("preflight mutated files")
			}
		})
	}
}
func TestAG061ExistingIdenticalFilesRemainDeveloperOwned(t *testing.T) {
	root := t.TempDir()
	r := mustPlan(t, root)
	for _, f := range r.Files {
		put(t, root, f.Path, f.Content)
	}
	before := snapshot(t, root)
	opened, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := preflight(opened, &r); err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Files {
		if f.Action != "unchanged" {
			t.Fatalf("unexpected rewrite of %s", f.Path)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("idempotent preflight wrote")
	}
}
func TestAG061SignatureAmbiguityFailsClosed(t *testing.T) {
	p, m := fixture(t.TempDir())
	generated, err := contract.RenderC9ApplicationCode(m, contract.ApplicationCodeOptions{RootImport: p.GeneratedGoImport})
	if err != nil {
		t.Fatal(err)
	}
	var port contract.GeneratedApplicationFile
	for _, f := range generated {
		if strings.HasSuffix(f.Path, "_application_port_gen.go") {
			port = f
		}
	}
	dup := port
	dup.Path = "shelf/application/zz_yunka_other_application_port_gen.go"
	if _, err := selectPort(append(generated, dup), m.Services[0]); err == nil {
		t.Fatal("ambiguous canonical identity guessed")
	}
	selected, err := selectPort(generated, m.Services[0])
	if err != nil {
		t.Fatal(err)
	}
	selected.methods[0].Names = []*ast.Ident{ast.NewIdent("hidden")}
	if _, err := starterFiles(selected, "example.com/library/owner", "example.com/library/ports", "shelf/catalog", "example.com/library/bootstrap"); err == nil {
		t.Fatal("unimplementable unexported method accepted")
	}
}
