package boundary

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

func TestBoundaryInspectBuildsFreshDeterministicReadOnlyProjection(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeBoundaryFixture(t, filepath.Join(root, "go.mod"), "module example.com/boundary\n\ngo 1.25.0\n")
	writeBoundaryFixture(t, filepath.Join(root, "contracts", "proto", "service.proto"), boundaryFixtureProto())
	writeBoundaryFixture(t, filepath.Join(root, "contracts", "generated", "manifest.json"), "{\"schemaVersion\":1}\n")

	before := boundaryFixtureContents(t, root)
	options := projectflow.Options{
		Root:       root,
		Protoc:     protoc,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
	}
	first, err := Build(context.Background(), options, "alpha/management")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), options, "alpha/management")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated inspections differ:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.Authority != "read_only" || first.Fingerprint.Application != "alpha/management" || first.Fingerprint.Service != "alpha.v1.API" {
		t.Fatalf("unexpected inspection identity: %#v", first)
	}
	if first.IntentCoverage.State != "declared" || !reflect.DeepEqual(first.IntentCoverage.Contexts, []string{"alpha.management"}) || !reflect.DeepEqual(first.IntentCoverage.Aggregates, []string{"account"}) {
		t.Fatalf("unexpected intent coverage: %#v", first.IntentCoverage)
	}
	if len(first.Fingerprint.Operations) != 1 || first.Fingerprint.Operations[0].Plan.OperationID != "alpha.echo" {
		t.Fatalf("unexpected operations: %#v", first.Fingerprint.Operations)
	}
	if first.FingerprintDigest == "" || first.OperationPlansDigest == "" {
		t.Fatalf("missing deterministic digests: %#v", first)
	}

	jsonOne, err := Render(first, "json")
	if err != nil {
		t.Fatal(err)
	}
	jsonTwo, err := Render(second, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	if jsonOne != jsonTwo {
		t.Fatalf("json/agent-json projections differ:\n%s\n%s", jsonOne, jsonTwo)
	}
	var decoded boundarycore.Inspection
	if err := json.Unmarshal([]byte(jsonOne), &decoded); err != nil {
		t.Fatal(err)
	}
	reencoded, err := Render(decoded, "json")
	if err != nil {
		t.Fatal(err)
	}
	if reencoded != jsonOne {
		t.Fatalf("JSON projection is not canonical after round-trip:\nfirst=%s\nagain=%s", jsonOne, reencoded)
	}
	text, err := Render(first, "text")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"BOUNDARY authority=read_only application=alpha/management service=alpha.v1.API",
		"INTENT state=declared",
		"alpha.echo context=\"alpha.management\" aggregate=\"account\"",
		"NOT_EVALUATED ",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("text projection missing %q:\n%s", required, text)
		}
	}
	if !reflect.DeepEqual(before, boundaryFixtureContents(t, root)) {
		t.Fatal("boundary inspect mutated project files")
	}
}

func TestBoundaryInspectRejectsInvalidApplicationWithoutMutation(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeBoundaryFixture(t, filepath.Join(root, "go.mod"), "module example.com/boundary\n\ngo 1.25.0\n")
	writeBoundaryFixture(t, filepath.Join(root, "contracts", "proto", "service.proto"), boundaryFixtureProto())
	before := boundaryFixtureContents(t, root)

	_, err = Build(context.Background(), projectflow.Options{
		Root:       root,
		Protoc:     protoc,
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
	}, "alpha")
	if err == nil || !strings.Contains(err.Error(), "application must be <domain>/<application>") {
		t.Fatalf("invalid application escaped validation: %v", err)
	}
	if !reflect.DeepEqual(before, boundaryFixtureContents(t, root)) {
		t.Fatal("failed boundary inspect mutated project files")
	}
}

func TestBoundaryCommandSurfaceIsReadOnly(t *testing.T) {
	command := Command()
	if command.Name != "boundary" || len(command.Subcommands) != 1 || command.Subcommands[0].Name != "inspect" {
		t.Fatalf("unexpected command surface: %#v", command)
	}
	for _, forbidden := range []string{"apply", "write", "repair", "merge", "generate"} {
		for _, subcommand := range command.Subcommands {
			if subcommand.Name == forbidden {
				t.Fatalf("boundary command exposed mutation verb %q", forbidden)
			}
		}
	}
	flags := map[string]bool{}
	for _, flag := range command.Subcommands[0].Flags {
		flags[strings.Split(flag.GetName(), ",")[0]] = true
	}
	for _, required := range []string{"root", "application", "protoc", "proto-path", "format"} {
		if !flags[required] {
			t.Fatalf("boundary inspect missing flag %q: %#v", required, flags)
		}
	}
}

func TestBoundaryRenderRejectsUnknownFormat(t *testing.T) {
	if _, err := Render(boundarycore.Inspection{}, "yaml"); err == nil {
		t.Fatal("unsupported output format was accepted")
	}
}

func boundaryFixtureProto() string {
	return "syntax = \"proto3\";\n" +
		"package alpha.v1;\n" +
		"import \"yunka/dsl/v1/options.proto\";\n" +
		"option (yunka.dsl.v1.domain) = { name: \"alpha\" version: \"v1\" };\n" +
		"message Request {}\n" +
		"message Response {}\n" +
		"service API {\n" +
		"  option (yunka.dsl.v1.application) = { name: \"management\" };\n" +
		"  rpc Echo(Request) returns (Response) {\n" +
		"    option (yunka.dsl.v1.operation) = {\n" +
		"      id: \"alpha.echo\"\n" +
		"      use_case: \"echo\"\n" +
		"      public: true\n" +
		"      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }\n" +
		"      boundary: { context: \"alpha.management\" aggregate: \"account\" }\n" +
		"    };\n" +
		"  }\n" +
		"}\n"
}

func writeBoundaryFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func boundaryFixtureContents(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
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
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
