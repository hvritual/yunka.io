package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Kept independent of new helper APIs so the same test can prove RED on the
// previous provenance substrate once its unrelated compiler truncation is fixed.
func TestIssue160InventoryIdentityRegression(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		writeProtoFixture(t, filepath.Join(root, name, "common.proto"), fmt.Sprintf(`syntax="proto3"; package %s.v1; message Request {} message Response {}`, name))
		writeProtoFixture(t, filepath.Join(root, name, "service.proto"), fmt.Sprintf(`syntax="proto3"; package %s.v1; import "common.proto"; service API { rpc Echo(Request) returns(Response); }`, name))
	}
	inventory := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{
		{Name: "alpha", Root: "alpha", Files: []string{"common.proto", "service.proto"}},
		{Name: "beta", Root: "beta", Files: []string{"common.proto", "service.proto"}},
	}}
	result, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inventory), Protoc: testProtoc(t)})
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range result.Manifest.Messages {
		want := strings.Split(message.FullName, ".")[0] + "/common.proto"
		if message.SourceFile != want {
			t.Fatalf("ISSUE160_INVENTORY_IDENTITY: %s source=%q want=%q", message.FullName, message.SourceFile, want)
		}
	}
	for _, service := range result.Manifest.Services {
		want := strings.Split(service.FullName, ".")[0] + "/service.proto"
		if service.SourceFile != want || service.Methods[0].SourceFile != want {
			t.Fatalf("service provenance=%#v", service)
		}
	}
}

func issue160Support(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contracts", "proto", "yunka", "dsl", "v1", "options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	writeProtoFixture(t, filepath.Join(root, "support", "yunka", "dsl", "v1", "options.proto"), string(data))
}

func issue160Service(pkg, importName string) string {
	return fmt.Sprintf(`syntax="proto3";
 package %s.v1;
 import "yunka/dsl/v1/options.proto";
 import %q;
 option (yunka.dsl.v1.domain)={name:%q version:"v1"};
 service API {
  option (yunka.dsl.v1.application)={name:"management"};
  rpc Echo(Request) returns(Response) {
   option (yunka.dsl.v1.operation)={id:%q use_case:"echo" public:true};
  }
 }`, pkg, importName, pkg, pkg+".echo")
}

func TestIssue160InventorySharedClosureMoveAndDeterminism(t *testing.T) {
	root := t.TempDir()
	issue160Support(t, root)
	writeProtoFixture(t, filepath.Join(root, "shared", "model.proto"), `syntax="proto3";package shared.v1; message Metadata { enum Kind { UNKNOWN=0; TEXT=1; } Kind kind=1; }`)
	writeProtoFixture(t, filepath.Join(root, "alpha", "common.proto"), `syntax="proto3";package alpha.v1;import "model.proto"; message Request { map<string,shared.v1.Metadata> metadata=1; } message Response { shared.v1.Metadata.Kind kind=1; }`)
	writeProtoFixture(t, filepath.Join(root, "alpha", "service.proto"), issue160Service("alpha", "common.proto"))
	writeProtoFixture(t, filepath.Join(root, "beta", "common.proto"), `syntax="proto3";package beta.v1;message Request {} message Response {}`)
	writeProtoFixture(t, filepath.Join(root, "beta", "service.proto"), issue160Service("beta", "common.proto"))
	inv := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{
		{Name: "alpha", Root: "alpha", Files: []string{"service.proto", "common.proto"}, ProtoPaths: []string{"support", "shared"}},
		{Name: "beta", Root: "beta", Files: []string{"common.proto", "service.proto"}, ProtoPaths: []string{"support"}},
		{Name: "shared", Root: "shared", Files: []string{"model.proto"}},
	}}
	compile := func() CompileResult {
		t.Helper()
		result, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inv), Protoc: testProtoc(t)})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := compile()
	want := []string{"alpha/common.proto", "alpha/service.proto", "shared/model.proto"}
	resolved, err := ResolveOperationContractContext(first.Manifest, "alpha.echo")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resolved.SourceFiles, want) {
		t.Fatalf("closure=%v want=%v", resolved.SourceFiles, want)
	}
	if !reflect.DeepEqual(resolved.ExternalImports, []string{"yunka/dsl/v1/options.proto"}) {
		t.Fatalf("external imports=%v", resolved.ExternalImports)
	}
	if len(first.Manifest.Enums) != 1 || first.Manifest.Enums[0].SourceFile != "shared/model.proto" {
		t.Fatalf("nested enum provenance=%#v", first.Manifest.Enums)
	}
	one, err := RenderArtifacts(first.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(one.Manifest, []byte(`"schemaVersion": 5`)) {
		t.Fatalf("typed manifest not v5: %s", one.Manifest)
	}
	out := filepath.Join(root, "out")
	if err := WriteArtifacts(out, one); err != nil {
		t.Fatal(err)
	}
	inv.SourceSets[0], inv.SourceSets[2] = inv.SourceSets[2], inv.SourceSets[0]
	second := compile()
	two, err := RenderArtifacts(second.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckArtifacts(out, two); err != nil || len(drift) != 0 {
		t.Fatalf("reordered generation drift=%v err=%v", drift, err)
	}
	if first.DescriptorSHA != second.DescriptorSHA {
		t.Fatal("inventory order changed descriptor identity")
	}
	for i := range inv.SourceSets {
		if inv.SourceSets[i].Name == "alpha" {
			inv.SourceSets[i].Files = []string{"model/common.proto", "service.proto"}
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "alpha", "model"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "alpha", "common.proto"), filepath.Join(root, "alpha", "model", "common.proto")); err != nil {
		t.Fatal(err)
	}
	writeProtoFixture(t, filepath.Join(root, "alpha", "service.proto"), issue160Service("alpha", "model/common.proto"))
	moved := compile()
	after, err := RenderArtifacts(moved.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.OperationPlans, after.OperationPlans) || !bytes.Equal(one.OpenAPI, after.OpenAPI) || !bytes.Equal(one.TypeScript, after.TypeScript) {
		t.Fatal("source move changed semantic artifacts")
	}
	resolved, err = ResolveOperationContractContext(moved.Manifest, "alpha.echo")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"alpha/model/common.proto", "alpha/service.proto", "shared/model.proto"}
	if !reflect.DeepEqual(resolved.SourceFiles, want) {
		t.Fatalf("moved closure=%v", resolved.SourceFiles)
	}
}

func TestIssue160InventoryImportOrderAndExternalCollision(t *testing.T) {
	t.Run("ordered include roots", func(t *testing.T) {
		root := t.TempDir()
		writeProtoFixture(t, filepath.Join(root, "api", "service.proto"), `syntax="proto3";package api.v1;import "model.proto";message Local {}`)
		for _, name := range []string{"first", "second"} {
			writeProtoFixture(t, filepath.Join(root, name, "model.proto"), fmt.Sprintf(`syntax="proto3";package %s.v1;message Model {}`, name))
		}
		inv := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{{Name: "api", Root: "api", Files: []string{"service.proto"}, ProtoPaths: []string{"second", "first"}}, {Name: "first", Root: "first", Files: []string{"model.proto"}}, {Name: "second", Root: "second", Files: []string{"model.proto"}}}}
		for _, want := range []string{"second/model.proto", "first/model.proto"} {
			result, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inv), Protoc: testProtoc(t)})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Manifest.Files[0].Dependencies, []string{want}) {
				t.Fatalf("include selection=%#v want %s", result.Manifest.Files[0], want)
			}
			inv.SourceSets[0].ProtoPaths = []string{"first", "second"}
		}
	})
	t.Run("external name cannot impersonate owned path", func(t *testing.T) {
		root := t.TempDir()
		issue160Support(t, root)
		writeProtoFixture(t, filepath.Join(root, "api", "common.proto"), `syntax="proto3";package api.v1;import "owned/model.proto";message Request {} message Response {}`)
		writeProtoFixture(t, filepath.Join(root, "api", "service.proto"), issue160Service("api", "common.proto"))
		writeProtoFixture(t, filepath.Join(root, "includes", "owned", "model.proto"), `syntax="proto3";package external.v1;message External {}`)
		writeProtoFixture(t, filepath.Join(root, "owned", "model.proto"), `syntax="proto3";package unrelated.v1;message Unrelated {}`)
		inv := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{{Name: "api", Root: "api", Files: []string{"common.proto", "service.proto"}, ProtoPaths: []string{"support", "includes"}}, {Name: "unrelated", Root: "owned", Files: []string{"model.proto"}}}}
		result, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inv), Protoc: testProtoc(t)})
		if err != nil {
			t.Fatal(err)
		}
		value, err := ResolveOperationContractContext(result.Manifest, "api.echo")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value.SourceFiles, []string{"api/common.proto", "api/service.proto"}) {
			t.Fatalf("external import widened local scope: %#v", value)
		}
		if !reflect.DeepEqual(value.ExternalImports, []string{"owned/model.proto", "yunka/dsl/v1/options.proto"}) {
			t.Fatalf("external imports=%v", value.ExternalImports)
		}
	})
}

func TestIssue160InventoryRejectsPhysicalAliasAndEscape(t *testing.T) {
	t.Run("duplicate physical owner", func(t *testing.T) {
		root := t.TempDir()
		writeProtoFixture(t, filepath.Join(root, "api", "one.proto"), `syntax="proto3";message One {}`)
		if err := os.Symlink(filepath.Join(root, "api"), filepath.Join(root, "alias")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		inv := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{{Name: "api", Root: "api", Files: []string{"one.proto"}}, {Name: "alias", Root: "alias", Files: []string{"one.proto"}}}}
		_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inv), Protoc: "unused"})
		if err == nil || !strings.Contains(err.Error(), "duplicate physical source ownership") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("escaping source symlink", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.proto")
		writeProtoFixture(t, outside, `syntax="proto3";message Outside {}`)
		if err := os.MkdirAll(filepath.Join(root, "api"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "api", "one.proto")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		inv := SourceInventory{SchemaVersion: 1, SourceSets: []SourceSet{{Name: "api", Root: "api", Files: []string{"one.proto"}}}}
		_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: root, InventoryPath: writeInventory(t, root, "sources.json", inv), Protoc: "unused"})
		if err == nil || !strings.Contains(err.Error(), "escapes repository") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestIssue160LegacyProjectionAndSchemaCompatibility(t *testing.T) {
	root := t.TempDir()
	writeProtoFixture(t, filepath.Join(root, "api.proto"), `syntax="proto3";package legacy.v1;message Request {} message Response {} service API {rpc Echo(Request) returns(Response);}`)
	result, err := Compile(context.Background(), CompileOptions{Dir: root, Protoc: testProtoc(t)})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(result.Manifest)
	artifacts, err := RenderArtifacts(result.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(artifacts.Manifest, []byte(`sourceFile`)) || bytes.Contains(artifacts.Manifest, []byte(`dependencies`)) {
		t.Fatalf("legacy output changed: %s", artifacts.Manifest)
	}
	after, _ := json.Marshal(result.Manifest)
	if !bytes.Equal(before, after) {
		t.Fatal("legacy render destroyed in-memory provenance")
	}
	for _, version := range []int{1, 2, 3, 4} {
		path := filepath.Join(root, fmt.Sprintf("v%d.json", version))
		writeProtoFixture(t, path, fmt.Sprintf(`{"schemaVersion":%d,"files":[],"messages":[],"enums":[],"services":[]}`, version))
		if _, err := LoadManifest(path); err != nil {
			t.Fatal(err)
		}
	}
}
