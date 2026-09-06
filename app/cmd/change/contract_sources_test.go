package change

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/projectflow"
)

type sourceScopeFixture struct {
	root, repo string
	paths      map[string]string
	contract   ChangeContract
	set        ChangeSet
}

func newSourceScopeFixture(t *testing.T, inventory, nested bool) sourceScopeFixture {
	t.Helper()
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("real protoc is required")
	}
	repo := t.TempDir()
	root := repo
	if nested {
		root = filepath.Join(repo, "backend-yunka")
		writePressureFile(t, filepath.Join(repo, "outside.proto"), "repository sibling not a project source\n")
	}
	_, here, _, _ := runtime.Caller(0)
	support, err := os.ReadFile(filepath.Join(filepath.Dir(here), "../../../contracts/proto/yunka/dsl/v1/options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	api := "contracts/proto"
	shared := api
	includes := api
	if inventory {
		api = "apis/lifecycle"
		shared = "apis/shared"
		includes = "support"
	}
	paths := map[string]string{"service": api + "/service.proto", "dto": api + "/dto.proto", "other": api + "/other.proto", "common": shared + "/common.proto"}
	writePressureFile(t, filepath.Join(root, includes, "yunka/dsl/v1/options.proto"), string(support))
	writePressureFile(t, filepath.Join(root, "go.mod"), "module example.com/scope\n\ngo 1.25.0\n")
	writePressureFile(t, filepath.Join(root, paths["service"]), scopeServiceProto)
	writePressureFile(t, filepath.Join(root, paths["dto"]), scopeDTOProto)
	writePressureFile(t, filepath.Join(root, paths["other"]), scopeOtherProto)
	writePressureFile(t, filepath.Join(root, paths["common"]), scopeCommonProto)
	if inventory {
		inv := contract.SourceInventory{SchemaVersion: 1, SourceSets: []contract.SourceSet{
			{Name: "api", Root: api, Files: []string{"service.proto", "dto.proto", "other.proto"}, ProtoPaths: []string{shared, includes}},
			{Name: "shared", Root: shared, Files: []string{"common.proto"}},
		}}
		contents, _ := json.Marshal(inv)
		writePressureFile(t, filepath.Join(root, "contracts/sources.json"), string(contents))
	}
	if _, err := add.AddModule(add.ModuleOptions{Root: root, Name: "baseline", Version: "v0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := projectflow.Generate(context.Background(), projectflow.Options{Root: root}); err != nil {
		t.Fatal(err)
	}
	gitPressure(t, repo, "init")
	gitPressure(t, repo, "config", "user.name", "Source Scope Test")
	gitPressure(t, repo, "config", "user.email", "scope@example.invalid")
	gitPressure(t, repo, "add", "-A")
	gitPressure(t, repo, "commit", "-m", "canonical multi-file baseline")
	value, _, err := BuildChangeContract(root, "scope.update", IntentBoth, "HEAD", nil, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteChangeContract(root, "", value); err != nil {
		t.Fatal(err)
	}
	set, _, err := BuildChangeSet(root, "HEAD", []string{DefaultChangeContractPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteChangeSet(root, "", set); err != nil {
		t.Fatal(err)
	}
	return sourceScopeFixture{root: root, repo: repo, paths: paths, contract: value, set: set}
}

func (f sourceScopeFixture) replace(t *testing.T, key, old, next string) {
	t.Helper()
	path := filepath.Join(f.root, f.paths[key])
	data := readPressureFile(t, path)
	if !strings.Contains(data, old) {
		t.Fatalf("missing %q in %s", old, path)
	}
	writePressureFile(t, path, strings.Replace(data, old, next, 1))
}
func (f sourceScopeFixture) check(t *testing.T) ChangeSetCheckReport {
	t.Helper()
	value, err := ReconcileChangeSet(f.root, f.set)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func sourceViolation(t *testing.T, report ChangeSetCheckReport, kind, detail string) {
	t.Helper()
	if report.Conformant || report.ContractSources == nil {
		t.Fatalf("expected source rejection: %#v", report)
	}
	for _, v := range report.ContractSources.Violations {
		if v.Kind == kind && (strings.Contains(v.Path, detail) || strings.Contains(v.Detail, detail)) {
			return
		}
	}
	t.Fatalf("missing %s %s: %#v", kind, detail, report.ContractSources)
}

func TestIssue160SourceScopePlanAndSharedDTO(t *testing.T) {
	for _, inventory := range []bool{false, true} {
		t.Run(map[bool]string{false: "proto-root", true: "inventory"}[inventory], func(t *testing.T) {
			f := newSourceScopeFixture(t, inventory, true)
			want := uniqueSorted([]string{f.paths["service"], f.paths["dto"], f.paths["common"]})
			if !reflect.DeepEqual(f.contract.EditablePaths, want) {
				t.Fatalf("scope=%v want=%v", f.contract.EditablePaths, want)
			}
			contextValue, err := projectflow.DescribeOperationContractContext(context.Background(), projectflow.Options{Root: f.root}, "scope.update")
			if err != nil {
				t.Fatal(err)
			}
			if !contains(contextValue.SourceFiles, f.paths["other"]) || contains(contextValue.DeclarationFiles, f.paths["other"]) {
				t.Fatalf("import/declaration separation lost: %#v", contextValue)
			}
			f.replace(t, "common", "string name = 1;", "string name = 1; string note = 2;")
			if _, err := projectflow.Generate(context.Background(), projectflow.Options{Root: f.root}); err != nil {
				t.Fatal(err)
			}
			report := f.check(t)
			if !report.Conformant || report.ContractSources == nil {
				t.Fatalf("required shared DTO rejected: %#v", report)
			}
			first, _ := json.Marshal(report)
			second, _ := json.Marshal(f.check(t))
			if string(first) != string(second) {
				t.Fatal("non-deterministic source reconciliation")
			}
			single, err := ReconcileGitDelta(f.root, f.contract)
			if err != nil || len(single.Violations) != 0 {
				t.Fatalf("single source reconciliation: %#v %v", single, err)
			}
		})
	}
}

func TestIssue160SourceScopeRejectsUnrelatedAndTamperedPaths(t *testing.T) {
	for _, field := range []string{"normal", "editable", "generated", "current-reference"} {
		t.Run(field, func(t *testing.T) {
			f := newSourceScopeFixture(t, false, false)
			f.replace(t, "other", "string unrelated = 1;", "string unrelated = 1; string extra = 2;")
			switch field {
			case "editable":
				f.set.Subjects[0].Existing.EditablePaths = append(f.set.Subjects[0].Existing.EditablePaths, f.paths["other"])
			case "generated":
				f.set.Subjects[0].Existing.GeneratedPaths = append(f.set.Subjects[0].Existing.GeneratedPaths, f.paths["other"])
			case "current-reference":
				f.replace(t, "dto", `import "common.proto";`, `import "common.proto"; import "other.proto";`)
				f.replace(t, "dto", "Metadata metadata = 1;", "Metadata metadata = 1; OtherRequest unrelated = 2;")
			}
			// Do not regenerate: stale artifacts cannot conceal the actual source mutation.
			report := f.check(t)
			sourceViolation(t, report, "contract-source", f.paths["other"])
			sourceViolation(t, report, "contract-declaration", "scope.v1.OtherRequest")
		})
	}
}

func TestIssue160SourceScopeCoLocatedDeclarationsAndNewReachableTypes(t *testing.T) {
	for _, kind := range []string{"unrelated-message", "unrelated-enum", "reachable-new", "unused-new"} {
		t.Run(kind, func(t *testing.T) {
			f := newSourceScopeFixture(t, false, false)
			switch kind {
			case "unrelated-message":
				f.replace(t, "common", "string noise = 1;", "string noise = 1; string more = 2;")
			case "unrelated-enum":
				f.replace(t, "common", "UNUSED_ZERO = 0;", "UNUSED_ZERO = 0; UNUSED_ONE = 1;")
			case "reachable-new":
				f.replace(t, "dto", "Metadata metadata = 1;", "Metadata metadata = 1; Added added = 2;")
				f.replace(t, "dto", "message UpdateResponse", "message Added { string value = 1; }\nmessage UpdateResponse")
			case "unused-new":
				f.replace(t, "dto", "message UpdateResponse", "message UnusedAdded { string value = 1; }\nmessage UpdateResponse")
			}
			report := f.check(t)
			if kind == "reachable-new" {
				if !report.Conformant {
					t.Fatalf("reachable DTO rejected: %#v", report)
				}
				return
			}
			sourceViolation(t, report, "contract-declaration", map[string]string{"unrelated-message": "NotReferenced", "unrelated-enum": "UnusedState", "unused-new": "UnusedAdded"}[kind])
		})
	}
}

func TestIssue160SourceScopeReadsFreshSemanticAndImmutableBase(t *testing.T) {
	t.Run("relative-external-include", func(t *testing.T) {
		fixture := newPressureFixture(t)
		include, err := filepath.Rel(fixture.Root, fixture.ProtoPath)
		if err != nil {
			t.Fatal(err)
		}
		writePressureFile(t, fixture.protoFile(), readPressureFile(t, fixture.protoFile())+"\n// bounded source edit\n")
		report, err := ReconcileGitDeltaWithOptions(context.Background(), projectflow.Options{Root: fixture.Root, ProtoPaths: []string{include}}, fixture.Contract)
		if err != nil || report.ContractSources == nil || len(report.Violations) != 0 {
			t.Fatalf("explicit external include changed identity in the snapshot: %#v %v", report, err)
		}
	})
	t.Run("stale-generated-security", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		f.replace(t, "service", `permissions: "scope.write"`, `permissions: "scope.admin"`)
		report := f.check(t)
		sourceViolation(t, report, "contract-semantic", "permission")
	})
	t.Run("base-not-current-head", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		f.replace(t, "other", "string unrelated = 1;", "string unrelated = 1; string extra = 2;")
		gitPressure(t, f.repo, "add", "-A")
		gitPressure(t, f.repo, "commit", "-m", "concurrent source change")
		report := f.check(t)
		sourceViolation(t, report, "contract-source", f.paths["other"])
		if report.ContractSources.BaseSHA != f.contract.BaseSHA {
			t.Fatal("base identity changed")
		}
	})
	t.Run("broken-source-no-success", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		f.replace(t, "other", "syntax = \"proto3\";", "not protobuf")
		if _, err := ReconcileChangeSet(f.root, f.set); err == nil {
			t.Fatal("broken raw input accepted")
		}
	})
	t.Run("cancelled-no-success", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		f.replace(t, "common", "string name = 1;", "string name = 1; string note = 2;")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := ReconcileChangeSetWithOptions(ctx, projectflow.Options{Root: f.root}, f.set); err == nil {
			t.Fatal("cancelled query accepted")
		}
	})
}

func TestIssue160SourceScopeMultiSubjectAndCreate(t *testing.T) {
	t.Run("two-existing-subjects", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		second, _, err := BuildChangeContract(f.root, "scope.other", IntentBoth, "HEAD", nil, nil, 3)
		if err != nil {
			t.Fatal(err)
		}
		f.set.Subjects = append(f.set.Subjects, ChangeSetSubject{Kind: ChangeSubjectExistingOperation, Existing: &second})
		f.replace(t, "other", "string unrelated = 1;", "string unrelated = 1; string extra = 2;")
		if report := f.check(t); !report.Conformant {
			t.Fatalf("declared second subject rejected: %#v", report)
		}
	})
	t.Run("create-with-unrelated-edit", func(t *testing.T) {
		f := newSourceScopeFixture(t, false, false)
		servicePath := filepath.Join(f.root, f.paths["service"])
		text := readPressureFile(t, servicePath)
		text = strings.ReplaceAll(text, "option (yunka.dsl.v1.operation) = {", `option (yunka.dsl.v1.operation) = { boundary: { context: "scope.lifecycle" aggregate: "state" }`)
		at := strings.LastIndex(text, "\n}")
		text = text[:at] + `
 rpc CreatePeer(CreateRequest) returns(CreateResponse) {
 option (yunka.dsl.v1.operation) = { id: "scope.create-peer" use_case: "create_peer" permissions: "scope.write" permission_mode: PERMISSION_ALL tenant_required: true authentication: AUTHENTICATION_JWT composition: COMPOSITION_LOCAL execution: {transaction: TRANSACTION_LOCAL idempotency: IDEMPOTENCY_NONE} boundary: {context: "scope.lifecycle" aggregate: "state"} };
 }
` + text[at:] + `
message CreateRequest {option (yunka.dsl.v1.dto) = {kind: DTO_INPUT};}
message CreateResponse {option (yunka.dsl.v1.dto) = {kind: DTO_OUTPUT};}
`
		writePressureFile(t, servicePath, text)
		if _, err := projectflow.Generate(context.Background(), projectflow.Options{Root: f.root}); err != nil {
			t.Fatal(err)
		}
		gitPressure(t, f.repo, "add", "-A")
		gitPressure(t, f.repo, "commit", "-m", "reviewed boundary fixture for source-scope create")
		options := add.OperationOptions{Boundary: &contract.BoundaryIntent{Context: "scope.lifecycle", Aggregate: "state"}, Root: f.root, ApplicationKey: "scope/lifecycle", OperationID: "scope.create", UseCase: "create", Access: "protected", Permissions: []string{"scope.write"}, PermissionMode: "all", Tenant: "required", Authentication: []string{"jwt"}, Transaction: "local", Idempotency: "none", Composition: "local"}
		plan, err := add.PlanOperation(options)
		if err != nil {
			t.Fatal(err)
		}
		contents, err := add.Render(plan, add.FormatAgentJSON)
		if err != nil {
			t.Fatal(err)
		}
		planPath := filepath.Join(f.root, ".git", "create.json")
		writePressureFile(t, planPath, contents)
		set, _, err := BuildChangeSet(f.root, "HEAD", nil, []string{planPath})
		if err != nil {
			t.Fatal(err)
		}
		f.set = set
		if _, err := add.AddOperation(options); err != nil {
			t.Fatal(err)
		}
		if _, err := projectflow.Generate(context.Background(), projectflow.Options{Root: f.root}); err != nil {
			t.Fatal(err)
		}
		if report := f.check(t); !report.Conformant {
			t.Fatalf("matching new operation rejected: %#v", report)
		}
		f.replace(t, "other", "string unrelated = 1;", "string unrelated = 1; string extra = 2;")
		f.set.Subjects[0].Create.EditablePaths = append(f.set.Subjects[0].Create.EditablePaths, f.paths["other"])
		sourceViolation(t, f.check(t), "contract-source", f.paths["other"])
	})
}

func TestIssue160SourceScopePublicCLIAndUnplannedMove(t *testing.T) {
	f := newSourceScopeFixture(t, false, true)
	f.replace(t, "other", "string unrelated = 1;", "string unrelated = 1; string extra = 2;")
	app := cli.NewApp()
	app.Commands = []cli.Command{Command()}
	app.ExitErrHandler = func(*cli.Context, error) {}
	if err := app.Run([]string{"yunka", "change", "set", "check", "--root", f.root, "--format", "agent-json"}); err == nil {
		t.Fatal("public set check accepted unrelated source")
	}
	// A DTO file move cannot silently acquire path authority from the new import.
	gitPressure(t, f.repo, "restore", f.pathsForGit("other"))
	old := filepath.Join(f.root, f.paths["dto"])
	next := filepath.Join(filepath.Dir(old), "moved.proto")
	if err := os.Rename(old, next); err != nil {
		t.Fatal(err)
	}
	f.replace(t, "service", `import "dto.proto";`, `import "moved.proto";`)
	report := f.check(t)
	sourceViolation(t, report, "contract-source", "moved.proto")
}
func (f sourceScopeFixture) pathsForGit(key string) string {
	rel, _ := filepath.Rel(f.repo, filepath.Join(f.root, f.paths[key]))
	return rel
}

const scopeServiceProto = `syntax = "proto3";
package scope.v1;
import "yunka/dsl/v1/options.proto";
import "dto.proto";
import "other.proto";
option go_package = "example.com/scope/contracts/scope;scopev1";
option (yunka.dsl.v1.domain) = { name: "scope" version: "v1" };
service LifecycleService {
 option (yunka.dsl.v1.application) = { name: "lifecycle" };
 rpc Update(UpdateRequest) returns(UpdateResponse) {
  option (yunka.dsl.v1.operation) = { id: "scope.update" use_case: "update" permissions: "scope.write" tenant_required: true authentication: AUTHENTICATION_JWT composition: COMPOSITION_LOCAL execution: {transaction: TRANSACTION_LOCAL idempotency: IDEMPOTENCY_NONE} };
 }
 rpc Other(OtherRequest) returns(OtherResponse) {
  option (yunka.dsl.v1.operation) = { id: "scope.other" use_case: "other" public: true execution: {transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE} };
 }
}
`
const scopeDTOProto = `syntax = "proto3";
package scope.v1;
import "yunka/dsl/v1/options.proto";
import "common.proto";
option go_package = "example.com/scope/contracts/scope;scopev1";
message UpdateRequest { option (yunka.dsl.v1.dto) = { kind: DTO_INPUT }; Metadata metadata = 1; }
message UpdateResponse { option (yunka.dsl.v1.dto) = { kind: DTO_OUTPUT }; Metadata metadata = 1; }
`
const scopeOtherProto = `syntax = "proto3";
package scope.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/scope/contracts/scope;scopev1";
message OtherRequest { option (yunka.dsl.v1.dto) = { kind: DTO_INPUT }; string unrelated = 1; }
message OtherResponse { option (yunka.dsl.v1.dto) = { kind: DTO_OUTPUT }; string result = 1; }
`
const scopeCommonProto = `syntax = "proto3";
package scope.v1;
option go_package = "example.com/scope/contracts/scope;scopev1";
message Metadata { map<string, Label> labels = 1; State state = 2; }
message Label { string name = 1; }
message NotReferenced { string noise = 1; }
enum State { STATE_ZERO = 0; STATE_ONE = 1; }
enum UnusedState { UNUSED_ZERO = 0; }
`
