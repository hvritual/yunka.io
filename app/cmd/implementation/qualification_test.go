package implementation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/urfave/cli"
	"yunka.io/app/cmd/applicationboundary"
	modulecmd "yunka.io/app/cmd/module"
	"yunka.io/app/cmd/ownership"
	projectcmd "yunka.io/app/cmd/project"
	"yunka.io/app/cmd/projectflow"
)

func childGo(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
	command.Dir = root
	command.WaitDelay = time.Second
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GOFLAGS", "GOWORK", "GOTOOLCHAIN", "GOENV", "GOROOT":
		default:
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GOWORK=off", "GOTOOLCHAIN=local", "GOENV=off", "GOFLAGS=")
	out, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Go evidence incomplete: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
}

// This is a typed shape projection, deliberately separate from the real-protoc
// canonical generation test below. It runs generated ports plus editable starter
// files against simple DTO declarations, not a replacement protobuf generator.
func TestAG061ExecutableShapeAndBoundaries(t *testing.T) {
	root := t.TempDir()
	p, m := fixture(root)
	r, err := render(p, m, "shelf/catalog", p.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "go.mod", "module example.com/library\n\ngo 1.25.0\n")
	put(t, root, "wire/model.go", "package shelfv1\ntype Request struct{}\ntype Response struct{}\n")
	generated, err := contract.RenderC9ApplicationCode(m, contract.ApplicationCodeOptions{RootImport: p.GeneratedGoImport})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range generated {
		if strings.HasSuffix(f.Path, "_application_port_gen.go") {
			put(t, root, p.GeneratedGoRoot+"/"+f.Path, string(f.Content))
		}
	}
	for _, f := range r.Files {
		put(t, root, f.Path, f.Content)
	}
	put(t, root, "bootstrap/build.go", `package bootstrap
import (owner "example.com/library/src/app/shelf/application/catalog"; port "example.com/library/src/app/shelf/application")
func BuildCatalog()port.ShelfAPI{return owner.Build()}
`)
	put(t, root, "bootstrap/build_test.go", `package bootstrap
import("context";"strings";"testing")
func TestExplicitUnimplemented(t *testing.T){result,err:=BuildCatalog().ListBooks(context.Background(),nil);if result!=nil||err==nil||!strings.Contains(err.Error(),"not implemented"){t.Fatalf("invented behavior: %v %v",result,err)}}
`)
	if out, err := childGo(t, root, "test", "-mod=readonly", "-count=1", "./..."); err != nil {
		t.Fatalf("starter does not compile or fail explicitly: %v\n%s", err, out)
	}
	policyData, err := os.ReadFile(filepath.Join(root, "src/app/shelf/application/catalog/architecture.types.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := applicationboundary.ReadPolicy(bytes.NewReader(policyData))
	if err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	original := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	if original.Status != applicationboundary.Pass || original.CheckedReferences != 1 {
		t.Fatalf("canonical narrow factory rejected: %+v", original)
	}
	// Actual extra method on the hidden implementation, not a renamed interface.
	file := filepath.Join(root, "src/app/shelf/application/catalog/internal/usecase/wiring.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, append(append([]byte{}, source...), []byte("\nfunc (*service) ExtraAuthority() {}\n")...), 0640); err != nil {
		t.Fatal(err)
	}
	rejected := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	if rejected.Status != applicationboundary.Fail || !hasRule(rejected, "AG-TYPE-003") {
		t.Fatalf("widened implementation escaped: %+v", rejected)
	}
	if err = os.WriteFile(file, source, 0640); err != nil {
		t.Fatal(err)
	}
	// A real Go compiler must reject a sibling importing the nested implementation.
	badPath := "outsider/probe.go"
	put(t, root, badPath, `package outsider
import hidden "example.com/library/src/app/shelf/application/catalog/internal/usecase"
func Probe(){_ = hidden.New()}
`)
	output, err := childGo(t, root, "build", "-mod=readonly", "./outsider")
	if err == nil || !strings.Contains(output, "use of internal package example.com/library/src/app/shelf/application/catalog/internal/usecase not allowed") {
		t.Fatalf("wrong hidden-package rejection: %v\n%s", err, output)
	}
	put(t, root, badPath, `package outsider
import renamed "example.com/library/src/app/shelf/application/catalog"
func Probe(){_ = renamed.Build()}
`)
	rejected = applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	if rejected.Status != applicationboundary.Fail || !hasRule(rejected, "AG-TYPE-001") {
		t.Fatalf("unauthorized factory escaped: %+v", rejected)
	}
	if err = os.Remove(filepath.Join(root, badPath)); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "outsider")); err != nil {
		t.Fatal(err)
	}
	restored := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	a, _ := original.Marshal()
	b, _ := restored.Marshal()
	if !bytes.Equal(a, b) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("restored source or evidence changed")
	}
}
func hasRule(r applicationboundary.Report, rule string) bool {
	for _, f := range r.Findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func compilerProject(t *testing.T) (string, Options) {
	t.Helper()
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Fatal("AG06.1 canonical qualification requires protoc")
	}
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/bookshop\n\ngo 1.25.0\n")
	put(t, root, "contracts/proto/shelf.proto", `syntax = "proto3";
package shelf.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/bookshop/wire;shelfv1";
option (yunka.dsl.v1.domain) = {name:"shelf" version:"v1"};
message Request {string id = 1;}
message Response {string id = 1;}
service ShelfAPI {
 option (yunka.dsl.v1.application) = {
  name:"catalog"
  operations:{id:"shelf.list" use_case:"list_books" public:true
   request_type:"shelf.v1.Request" response_type:"shelf.v1.Response" application_method:"ListBooks"
   execution:{transaction:TRANSACTION_NONE idempotency:IDEMPOTENCY_NONE}}
 };
}
`)
	if err = modulecmd.GenerateWithOptions(modulecmd.Options{Name: "shelf", Root: filepath.Join(root, "modules"), NoConfig: true, Logger: false}); err != nil {
		t.Fatal(err)
	}
	return root, Options{Project: projectflow.Options{Root: root, Protoc: protoc, ProtoPaths: []string{filepath.Join(repo, "contracts/proto")}}, Application: "shelf/catalog", CompositionPackage: "example.com/bookshop/bootstrap"}
}

func TestAG061CanonicalCompileApplyAndRegenerate(t *testing.T) {
	root, options := compilerProject(t)
	ctx := context.Background()
	// Canonical compilation/generation remains the same owner; template output
	// never writes a PB, generated port, policy or Assembly file.
	if _, err := projectflow.Generate(ctx, options.Project); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r, err := Run(ctx, options, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("public plan changed source")
	}
	applied, err := Run(ctx, options, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Mode != "applied" || len(applied.Files) != len(r.Files) {
		t.Fatal("wrong application")
	}
	var targets []string
	for _, f := range applied.Files {
		if f.Action != "created" {
			t.Fatal("file was not created")
		}
		targets = append(targets, f.Path)
	}
	own, err := ownership.Build(root, targets)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range own.Decisions {
		if !d.SafeAutoEdit || d.Owner != "developer-code" {
			t.Fatalf("wrong starter ownership: %+v", d)
		}
	}
	once := snapshot(t, root)
	for i := 0; i < 2; i++ {
		if _, err := projectflow.Generate(ctx, options.Project); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(once, snapshot(t, root)) {
			t.Fatal("canonical regeneration changed developer starter")
		}
	}
	if _, err := projectflow.Check(ctx, options.Project); err != nil {
		t.Fatal(err)
	}
	repeated, err := Run(ctx, options, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range repeated.Files {
		if f.Action != "unchanged" {
			t.Fatal("repeated apply rewrote starter")
		}
	}

	// User edit remains intact and prevents partial creation/rewriting on rerun.
	put(t, root, r.Files[len(r.Files)-1].Path, "developer edit\n")
	edited := snapshot(t, root)
	if _, err := Run(ctx, options, true); err == nil {
		t.Fatal("overwriting user change accepted")
	}
	if !reflect.DeepEqual(edited, snapshot(t, root)) {
		t.Fatal("conflict changed existing source")
	}
}

func TestAG061PublicCommandPlanAndApply(t *testing.T) {
	root, options := compilerProject(t)
	before := snapshot(t, root)
	invoke := func(apply bool) Report {
		t.Helper()
		var out bytes.Buffer
		app := cli.NewApp()
		app.Writer = &out
		app.ErrWriter = &out
		app.Commands = []cli.Command{{Name: "add", Subcommands: []cli.Command{Command()}}}
		args := []string{"yunka", "add", "implementation", "--root", root, "--protoc", options.Project.Protoc, "--proto-path", options.Project.ProtoPaths[0], "--composition-package", options.CompositionPackage, "--format", "agent-json"}
		if apply {
			args = append(args, "--apply")
		}
		args = append(args, options.Application)
		if err := app.Run(args); err != nil {
			t.Fatalf("public CLI: %v\n%s", err, out.String())
		}
		var r Report
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	plan := invoke(false)
	if plan.Mode != "plan" || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("default command was not read-only")
	}
	r := invoke(true)
	if r.Mode != "applied" {
		t.Fatal("explicit apply failed")
	}
	// Legacy project initialization preserves all generated starter content.
	before = snapshot(t, root)
	if _, err := projectcmd.Initialize(root, ""); err != nil {
		t.Fatal(err)
	}
	for name, hash := range before {
		if snapshot(t, root)[name] != hash {
			t.Fatalf("init rewrote %s", name)
		}
	}
}
