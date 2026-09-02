package applicationgraph

import (
	"testing"
	"github.com/hvritual/yunka.io/pkg/contract"
)

func TestContractCompositionCreatesDependencyEdges(t *testing.T) {
	manifest := contract.Manifest{SchemaVersion: contract.ManifestVersion, Files: []contract.File{{Name: "a.proto", Domain: &contract.DomainDeclaration{Name: "a"}}, {Name: "b.proto", Domain: &contract.DomainDeclaration{Name: "b"}}}, Services: []contract.Service{
		{Name: "A", FullName: "a.v1.A", Domain: "a", Application: &contract.ApplicationDeclaration{Name: "query"}, Methods: []contract.Method{{Name: "Get", FullName: "a.v1.A.Get", Operation: &contract.OperationDeclaration{ID: "a.get", UseCase: "get", Permissions: []string{"a.read"}, PermissionMode: "all"}}}},
		{Name: "B", FullName: "b.v1.B", Domain: "b", Application: &contract.ApplicationDeclaration{Name: "compose", Requires: []string{"a/query"}}, Methods: []contract.Method{{Name: "Do", FullName: "b.v1.B.Do", Operation: &contract.OperationDeclaration{ID: "b.do", UseCase: "do", Permissions: []string{"a.read", "b.write"}, PermissionMode: "all", RequiresOperations: []string{"a.get"}}}}},
	}}
	builder := NewBuilder()
	if err := AddContract(builder, manifest); err != nil {
		t.Fatal(err)
	}
	graph, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	wantApp := false
	wantOp := false
	for _, edge := range graph.Edges {
		if edge.Kind == EdgeDependsOn && edge.From == ID(NodeApplication, "b/compose") && edge.To == ID(NodeApplication, "a/query") {
			wantApp = true
		}
		if edge.Kind == EdgeDependsOn && edge.From == ID(NodeOperation, "b.do") && edge.To == ID(NodeOperation, "a.get") {
			wantOp = true
		}
	}
	if !wantApp || !wantOp {
		t.Fatalf("dependency edges app=%v op=%v graph=%#v", wantApp, wantOp, graph.Edges)
	}
}
