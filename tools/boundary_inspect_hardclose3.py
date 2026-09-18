#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path):
    return (ROOT / path).read_text()

def write(path, text):
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text)

def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:180]!r}")
    write(path, text.replace(old, new, 1))

write("app/cmd/boundary/command.go", r'''package boundary

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "boundary"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect canonical Service Boundary evidence without mutating the project",
		Subcommands: []cli.Command{
			inspectCommand(),
		},
	}
}

func inspectCommand() cli.Command {
	return cli.Command{
		Name:  "inspect",
		Usage: "compile current canonical protobuf sources and project one Application boundary",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "application", Usage: "canonical Application identity in <domain>/<application> form"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary used to compile current canonical sources"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protobuf include path; may be repeated"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			application := strings.TrimSpace(c.String("application"))
			if application == "" {
				return fmt.Errorf("boundary inspect: --application is required")
			}
			inspection, err := Build(context.Background(), projectflow.Options{
				Root:       c.String("root"),
				Protoc:     c.String("protoc"),
				ProtoPaths: append([]string(nil), c.StringSlice("proto-path")...),
			}, application)
			if err != nil {
				return err
			}
			output, err := Render(inspection, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

// Build is a read-only adapter over the canonical source compiler and
// boundarycore.Inspect. It never reads generated manifest artifacts and never
// persists the inspection.
func Build(ctx context.Context, options projectflow.Options, application string) (boundarycore.Inspection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot, err := projectflow.DescribeContractSourceSnapshot(ctx, options)
	if err != nil {
		return boundarycore.Inspection{}, fmt.Errorf("boundary inspect: compile current canonical source: %w", err)
	}
	inspection, err := boundarycore.Inspect(snapshot.Manifest, application)
	if err != nil {
		return boundarycore.Inspection{}, err
	}
	return inspection, nil
}

func Render(inspection boundarycore.Inspection, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return renderText(inspection), nil
	case "json", "agent-json":
		contents, err := json.MarshalIndent(inspection, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	default:
		return "", fmt.Errorf("boundary inspect: unsupported format %q; use text, json, or agent-json", format)
	}
}

func renderText(inspection boundarycore.Inspection) string {
	fingerprint := inspection.Fingerprint
	var builder strings.Builder
	fmt.Fprintf(&builder, "BOUNDARY authority=%s application=%s service=%s source=%s\n",
		inspection.Authority, fingerprint.Application, fingerprint.Service, fingerprint.ServiceSource)
	fmt.Fprintf(&builder, "DIGEST fingerprint=%s operationPlans=%s\n",
		inspection.FingerprintDigest, inspection.OperationPlansDigest)
	fmt.Fprintf(&builder, "INTENT state=%s declared=%d unknown=%d contexts=%s aggregates=%s\n",
		inspection.IntentCoverage.State,
		len(inspection.IntentCoverage.DeclaredOperations),
		len(inspection.IntentCoverage.UnknownOperations),
		joinOrDash(inspection.IntentCoverage.Contexts),
		joinOrDash(inspection.IntentCoverage.Aggregates),
	)
	fmt.Fprintf(&builder, "OPERATIONS %d\n", len(fingerprint.Operations))
	for _, operation := range fingerprint.Operations {
		contextValue, aggregateValue := operationIntent(operation)
		fmt.Fprintf(&builder, "  %s context=%q aggregate=%q transaction=%s idempotency=%s public=%t\n",
			operation.Plan.OperationID,
			contextValue,
			aggregateValue,
			operation.Plan.Execution.Transaction,
			operation.Plan.Execution.Idempotency,
			operation.Plan.Security.Public,
		)
	}
	fmt.Fprintf(&builder, "DEPENDENCIES %d\n", len(fingerprint.DependencyOperations))
	for _, operation := range fingerprint.DependencyOperations {
		contextValue, aggregateValue := operationIntent(operation)
		fmt.Fprintf(&builder, "  %s context=%q aggregate=%q application=%s/%s\n",
			operation.Plan.OperationID,
			contextValue,
			aggregateValue,
			operation.Plan.Domain,
			operation.Plan.Application,
		)
	}
	notEvaluated := append([]string(nil), inspection.NotEvaluated...)
	sort.Strings(notEvaluated)
	fmt.Fprintf(&builder, "NOT_EVALUATED %s\n", joinOrDash(notEvaluated))
	return builder.String()
}

func operationIntent(operation boundarycore.OperationEvidence) (string, string) {
	if operation.Boundary == nil {
		return "<unknown>", "<unknown>"
	}
	aggregate := strings.TrimSpace(operation.Boundary.Aggregate)
	if aggregate == "" {
		aggregate = "n/a:" + strings.TrimSpace(operation.Boundary.AggregateNotApplicableReason)
	}
	return strings.TrimSpace(operation.Boundary.Context), aggregate
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
''')

write("app/cmd/boundary/command_test.go", r'''package boundary

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
	if !reflect.DeepEqual(decoded, first) {
		t.Fatalf("JSON projection changed inspection facts: %#v %#v", decoded, first)
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
''')

replace_once(
    "app/cmd/yunka.go",
    '''	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/change"''',
    '''	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/boundary"
	"yunka.io/app/cmd/change"''')

replace_once(
    "app/cmd/yunka.go",
    '''		audit.Command(),
		change.Command(),''',
    '''		audit.Command(),
		boundary.Command(),
		change.Command(),''')

replace_once(
    "app/cmd/discoverability.go",
    '''	"audit":      categoryDiagnostics,
	"context":    categoryDiagnostics,''',
    '''	"audit":      categoryDiagnostics,
	"boundary":   categoryDiagnostics,
	"context":    categoryDiagnostics,''')

replace_once(
    "app/cmd/discoverability.go",
    '''Use audit for read-only deterministic framework-conformance evidence; findings report existing debt but do not block by default.''',
    '''Use boundary inspect for a fresh read-only canonical Service Boundary projection of one Application; the projection and digests are evidence only and do not authorize mutation or decide Operation Growth. Use audit for read-only deterministic framework-conformance evidence; findings report existing debt but do not block by default.''')

replace_once(
    "app/cmd/discoverability_qualification_test.go",
    '''"api", "assembly", "check", "contract", "dependency", "dev", "doc", "doctor", "domain", "explain", "generate", "graph", "init", "inspect", "module",''',
    '''"api", "assembly", "boundary", "check", "contract", "dependency", "dev", "doc", "doctor", "domain", "explain", "generate", "graph", "init", "inspect", "module",''')

replace_once(
    "app/cmd/discoverability_qualification_test.go",
    '''	if strings.Contains(strings.ToLower(firstHelp), "deprecated") {
		t.Fatalf("root help introduced deprecation language:\\n%s", firstHelp)
	}

	expertSubcommands :=''',
    '''	if strings.Contains(strings.ToLower(firstHelp), "deprecated") {
		t.Fatalf("root help introduced deprecation language:\\n%s", firstHelp)
	}

	boundaryHelp, runErr := c116DRunCLI(yunkaBinary, appRoot, "boundary", "--help")
	if runErr != nil {
		t.Fatalf("boundary --help: %v\\n%s", runErr, boundaryHelp)
	}
	if !c116DHelpHasCommand(boundaryHelp, "inspect") {
		t.Fatalf("boundary help does not expose read-only inspect:\\n%s", boundaryHelp)
	}
	for _, forbidden := range []string{"apply", "write", "repair", "merge", "generate"} {
		if c116DHelpHasCommand(boundaryHelp, forbidden) {
			t.Fatalf("boundary help exposes mutation command %q:\\n%s", forbidden, boundaryHelp)
		}
	}

	expertSubcommands :=''')

print("Hardclose 3 public read-only boundary inspect prepared")
