package agentcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
)

func TestIssue160ContextBootstrapDoesNotCompile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "contracts/proto/broken.proto"), "not valid protobuf")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PROTOC", filepath.Join(root, "no-protoc"))
	before, _ := treeDigest(root)
	output, err := runContextCLI("--root", root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	value := decodeContext(t, output)
	if value.SchemaVersion != 5 || value.ContractContext != nil || strings.Contains(output, `"contractContext"`) {
		t.Fatalf("bootstrap invented compiled context: %s", output)
	}
	if value.AgentProtocol.OperationContext != "yunka context --operation <operation> --json" || value.AgentProtocol.AllOperationContexts != "yunka context --all-operations --json" {
		t.Fatalf("missing discovery: %#v", value.AgentProtocol)
	}
	after, _ := treeDigest(root)
	if before != after {
		t.Fatal("bootstrap wrote project files")
	}
}

func TestIssue160ContextCLIRealProtocNamespaces(t *testing.T) {
	protoc := contextProtoc(t)
	for _, inventory := range []bool{false, true} {
		t.Run(fmt.Sprintf("inventory=%t", inventory), func(t *testing.T) {
			root, args, expected := contextFixture(t, inventory, protoc)
			before, _ := treeDigest(root)
			query := append(append([]string{}, args...), "--operation", "alpha.echo", "--json")
			first, err := runContextCLI(query...)
			if err != nil {
				t.Fatal(err)
			}
			second, err := runContextCLI(query...)
			if err != nil || first != second {
				t.Fatalf("non-deterministic output: err=%v\n%s\n%s", err, first, second)
			}
			value := decodeContext(t, first)
			if value.ContractContext == nil || value.ContractContext.Authority != "read_only" || value.ContractContext.Scope != "canonical_file_import_closure" {
				t.Fatalf("wrong context authority: %s", first)
			}
			if len(value.ContractContext.Operations) != 1 {
				t.Fatalf("scoped query returned other Operations: %s", first)
			}
			op := value.ContractContext.Operations[0]
			if op.OperationID != "alpha.echo" || op.Service != "alpha.v1.API" || !reflect.DeepEqual(op.SourceFiles, expected) {
				t.Fatalf("wrong source closure: %#v, want %v", op, expected)
			}
			if !reflect.DeepEqual(op.ExternalImports, []string{"google/protobuf/timestamp.proto", "yunka/dsl/v1/options.proto"}) {
				t.Fatalf("external imports lost or mistaken for local files: %v", op.ExternalImports)
			}
			for _, path := range op.SourceFiles {
				if strings.Contains(path, "beta") || filepath.IsAbs(path) {
					t.Fatalf("unrelated or absolute source: %q", path)
				}
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(first, root) || strings.Contains(first, "do-not-disclose-this-payload") || strings.Contains(first, `"editablePaths"`) {
				t.Fatalf("context leaked payload/absolute root or invented authority: %s", first)
			}
			all, err := runContextCLI(append(append([]string{}, args...), "--all-operations", "--json")...)
			if err != nil {
				t.Fatal(err)
			}
			operations := decodeContext(t, all).ContractContext.Operations
			ids := []string{}
			for _, item := range operations {
				ids = append(ids, item.OperationID)
			}
			if !reflect.DeepEqual(ids, []string{"alpha.echo", "alpha.inspect", "beta.echo"}) || !reflect.DeepEqual(operations[0], op) || !reflect.DeepEqual(operations[1].SourceFiles, expected) {
				t.Fatalf("all/single/internal disagree: %#v", operations)
			}
			allAgain, err := runContextCLI(append(append([]string{}, args...), "--all-operations", "--json")...)
			if err != nil || all != allAgain {
				t.Fatalf("all-operation output not deterministic: %v", err)
			}
			text, err := runContextCLI(append(append([]string{}, args...), "--operation", "alpha.inspect")...)
			if err != nil || !strings.Contains(text, "authority=read_only") || !strings.Contains(text, "OPERATION alpha.inspect service=alpha.v1.API") || !strings.Contains(text, "EXTERNAL IMPORT google/protobuf/timestamp.proto") {
				t.Fatalf("bad human output: %s; %v", text, err)
			}
			after, _ := treeDigest(root)
			if before != after {
				t.Fatal("CLI changed project or generated files")
			}
		})
	}
}

func TestIssue160ContextCLIRecompilesAfterSourceMove(t *testing.T) {
	protoc := contextProtoc(t)
	for _, inventory := range []bool{false, true} {
		t.Run(fmt.Sprintf("inventory=%t", inventory), func(t *testing.T) {
			root, args, expected := contextFixture(t, inventory, protoc)
			query := append(append([]string{}, args...), "--operation", "alpha.echo", "--json")
			if _, err := runContextCLI(query...); err != nil {
				t.Fatal(err)
			}
			old := expected[0]
			moved := strings.Replace(old, "dto.proto", "moved.proto", 1)
			if err := os.Rename(filepath.Join(root, old), filepath.Join(root, moved)); err != nil {
				t.Fatal(err)
			}
			service := filepath.Join(root, expected[1])
			contents, err := os.ReadFile(service)
			if err != nil {
				t.Fatal(err)
			}
			mustWrite(t, service, strings.Replace(string(contents), "dto.proto", "moved.proto", 1))
			if inventory {
				path := filepath.Join(root, "contracts/sources.json")
				value, err := contractcore.LoadSourceInventory(path)
				if err != nil {
					t.Fatal(err)
				}
				for i := range value.SourceSets {
					if value.SourceSets[i].Name == "alpha" {
						value.SourceSets[i].Files = []string{"service.proto", "moved.proto"}
					}
				}
				data, _ := json.Marshal(value)
				mustWrite(t, path, string(data))
			}
			before, _ := treeDigest(root)
			output, err := runContextCLI(query...)
			if err != nil {
				t.Fatal(err)
			}
			expected[0] = moved
			if !reflect.DeepEqual(decodeContext(t, output).ContractContext.Operations[0].SourceFiles, expected) {
				t.Fatalf("used stale generated provenance after move: %s", output)
			}
			after, _ := treeDigest(root)
			if before != after {
				t.Fatal("read-only query regenerated artifacts")
			}
		})
	}
}

func TestIssue160ContextCLIFailsClosed(t *testing.T) {
	protoc := contextProtoc(t)
	root, args, _ := contextFixture(t, false, protoc)
	for _, extra := range [][]string{
		{"--operation", ""}, {"--operation", "   "},
		{"--operation", "alpha.echo", "--all-operations"},
		{"--operation", "does.not.exist"},
		{"--operation", "alpha.*"},
		{"--operation", "alpha.echo", "unexpected"},
		{"--operation", "alpha.echo", "--proto-path", ""},
		{"--operation", "alpha.echo", "--proto-path="},
		{"--operation", "alpha.echo", "--proto-path", "   "},
	} {
		t.Run(strings.Join(extra, "/"), func(t *testing.T) {
			before, _ := treeDigest(root)
			output, err := runContextCLI(append(append(append([]string{}, args...), "--json"), extra...)...)
			if err == nil || output != "" {
				t.Fatalf("query did not fail closed: %v; stdout=%q", err, output)
			}
			after, _ := treeDigest(root)
			if before != after {
				t.Fatal("failed query mutated project")
			}
		})
	}
	for _, flags := range [][]string{{"--protoc", protoc}, {"--proto-path", "support"}} {
		if output, err := runContextCLI(append([]string{"--root", root, "--json"}, flags...)...); err == nil || output != "" {
			t.Fatalf("accepted ignored compiler options: %v %q", err, output)
		}
	}
	_, inventoryArgs, _ := contextFixture(t, true, protoc)
	if output, err := runContextCLI(append(inventoryArgs, "--operation", "alpha.echo", "--proto-path", "support")...); err == nil || !strings.Contains(err.Error(), "sourceSets[].protoPaths") || output != "" {
		t.Fatalf("silently ignored inventory include override: %v %q", err, output)
	}
	mustWrite(t, filepath.Join(root, "contracts/proto/alpha/dto.proto"), "invalid protobuf")
	if output, err := runContextCLI(append(args, "--operation", "alpha.echo", "--json")...); err == nil || !strings.Contains(err.Error(), "protoc failed") || output != "" {
		t.Fatalf("fell back to stale artifacts or full context: %v %q", err, output)
	}
}

func TestIssue160ContextCLIEmptyOperationSet(t *testing.T) {
	protoc := contextProtoc(t)
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "contracts/proto/dto.proto"), `syntax="proto3"; message DTO { string name=1; }`)
	output, err := runContextCLI("--root", root, "--all-operations", "--protoc", protoc, "--json")
	if err != nil {
		t.Fatal(err)
	}
	value := decodeContext(t, output)
	if value.ContractContext == nil || len(value.ContractContext.Operations) != 0 || !strings.Contains(output, `"operations": []`) {
		t.Fatalf("empty inventory not explicitly represented: %s", output)
	}
}

func TestIssue160ContextBuildOptionsCancellationAndValidation(t *testing.T) {
	var paths protoPathValues
	for _, path := range []string{"support,extra", ""} {
		if err := paths.Set(path); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual([]string(paths), []string{"support,extra", ""}) {
		t.Fatalf("include flag silently transformed explicit input: %#v", paths)
	}
	for _, options := range []Options{
		{Operation: "   "}, {Operation: "alpha.echo", AllOperations: true}, {Protoc: "unused"},
	} {
		if value, err := BuildWithOptions(nil, options); err == nil || value.ContractContext != nil {
			t.Fatalf("invalid options accepted: %#v %v", value, err)
		}
	}
	protoc := contextProtoc(t)
	root, _, _ := contextFixture(t, false, protoc)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := BuildWithOptions(ctx, Options{Root: root, Operation: "alpha.echo", Protoc: protoc, ProtoPaths: []string{"support"}}); err == nil || value.ContractContext != nil {
		t.Fatalf("cancelled query returned context: %#v %v", value, err)
	}
}

func runContextCLI(args ...string) (string, error) {
	app := cli.NewApp()
	app.Name = "yunka"
	app.Commands = []cli.Command{Command()}
	var output bytes.Buffer
	app.Writer = &output
	app.ErrWriter = &bytes.Buffer{}
	err := app.Run(append([]string{"yunka", "context"}, args...))
	return output.String(), err
}

func decodeContext(t *testing.T, contents string) Snapshot {
	t.Helper()
	var value Snapshot
	if err := json.Unmarshal([]byte(contents), &value); err != nil {
		t.Fatalf("not one valid JSON document: %v\n%s", err, contents)
	}
	return value
}

func contextProtoc(t *testing.T) string {
	t.Helper()
	name := os.Getenv("PROTOC")
	if name == "" {
		name = "protoc"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skip("real protoc is required; qualification must install the locked compiler")
	}
	return path
}

func contextFixture(t *testing.T, inventory bool, protoc string) (string, []string, []string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "nested", "backend")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/context\n\ngo 1.25.0\n")
	support, err := os.ReadFile("../../../contracts/proto/yunka/dsl/v1/options.proto")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "support/yunka/dsl/v1/options.proto"), string(support))
	alpha, beta, shared := "contracts/proto/alpha", "contracts/proto/beta", "contracts/proto/shared/common.proto"
	alphaDTOImport, betaDTOImport := "alpha/dto.proto", "beta/dto.proto"
	args := []string{"--root", root, "--protoc", protoc}
	if inventory {
		alpha, beta, shared = "apis/alpha", "apis/beta", "deps/shared/common.proto"
		alphaDTOImport, betaDTOImport = "dto.proto", "dto.proto"
		value := contractcore.SourceInventory{SchemaVersion: 1, SourceSets: []contractcore.SourceSet{
			{Name: "beta", Root: beta, Files: []string{"dto.proto", "service.proto"}, ProtoPaths: []string{"support"}},
			{Name: "shared", Root: "deps", Files: []string{"shared/common.proto"}},
			{Name: "alpha", Root: alpha, Files: []string{"service.proto", "dto.proto"}, ProtoPaths: []string{"deps", "support"}},
		}}
		data, _ := json.Marshal(value)
		mustWrite(t, filepath.Join(root, "contracts/sources.json"), string(data))
	} else {
		args = append(args, "--proto-path", "support")
	}
	mustWrite(t, filepath.Join(root, alpha, "dto.proto"), `syntax="proto3"; package alpha.v1; import "shared/common.proto"; message Request { shared.v1.Metadata metadata=1; } message Response { string name=1; }`)
	mustWrite(t, filepath.Join(root, shared), `syntax="proto3"; package shared.v1; import "google/protobuf/timestamp.proto"; message Metadata { enum Kind { UNSPECIFIED=0; ACTIVE=1; } message Detail { string value=1; } map<string, Detail> details=1; Kind kind=2; google.protobuf.Timestamp time=3; }`)
	mustWrite(t, filepath.Join(root, beta, "dto.proto"), `syntax="proto3"; package beta.v1; message Request {} message Response { string value=1; }`)
	for _, item := range []struct{ name, directory, dto string }{{"alpha", alpha, alphaDTOImport}, {"beta", beta, betaDTOImport}} {
		internal := ""
		if item.name == "alpha" {
			internal = `operations: { id:"alpha.inspect" use_case:"inspect" public:true request_type:"alpha.v1.Request" response_type:"alpha.v1.Response" application_method:"Inspect" }`
		}
		service := fmt.Sprintf(`syntax="proto3"; package %s.v1; import %q; import "yunka/dsl/v1/options.proto";
option (yunka.dsl.v1.domain)={name:%q version:"v1"};
service API { option (yunka.dsl.v1.application)={name:"management" %s};
rpc Echo(Request) returns(Response) { option (yunka.dsl.v1.operation)={id:%q use_case:"echo" public:true}; }}
`, item.name, item.dto, item.name, internal, item.name+".echo")
		mustWrite(t, filepath.Join(root, item.directory, "service.proto"), service)
	}
	// Stale/unparseable generated metadata must never be used as source truth.
	mustWrite(t, filepath.Join(root, "contracts/generated/manifest.json"), "stale do-not-disclose-this-payload")
	mustWrite(t, filepath.Join(root, "contracts/generated/operation-plans.json"), "stale operation-plans")
	return root, args, []string{alpha + "/dto.proto", alpha + "/service.proto", shared}
}
