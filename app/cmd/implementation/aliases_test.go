package implementation

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/applicationboundary"
)

// This compiles canonical ports plus the same explicit DTO shape projection as
// the existing executable-boundary test. Only the legal go_package alias varies;
// it does not add a second PB generator or claim runtime qualification.
func TestAG061CanonicalImportAliasesDoNotCollide(t *testing.T) {
	for _, alias := range []string{"service", "New", "handleListBooks", "ctx", "request", "s", "_startertype0", "ordinary"} {
		t.Run(alias, func(t *testing.T) {
			root := t.TempDir()
			p, m := fixture(root)
			m.Files[0].GoPackage = "example.com/library/wire;" + alias
			r, err := render(p, m, "shelf/catalog", p.GoModule+"/bootstrap")
			if err != nil {
				t.Fatal(err)
			}
			put(t, root, "go.mod", "module example.com/library\n\ngo 1.25.0\n")
			put(t, root, "wire/model.go", "package "+alias+"\ntype Request struct{}\ntype Response struct{}\n")
			generated, err := contract.RenderC9ApplicationCode(m, contract.ApplicationCodeOptions{RootImport: p.GeneratedGoImport})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range generated {
				if strings.HasSuffix(f.Path, "_application_port_gen.go") {
					put(t, root, p.GeneratedGoRoot+"/"+f.Path, string(f.Content))
				}
			}
			var policy applicationboundary.Policy
			for _, f := range r.Files {
				put(t, root, f.Path, f.Content)
				if strings.HasSuffix(f.Path, "architecture.types.json") {
					policy, err = applicationboundary.ReadPolicy(strings.NewReader(f.Content))
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			put(t, root, "bootstrap/build.go", `package bootstrap
import (owner "example.com/library/src/app/shelf/application/catalog"; port "example.com/library/src/app/shelf/application")
func BuildCatalog()port.ShelfAPI{return owner.Build()}
`)
			put(t, root, "bootstrap/build_test.go", `package bootstrap
import("context";"strings";"testing")
func TestExplicitUnimplemented(t *testing.T){r,err:=BuildCatalog().ListBooks(context.Background(),nil);if r!=nil||err==nil||!strings.Contains(err.Error(),"not implemented"){t.Fatalf("invented behavior: %v %v",r,err)}}
`)
			before := snapshot(t, root)
			if out, err := childGo(t, root, "test", "-mod=readonly", "-count=1", "./..."); err != nil {
				t.Fatalf("legal canonical alias %s cannot compile: %v\n%s", alias, err, out)
			}
			checked := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
			if checked.Status != applicationboundary.Pass || checked.CheckedReferences != 1 {
				t.Fatalf("alias changed the canonical capability type: %+v", checked)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("compilation or type checking mutated source")
			}
		})
	}
}

func TestAG061AliasNormalizationPreservesCanonicalInput(t *testing.T) {
	p, m := fixture(t.TempDir())
	m.Files[0].GoPackage = "example.com/library/wire;service"
	generated, err := contract.RenderC9ApplicationCode(m, contract.ApplicationCodeOptions{RootImport: p.GeneratedGoImport})
	if err != nil {
		t.Fatal(err)
	}
	port, err := selectPort(generated, m.Services[0])
	if err != nil {
		t.Fatal(err)
	}
	source := append([]byte(nil), port.source...)
	before, err := expression(port.methods[0].Type)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]string{}
	for k, v := range port.imports {
		imports[k] = v
	}
	first, err := starterFiles(port, p.GoModule+"/owner", p.GeneratedGoImport+"/shelf/application", "shelf/catalog", p.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	second, err := starterFiles(port, p.GoModule+"/owner", p.GeneratedGoImport+"/shelf/application", "shelf/catalog", p.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	after, err := expression(port.methods[0].Type)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || before != after || !reflect.DeepEqual(imports, port.imports) || !bytes.Equal(source, port.source) {
		t.Fatal("starter alias projection mutated canonical input or made rerender differ")
	}
}
