package implementation_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/change"
	"yunka.io/app/cmd/implementation"
	modulecmd "yunka.io/app/cmd/module"
	"yunka.io/app/cmd/projectflow"
)

// External test avoids making the authoring layer depend on its change checker.
func TestAG061StarterRemainsInsideCanonicalChangeScope(t *testing.T) {
	root := t.TempDir()
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "contracts/proto"), 0750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module example.com/books\n\ngo 1.25.0\n",
		"contracts/proto/shelf.proto": `syntax="proto3";package shelf.v1;
import "yunka/dsl/v1/options.proto";
option go_package="example.com/books/wire;shelfv1";
option (yunka.dsl.v1.domain)={name:"shelf" version:"v1"};
message Request{string id=1;} message Response{string id=1;}
service ShelfAPI{option (yunka.dsl.v1.application)={name:"catalog" operations:{id:"shelf.list" use_case:"list_books" public:true request_type:"shelf.v1.Request" response_type:"shelf.v1.Response" application_method:"ListBooks" execution:{transaction:TRANSACTION_NONE idempotency:IDEMPOTENCY_NONE}}};}
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0640); err != nil {
			t.Fatal(err)
		}
	}
	if err := modulecmd.GenerateWithOptions(modulecmd.Options{Name: "shelf", Root: filepath.Join(root, "modules"), NoConfig: true, Logger: false}); err != nil {
		t.Fatal(err)
	}
	options := projectflow.Options{Root: root, Protoc: protoc, ProtoPaths: []string{filepath.Join(repo, "contracts/proto")}}
	if _, err := projectflow.Generate(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	r, err := implementation.Run(context.Background(), implementation.Options{Project: options, Application: "shelf/catalog", CompositionPackage: "example.com/books/bootstrap"}, true)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := change.BuildWithOptions(options, "shelf.list", change.IntentImplementation, 3)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := projectflow.DescribeOwnershipInputs(options)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := projectflow.DescribeImplementationLayout(inputs.Project, "shelf", "catalog", "ListBooks")
	if err != nil {
		t.Fatal(err)
	}
	canonicalScope := filepath.ToSlash(filepath.Join(inputs.Project.GeneratedGoRoot, "shelf", "application"))
	for _, file := range r.Files {
		if !strings.HasPrefix(file.Path, canonicalScope+"/") {
			t.Fatalf("starter %s escapes canonical scope %s", file.Path, canonicalScope)
		}
	}
	if len(plan.UnresolvedTargets) != 0 {
		t.Fatalf("sealed starter must resolve its handler, unresolved=%#v", plan.UnresolvedTargets)
	}
	found := false
	for _, target := range plan.EditableTargets {
		if target.Path != layout.Handler {
			continue
		}
		found = true
		if !strings.HasPrefix(target.Path, canonicalScope+"/") {
			t.Fatalf("resolved handler %s escapes canonical scope %s", target.Path, canonicalScope)
		}
	}
	if !found {
		t.Fatalf("canonical sealed handler %s absent from editable targets %#v", layout.Handler, plan.EditableTargets)
	}
}
