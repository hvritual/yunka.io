package projectflow

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

func TestIssue160ProjectContextNamespaces(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required")
	}
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	support, err := os.ReadFile(filepath.Join(repository, "contracts", "proto", "yunka", "dsl", "v1", "options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, inventoryMode := range []bool{false, true} {
		name := "proto-root"
		if inventoryMode {
			name = "inventory"
		}
		t.Run(name, func(t *testing.T) {
			// The consumer project itself is nested in a containing directory. Paths
			// must remain relative to this project, never to process cwd or inventory.
			root := filepath.Join(t.TempDir(), "backend")
			writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/context\n\ngo 1.25.0\n")
			writeTestFile(t, filepath.Join(root, "support", "yunka", "dsl", "v1", "options.proto"), string(support))
			sourceRoot := "contracts/proto"
			if inventoryMode {
				sourceRoot = "apis/alpha"
			}
			dto := `syntax="proto3";package alpha.v1;message Request {} message Response {}`
			service := `syntax="proto3";package alpha.v1;import "dto.proto";import "yunka/dsl/v1/options.proto";
    option (yunka.dsl.v1.domain)={name:"alpha" version:"v1"};
    service API { option (yunka.dsl.v1.application)={name:"management"};
     rpc Echo(Request) returns(Response) { option (yunka.dsl.v1.operation)={id:"alpha.echo" use_case:"echo" public:true}; }
    }`
			writeTestFile(t, filepath.Join(root, sourceRoot, "dto.proto"), dto)
			writeTestFile(t, filepath.Join(root, sourceRoot, "service.proto"), service)
			options := Options{Root: root, Protoc: protoc, ProtoPaths: []string{filepath.Join(root, "support")}}
			if inventoryMode {
				// Inventory is under contracts/, but its roots are project-relative.
				inv := contractcore.SourceInventory{SchemaVersion: 1, SourceSets: []contractcore.SourceSet{{Name: "alpha", Root: sourceRoot, Files: []string{"dto.proto", "service.proto"}, ProtoPaths: []string{"support"}}}}
				data, err := json.Marshal(inv)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "contracts", "sources.json"), string(data))
			}
			before := issue160SourceContents(t, root)
			value, err := DescribeOperationContractContext(context.Background(), options, "alpha.echo")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{sourceRoot + "/dto.proto", sourceRoot + "/service.proto"}
			if !reflect.DeepEqual(value.SourceFiles, want) {
				t.Fatalf("sourceFiles=%v want=%v", value.SourceFiles, want)
			}
			values, err := DescribeOperationContractContexts(context.Background(), options)
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != 1 || !reflect.DeepEqual(values[0], value) {
				t.Fatalf("single/all disagree: %#v %#v", value, values)
			}
			again, err := DescribeOperationContractContext(context.Background(), options, "alpha.echo")
			if err != nil {
				t.Fatal(err)
			}
			first, _ := json.Marshal(value)
			second, _ := json.Marshal(again)
			if string(first) != string(second) || strings.Contains(string(first), root) {
				t.Fatalf("unstable or absolute context: %s / %s", first, second)
			}
			if !reflect.DeepEqual(before, issue160SourceContents(t, root)) {
				t.Fatal("context query mutated the project")
			}
		})
	}
}

func TestIssue160ProjectContextRejectsGuessingAndEscapes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "proto", "dto.proto"), "source")
	project := resolvedProject{Root: root, ProtoDir: filepath.Join(root, "proto")}
	for _, source := range []string{"../outside.proto", "/outside.proto", "./dto.proto", "a/../dto.proto", "C:/dto.proto", "a\\dto.proto", "missing.proto"} {
		if _, err := sourcePathForProject(project, source); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	project.InventoryPath = filepath.Join(root, "config", "sources.json")
	writeTestFile(t, filepath.Join(root, "config", "dto.proto"), "must not be guessed")
	if _, err := sourcePathForProject(project, "dto.proto"); err == nil {
		t.Fatal("guessed path relative to inventory")
	}
	outside := filepath.Join(t.TempDir(), "outside.proto")
	writeTestFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(root, "escape.proto")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := sourcePathForProject(project, "escape.proto"); err == nil {
		t.Fatal("accepted escaping symlink")
	}
}

func issue160SourceContents(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
