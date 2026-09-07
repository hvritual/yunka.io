package add

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

func operationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	data, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}
func initOperationGit(t *testing.T, root string) {
	t.Helper()
	operationGit(t, root, "init")
	operationGit(t, root, "config", "user.name", "Boundary Test")
	operationGit(t, root, "config", "user.email", "boundary@example.invalid")
	operationGit(t, root, "add", "-A")
	operationGit(t, root, "commit", "-m", "canonical declared boundary fixture")
}
func fixtureProtoPath(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("real protoc is required")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../contracts/proto"))
}

// A reviewed canonical peer is fixture data, not created through a hidden bypass
// in AddOperation. First-Operation initialization is a separate policy.
func seedOperationPeer(t *testing.T, root string, options *OperationOptions) {
	t.Helper()
	options.ProtoPaths = []string{fixtureProtoPath(t)}
	options.Boundary = &contract.BoundaryIntent{Context: "qualification.lifecycle", Aggregate: "tenant"}
	copy := *options
	if err := validateOperationOptions(&copy); err != nil {
		t.Fatal(err)
	}
	req, res := options.RequestType, options.ResponseType
	rpc := options.RPCName
	if rpc == "" {
		rpc = exportedIdentifier(lastKeyPart(options.OperationID))
	}
	if req == "" {
		req = rpc + "Request"
	}
	if res == "" {
		res = rpc + "Response"
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	sources, err := loadSources(inputs)
	if err != nil {
		t.Fatal(err)
	}
	d, a, err := parseApplicationKey(options.ApplicationKey)
	if err != nil {
		t.Fatal(err)
	}
	source, service, err := selectApplicationSource(sources, d, a, options.Source)
	if err != nil {
		t.Fatal(err)
	}
	text := readFile(t, source.Absolute)
	copy.OperationID = "qualification.peer"
	copy.UseCase = "qualified_peer"
	if copy.HTTPPath != "" {
		copy.HTTPPath = "/qualification/peer/{id}"
	}
	peer := renderRPCOperation("QualifiedPeer", req, res, copy)
	text, err = insertServiceMember(text, service.Name, peer)
	if err != nil {
		t.Fatal(err)
	}
	if exists, _ := dtoMessageKind(text, req); !exists {
		text = appendProtoBlock(text, renderDTOMessage(req, "DTO_INPUT", "request fields"))
	}
	if exists, _ := dtoMessageKind(text, res); !exists {
		text = appendProtoBlock(text, renderDTOMessage(res, "DTO_OUTPUT", "response fields"))
	}
	// Existing HTTP mapping tests reference id; compile a real, valid input DTO.
	if options.HTTPPath != "" {
		text = strings.Replace(text, "// TODO(agent): add request fields.", "string id = 1;", 1)
	}
	mustWriteFile(t, source.Absolute, text)
	initOperationGit(t, root)
}

type boundaryFixture struct {
	root, repo, source string
	options            OperationOptions
}

func newBoundaryFixture(t *testing.T, inventory, nested bool) boundaryFixture {
	t.Helper()
	support := fixtureProtoPath(t)
	repo := t.TempDir()
	root := repo
	if nested {
		root = filepath.Join(repo, "backend")
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
	}
	source := "contracts/proto/service.proto"
	if inventory {
		source = "api/service.proto"
	}
	text := typedApplicationProto("sales", "sales.v1", "orders", "OrdersService")
	mustWriteFile(t, filepath.Join(root, source), text)
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0755); err != nil {
		t.Fatal(err)
	}
	opts := OperationOptions{Root: root, ApplicationKey: "sales/orders", OperationID: "sales.read-again", RPCName: "ReadAgain", UseCase: "read_again", RequestType: "ReadRequest", ResponseType: "ReadResponse", Access: "protected", Permissions: []string{"sales.read"}, PermissionMode: "all", Tenant: "required", Authentication: []string{"jwt"}, Transaction: "read-only", Idempotency: "none", Composition: "none", Boundary: &contract.BoundaryIntent{Context: "sales.orders", Aggregate: "order"}}
	peer := opts
	_ = validateOperationOptions(&peer)
	peer.OperationID = "sales.read"
	peer.UseCase = "read_order"
	text, err := insertServiceMember(text, "OrdersService", renderRPCOperation("Read", "ReadRequest", "ReadResponse", peer))
	if err != nil {
		t.Fatal(err)
	}
	text += renderDTOMessage("ReadRequest", "DTO_INPUT", "request fields") + renderDTOMessage("ReadResponse", "DTO_OUTPUT", "response fields")
	mustWriteFile(t, filepath.Join(root, source), text)
	if inventory {
		data, err := os.ReadFile(filepath.Join(support, "yunka/dsl/v1/options.proto"))
		if err != nil {
			t.Fatal(err)
		}
		mustWriteFile(t, filepath.Join(root, "support/yunka/dsl/v1/options.proto"), string(data))
		inv := contract.SourceInventory{SchemaVersion: 1, SourceSets: []contract.SourceSet{{Name: "api", Root: "api", Files: []string{"service.proto"}, ProtoPaths: []string{"support"}}}}
		raw, _ := json.Marshal(inv)
		mustWriteFile(t, filepath.Join(root, "contracts/sources.json"), string(raw))
	} else {
		opts.ProtoPaths = []string{support}
	}
	initOperationGit(t, repo)
	return boundaryFixture{root, repo, source, opts}
}
func noOperationWrites(t *testing.T, f boundaryFixture, before string) {
	t.Helper()
	if got := readFile(t, filepath.Join(f.root, f.source)); got != before {
		t.Fatal("source mutated")
	}
	if _, err := os.Stat(filepath.Join(f.root, "internal/sales/application/sales_read_again.go")); !os.IsNotExist(err) {
		t.Fatalf("unexpected landing: %v", err)
	}
}

func TestIssue161GatePlanApplyRealProtoc(t *testing.T) {
	for _, inventory := range []bool{false, true} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("inventory=%t/nested=%t", inventory, nested), func(t *testing.T) {
				f := newBoundaryFixture(t, inventory, nested)
				original := readFile(t, filepath.Join(f.root, f.source))
				status := operationGit(t, f.repo, "status", "--porcelain")
				first, err := PlanOperation(f.options)
				if err != nil {
					t.Fatal(err)
				}
				second, err := PlanOperation(f.options)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(first, second) || !boundaryAllows(first) || first.SchemaVersion != OperationReportVersion || len(first.Mutations) != 2 {
					t.Fatalf("plan mismatch: %#v", first)
				}
				if first.BaseSHA != operationGit(t, f.repo, "rev-parse", "HEAD") || len(first.InputsDigest) != 64 {
					t.Fatal("missing exact inputs")
				}
				if _, err := RevalidateOperationPlan(f.root, first); err != nil {
					t.Fatal(err)
				}
				noOperationWrites(t, f, original)
				if status != operationGit(t, f.repo, "status", "--porcelain") {
					t.Fatal("plan dirtied Git")
				}
				applied, err := AddOperation(f.options)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(first.BoundaryDecision, applied.BoundaryDecision) {
					t.Fatal("apply changed decision")
				}
				snapshot, err := projectflow.DescribeContractSourceSnapshot(context.Background(), projectflow.Options{Root: f.root, ProtoPaths: f.options.ProtoPaths})
				if err != nil {
					t.Fatal(err)
				}
				inspection, err := boundarycore.Inspect(snapshot.Manifest, "sales/orders")
				if err != nil || len(inspection.Fingerprint.Operations) != 2 {
					t.Fatalf("canonical result: %v %#v", err, inspection)
				}
			})
		}
	}
}
func TestIssue161GateBlocksBeforeWriting(t *testing.T) {
	for _, kind := range []string{"missing-intent", "different-context", "new-dto", "legacy-peer", "empty-application"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundaryFixture(t, false, false)
			switch kind {
			case "missing-intent":
				f.options.Boundary = nil
			case "different-context":
				f.options.Boundary = &contract.BoundaryIntent{Context: "support.tickets", Aggregate: "ticket"}
			case "new-dto":
				f.options.RequestType = "NewInput"
				f.options.ResponseType = "NewOutput"
			case "legacy-peer":
				path := filepath.Join(f.root, f.source)
				text := readFile(t, path)
				lines := strings.Split(text, "\n")
				for i, line := range lines {
					if strings.Contains(line, "boundary: {") {
						lines[i] = ""
					}
				}
				mustWriteFile(t, path, strings.Join(lines, "\n"))
			case "empty-application":
				mustWriteFile(t, filepath.Join(f.root, f.source), typedApplicationProto("sales", "sales.v1", "orders", "OrdersService"))
			}
			before := readFile(t, filepath.Join(f.root, f.source))
			plan, err := PlanOperation(f.options)
			if err != nil {
				t.Fatal(err)
			}
			want := boundarycore.ArchitectureReviewRequired
			if kind == "different-context" {
				want = boundarycore.CreateNewApplication
			}
			if plan.BoundaryDecision == nil || plan.BoundaryDecision.Outcome != want || len(plan.Mutations) != 0 || len(plan.Effects) != 0 {
				t.Fatalf("unsafe blocked plan: %#v", plan)
			}
			if _, err := RevalidateOperationPlan(f.root, plan); err == nil {
				t.Fatal("blocking plan revalidated as writable")
			}
			result, err := AddOperation(f.options)
			if !errors.Is(err, ErrBoundaryBlocked) || result.BoundaryDecision == nil {
				t.Fatalf("apply: %#v %v", result, err)
			}
			noOperationWrites(t, f, before)
		})
	}
}
func TestIssue161GateRevalidationRejectsTamperingAndDrift(t *testing.T) {
	for _, kind := range []string{"missing-proof", "outcome", "counter-evidence", "policy", "inputs", "head", "source-comment", "external-comment", "legacy-schema"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundaryFixture(t, false, false)
			// Isolated external include root, never mutate repository support fixtures.
			support := t.TempDir()
			data, err := os.ReadFile(filepath.Join(f.options.ProtoPaths[0], "yunka/dsl/v1/options.proto"))
			if err != nil {
				t.Fatal(err)
			}
			mustWriteFile(t, filepath.Join(support, "yunka/dsl/v1/options.proto"), string(data))
			f.options.ProtoPaths = []string{support}
			plan, err := PlanOperation(f.options)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "missing-proof":
				plan.BoundaryDecision = nil
			case "outcome":
				plan.BoundaryDecision.Outcome = boundarycore.CreateNewApplication
			case "counter-evidence":
				plan.BoundaryDecision.CounterEvidence = append(plan.BoundaryDecision.CounterEvidence, boundarycore.EvidenceRef{ID: "forged"})
			case "policy":
				plan.BoundaryDecision.PolicyVersion = "unknown"
			case "inputs":
				plan.InputsDigest = strings.Repeat("0", 64)
			case "head":
				operationGit(t, f.repo, "commit", "--allow-empty", "-m", "new baseline")
			case "source-comment":
				mustWriteFile(t, filepath.Join(f.root, f.source), readFile(t, filepath.Join(f.root, f.source))+"\n// concurrent edit\n")
			case "external-comment":
				mustWriteFile(t, filepath.Join(support, "yunka/dsl/v1/options.proto"), string(data)+"\n// changed external input\n")
			case "legacy-schema":
				plan.SchemaVersion = 1
			}
			if _, err := RevalidateOperationPlan(f.root, plan); err == nil {
				t.Fatalf("accepted %s", kind)
			}
		})
	}
}
func TestIssue161GateCLIBlocksAndValidatesBeforeApply(t *testing.T) {
	for _, kind := range []string{"blocked-plan", "blocked-apply", "bad-format", "extra-argument"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundaryFixture(t, false, false)
			before := readFile(t, filepath.Join(f.root, f.source))
			var output bytes.Buffer
			app := cli.NewApp()
			app.Writer = &output
			app.Commands = []cli.Command{Command()}
			app.ExitErrHandler = func(*cli.Context, error) {}
			args := []string{"yunka", "add", "operation", "--root", f.root, "--proto-path", f.options.ProtoPaths[0], "--format", "agent-json", "--use-case", "read_again", "--access", "protected", "--permission", "sales.read", "--permission-mode", "all", "--tenant", "required", "--authentication", "jwt", "--transaction", "read-only", "--idempotency", "none", "--composition", "none", "--request-type", "ReadRequest", "--response-type", "ReadResponse"}
			if kind == "blocked-plan" {
				args = append(args, "--plan")
			}
			if kind == "bad-format" {
				args = append(args, "--format", "invalid")
			}
			args = append(args, "sales/orders", "sales.read-again")
			if kind == "extra-argument" {
				args = append(args, "unexpected")
			}
			if err := app.Run(args); err == nil {
				t.Fatal("CLI accepted blocked/invalid request")
			}
			if strings.HasPrefix(kind, "blocked") {
				var report Report
				if err := json.Unmarshal(output.Bytes(), &report); err != nil {
					t.Fatalf("not structured: %v %s", err, output.String())
				}
				if report.BoundaryDecision == nil || len(report.Mutations) > 0 {
					t.Fatal("missing blocking decision")
				}
			}
			noOperationWrites(t, f, before)
		})
	}
}
func TestIssue161GateWriterLockCancellationAndInvalidSource(t *testing.T) {
	f := newBoundaryFixture(t, false, false)
	before := readFile(t, filepath.Join(f.root, f.source))
	release, err := lockOperationWriter(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddOperation(f.options); err == nil {
		t.Fatal("concurrent writer ignored lock")
	}
	release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	options := f.options
	options.Context = ctx
	if _, err := PlanOperation(options); err == nil {
		t.Fatal("cancelled plan succeeded")
	}
	if _, err := AddOperation(options); err == nil {
		t.Fatal("cancelled apply succeeded")
	}
	noOperationWrites(t, f, before)
	mustWriteFile(t, filepath.Join(f.root, f.source), "broken protobuf source")
	if _, err := AddOperation(f.options); err == nil {
		t.Fatal("broken source accepted")
	}
}

func TestIssue161GateDetectsInputChangeDuringRealCompile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell compiler wrapper uses POSIX")
	}
	real, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("real protoc required")
	}
	for _, kind := range []string{"source", "head"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundaryFixture(t, false, false)
			source := filepath.Join(f.root, f.source)
			before := readFile(t, source)
			marker := filepath.Join(t.TempDir(), "called")
			wrapper := filepath.Join(t.TempDir(), "protoc-wrapper")
			edit := "printf '\\n// concurrent external edit\\n' >> " + shellQuote(source)
			if kind == "head" {
				edit = "git -C " + shellQuote(f.repo) + " commit --allow-empty -m concurrent-head >/dev/null"
			}
			script := "#!/bin/sh\nset -eu\nif [ ! -e " + shellQuote(marker) + " ]; then\n touch " + shellQuote(marker) + "\n" + edit + "\nfi\nexec " + shellQuote(real) + " \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PROTOC", wrapper)
			if _, err := AddOperation(f.options); err == nil {
				t.Fatal("concurrent input accepted")
			}
			want := before
			if kind == "source" {
				want += "\n// concurrent external edit\n"
			}
			noOperationWrites(t, f, want)
		})
	}
}

func TestIssue161GateIncludesProfilesAndLinkedWorktree(t *testing.T) {
	for _, kind := range []string{"duplicate-includes", "relative-includes", "blank-includes", "inventory-override", "source-symlink", "profile", "linked-worktree", "no-aggregate"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundaryFixture(t, kind == "inventory-override", false)
			blocked := false
			switch kind {
			case "duplicate-includes":
				f.options.ProtoPaths = append(f.options.ProtoPaths, f.options.ProtoPaths[0])
			case "relative-includes":
				data, err := os.ReadFile(filepath.Join(f.options.ProtoPaths[0], "yunka/dsl/v1/options.proto"))
				if err != nil {
					t.Fatal(err)
				}
				mustWriteFile(t, filepath.Join(f.root, "support/yunka/dsl/v1/options.proto"), string(data))
				f.options.ProtoPaths = []string{"support"}
			case "blank-includes":
				f.options.ProtoPaths = append(f.options.ProtoPaths, " ")
				blocked = true
			case "inventory-override":
				f.options.ProtoPaths = []string{fixtureProtoPath(t)}
				blocked = true
			case "source-symlink":
				path := filepath.Join(f.root, f.source)
				dest := filepath.Join(t.TempDir(), "source.proto")
				mustWriteFile(t, dest, readFile(t, path))
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dest, path); err != nil {
					t.Fatal(err)
				}
				blocked = true
			case "profile":
				mustWriteFile(t, filepath.Join(f.root, ".yunka/project.json"), `{"version":2,"database":{"tablePrefix":"yk"},"workflow":{"contract":{"protoRoot":"contracts/proto","generated":"build/contracts"},"modules":{"root":"modules"},"generatedGo":{"root":"src/generated"},"dev":{"manifest":".yunka/dev.json"}}}`)
				if err := os.MkdirAll(filepath.Join(f.root, "src/generated"), 0755); err != nil {
					t.Fatal(err)
				}
			case "linked-worktree":
				linked := filepath.Join(t.TempDir(), "worktree")
				operationGit(t, f.repo, "worktree", "add", "--detach", linked, "HEAD")
				f.root = linked
				f.repo = linked
				f.options.Root = linked
			case "no-aggregate":
				text := readFile(t, filepath.Join(f.root, f.source))
				text = strings.ReplaceAll(text, `aggregate: "order" aggregate_not_applicable_reason: ""`, `aggregate_not_applicable_reason: "read-only projection without aggregate ownership"`)
				mustWriteFile(t, filepath.Join(f.root, f.source), text)
				f.options.Boundary = &contract.BoundaryIntent{Context: "sales.orders", AggregateNotApplicableReason: "read-only projection without aggregate ownership"}
			}
			before := readFile(t, filepath.Join(f.root, f.source))
			plan, err := PlanOperation(f.options)
			if blocked {
				if err == nil {
					t.Fatal("invalid compiler inputs accepted")
				}
				if _, err := AddOperation(f.options); err == nil {
					t.Fatal("invalid apply accepted")
				}
				noOperationWrites(t, f, before)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !boundaryAllows(plan) {
				t.Fatalf("not reusable: %+v", plan.BoundaryDecision)
			}
			if _, err := RevalidateOperationPlan(f.root, plan); err != nil {
				t.Fatal(err)
			}
			if _, err := AddOperation(f.options); err != nil {
				t.Fatal(err)
			}
			if _, err := RevalidateOperationPlan(f.root, plan); err == nil {
				t.Fatal("consumed plan was reusable")
			}
		})
	}
}

func TestIssue161GateCLIPassesOnlyDeclaredComparablePeer(t *testing.T) {
	for _, planOnly := range []bool{true, false} {
		t.Run(fmt.Sprint(planOnly), func(t *testing.T) {
			f := newBoundaryFixture(t, false, false)
			var output bytes.Buffer
			app := cli.NewApp()
			app.Writer = &output
			app.Commands = []cli.Command{Command()}
			app.ExitErrHandler = func(*cli.Context, error) {}
			args := []string{"yunka", "add", "operation", "--root", f.root, "--proto-path", f.options.ProtoPaths[0], "--format", "agent-json", "--context", "sales.orders", "--aggregate", "order", "--use-case", "read_again", "--access", "protected", "--permission", "sales.read", "--permission-mode", "all", "--tenant", "required", "--authentication", "jwt", "--transaction", "read-only", "--idempotency", "none", "--composition", "none", "--request-type", "ReadRequest", "--response-type", "ReadResponse"}
			if planOnly {
				args = append(args, "--plan")
			}
			args = append(args, "sales/orders", "sales.read-again")
			if err := app.Run(args); err != nil {
				t.Fatal(err)
			}
			var report Report
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.SchemaVersion != 2 || !boundaryAllows(report) || len(report.Mutations) != 2 {
				t.Fatal("invalid success report")
			}
			if _, err := os.Stat(filepath.Join(f.root, "internal/sales/application/sales_read_again.go")); planOnly != os.IsNotExist(err) {
				t.Fatal("incorrect mutation disposition")
			}
		})
	}
}

func TestIssue161GateMultiSourceSnapshotIsClosedAndFresh(t *testing.T) {
	f := newBoundaryFixture(t, true, false)
	other := filepath.Join(f.root, "other/service.proto")
	mustWriteFile(t, other, typedApplicationProto("warehouse", "warehouse.v1", "inventory", "InventoryService"))
	inv := contract.SourceInventory{SchemaVersion: 1, SourceSets: []contract.SourceSet{
		{Name: "api", Root: "api", Files: []string{"service.proto"}, ProtoPaths: []string{"support"}},
		{Name: "other", Root: "other", Files: []string{"service.proto"}, ProtoPaths: []string{"support"}},
	}}
	raw, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(f.root, "contracts/sources.json"), string(raw))
	plan, err := PlanOperation(f.options)
	if err != nil {
		t.Fatal(err)
	}
	if !boundaryAllows(plan) {
		t.Fatal("independent source set confused target identity")
	}
	before := readFile(t, filepath.Join(f.root, f.source))
	// Even a comment outside the target fingerprint changes the captured input set.
	mustWriteFile(t, other, readFile(t, other)+"\n// concurrent unrelated source change\n")
	if _, err := RevalidateOperationPlan(f.root, plan); !errors.Is(err, boundarycore.ErrStaleBoundaryProof) {
		t.Fatalf("stale inventory input: %v", err)
	}
	noOperationWrites(t, f, before)
	// A fresh decision may be made; old evidence is not silently upgraded.
	if _, err := AddOperation(f.options); err != nil {
		t.Fatal(err)
	}
}
