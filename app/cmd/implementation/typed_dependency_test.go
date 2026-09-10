package implementation

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/applicationboundary"
	modulecmd "yunka.io/app/cmd/module"
	"yunka.io/app/cmd/projectflow"
)

func typedDependencyFixture(root string) (projectflow.ProjectDescriptor, contract.Manifest) {
	project := projectflow.ProjectDescriptor{
		Root:              root,
		GoModule:          "example.com/logistics",
		GeneratedGoRoot:   "internal",
		GeneratedGoImport: "example.com/logistics/internal",
	}
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{
			{Name: "dispatch.proto", Package: "dispatch.v1", GoPackage: "example.com/logistics/wire;wire", Domain: &contract.DomainDeclaration{Name: "dispatch", Version: "v1"}},
			{Name: "inventory.proto", Package: "inventory.v1", GoPackage: "example.com/logistics/wire;wire", Domain: &contract.DomainDeclaration{Name: "inventory", Version: "v1"}},
		},
		Messages: []contract.Message{
			{Name: "PlanRequest", FullName: "dispatch.v1.PlanRequest", SourceFile: "dispatch.proto"},
			{Name: "PlanResponse", FullName: "dispatch.v1.PlanResponse", SourceFile: "dispatch.proto"},
			{Name: "ReserveRequest", FullName: "inventory.v1.ReserveRequest", SourceFile: "inventory.proto"},
			{Name: "ReserveResponse", FullName: "inventory.v1.ReserveResponse", SourceFile: "inventory.proto"},
		},
		Services: []contract.Service{
			{
				Name: "RoutesAPI", FullName: "dispatch.v1.RoutesAPI", Domain: "dispatch",
				Application: &contract.ApplicationDeclaration{Name: "routes", Requires: []string{"inventory/stock"}},
				Methods: []contract.Method{{
					Name: "Plan", FullName: "dispatch.v1.RoutesAPI.Plan", Request: "dispatch.v1.PlanRequest", Response: "dispatch.v1.PlanResponse",
					Operation: &contract.OperationDeclaration{ID: "dispatch.route.plan", UseCase: "plan_route", Public: true, PermissionMode: "all", RequiresOperations: []string{"inventory.stock.reserve"}, Composition: "local", Execution: &contract.ExecutionPolicy{Transaction: "local", Idempotency: "none"}},
				}},
			},
			{
				Name: "StockAPI", FullName: "inventory.v1.StockAPI", Domain: "inventory",
				Application: &contract.ApplicationDeclaration{Name: "stock"},
				Methods: []contract.Method{{
					Name: "Reserve", FullName: "inventory.v1.StockAPI.Reserve", Request: "inventory.v1.ReserveRequest", Response: "inventory.v1.ReserveResponse",
					Operation: &contract.OperationDeclaration{ID: "inventory.stock.reserve", UseCase: "reserve_stock", Public: true, PermissionMode: "all", Execution: &contract.ExecutionPolicy{Transaction: "local", Idempotency: "none"}},
				}},
			},
		},
	}
	return project, manifest
}

func TestAG063TypedDependencyStarterUsesCanonicalChildCapability(t *testing.T) {
	project, manifest := typedDependencyFixture(t.TempDir())
	report, err := render(project, manifest, "dispatch/routes", project.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range report.Files {
		files[filepath.Base(file.Path)] = file.Content
	}
	build := files["build.go"]
	if !strings.Contains(build, "dependency0 _port.RoutesToInventoryStockChildCapability") || !strings.Contains(build, "(_port.RoutesAPI, error)") {
		t.Fatalf("Build is not typed to the canonical source-edge capability:\n%s", build)
	}
	wiring := files["wiring.go"]
	if !strings.Contains(wiring, "dependency0 _starterport.RoutesToInventoryStockChildCapability") || !strings.Contains(wiring, "dependency inventory/stock is required") {
		t.Fatalf("hidden constructor is not fail-closed and typed:\n%s", wiring)
	}
	var handler string
	for _, file := range report.Files {
		if strings.HasSuffix(file.Path, "plan_handler.go") {
			handler = file.Content
		}
	}
	if !strings.Contains(handler, "dependency0 _starterdependency.RoutesToInventoryStockChildCapability") {
		t.Fatalf("use-case handler did not receive its exact child capability:\n%s", handler)
	}

	policy, err := applicationboundary.ReadPolicy(strings.NewReader(files["architecture.types.json"]))
	if err != nil {
		t.Fatal(err)
	}
	factory := policy.Factories[0]
	if len(factory.Arguments) != 1 || factory.Arguments[0].Index != 0 || factory.Arguments[0].Contract.Package != "example.com/logistics/internal/dispatch/application" || factory.Arguments[0].Contract.Name != "RoutesToInventoryStockChildCapability" {
		t.Fatalf("typed dependency is absent from AG-04 policy: %+v", factory.Arguments)
	}
}

func TestAG063TypedDependencyShapeAndBoundaries(t *testing.T) {
	root := t.TempDir()
	project, manifest := typedDependencyFixture(root)
	report, err := render(project, manifest, "dispatch/routes", project.GoModule+"/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "go.mod", "module example.com/logistics\n\ngo 1.25.0\n")
	put(t, root, "wire/model.go", `package wire
 type PlanRequest struct{}
 type PlanResponse struct{}
 type ReserveRequest struct{}
 type ReserveResponse struct{}
`)
	put(t, root, "internal/dispatch/application/contracts.go", `package application
import("context"; wire "example.com/logistics/wire")
type RoutesAPI interface { Plan(context.Context,*wire.PlanRequest)(*wire.PlanResponse,error) }
type RoutesToInventoryStockChildCapability interface { Reserve(context.Context,*wire.ReserveRequest)(*wire.ReserveResponse,error) }
`)
	for _, file := range report.Files {
		put(t, root, file.Path, file.Content)
	}
	put(t, root, "bootstrap/build.go", `package bootstrap
import(
 "context"
 owner "example.com/logistics/internal/dispatch/application/routes"
 wire "example.com/logistics/wire"
)
type stockDependency struct{}
func(stockDependency) Reserve(context.Context,*wire.ReserveRequest)(*wire.ReserveResponse,error){return &wire.ReserveResponse{},nil}
func BuildRoutes() error { _,err:=owner.Build(stockDependency{}); return err }
`)
	if out, err := childGo(t, root, "test", "-mod=readonly", "-count=1", "./..."); err != nil {
		t.Fatalf("typed starter does not compile: %v\n%s", err, out)
	}
	policyData, err := os.ReadFile(filepath.Join(root, "internal/dispatch/application/routes/architecture.types.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := applicationboundary.ReadPolicy(bytes.NewReader(policyData))
	if err != nil {
		t.Fatal(err)
	}
	clean := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	if clean.Status != applicationboundary.Pass {
		t.Fatalf("canonical typed dependency rejected: %+v", clean)
	}

	bootstrap := filepath.Join(root, "bootstrap/build.go")
	original, err := os.ReadFile(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	widened := strings.Replace(string(original), "func BuildRoutes() error", "func(stockDependency) ExtraAuthority() {}\nfunc BuildRoutes() error", 1)
	if err := os.WriteFile(bootstrap, []byte(widened), 0640); err != nil {
		t.Fatal(err)
	}
	rejected := applicationboundary.Check(context.Background(), root, policy, applicationboundary.Options{})
	if rejected.Status != applicationboundary.Fail || !hasRule(rejected, "AG-TYPE-003") {
		t.Fatalf("widened dependency escaped AG-04: %+v", rejected)
	}
	if err := os.WriteFile(bootstrap, original, 0640); err != nil {
		t.Fatal(err)
	}

	put(t, root, "outsider/probe.go", `package outsider
import hidden "example.com/logistics/internal/dispatch/application/routes/internal/usecase"
func Probe(){_ = hidden.New}
`)
	output, buildErr := childGo(t, root, "build", "-mod=readonly", "./outsider")
	if buildErr == nil || !strings.Contains(output, "use of internal package example.com/logistics/internal/dispatch/application/routes/internal/usecase not allowed") {
		t.Fatalf("hidden implementation import was not rejected: %v\n%s", buildErr, output)
	}
}

func TestAG063InvalidDependencyFactsFailClosed(t *testing.T) {
	project, manifest := typedDependencyFixture(t.TempDir())
	manifest.Services[0].Application.Requires = nil
	if _, err := render(project, manifest, "dispatch/routes", project.GoModule+"/bootstrap"); err == nil || !strings.Contains(err.Error(), "undeclared application capability") {
		t.Fatalf("undeclared operation dependency accepted: %v", err)
	}

	_, manifest = typedDependencyFixture(t.TempDir())
	manifest.Services[0].Methods[0].Operation.RequiresOperations = []string{"inventory.stock.missing"}
	if _, err := render(project, manifest, "dispatch/routes", project.GoModule+"/bootstrap"); err == nil || !strings.Contains(err.Error(), "unknown required operation") {
		t.Fatalf("unknown operation dependency accepted: %v", err)
	}
}

func TestAG063LeafStarterContractUnchanged(t *testing.T) {
	root := t.TempDir()
	report := mustPlan(t, root)
	files := map[string]string{}
	for _, file := range report.Files {
		files[filepath.Base(file.Path)] = file.Content
	}
	if !strings.Contains(files["build.go"], "func Build() _port.ShelfAPI") {
		t.Fatalf("leaf Build signature drifted:\n%s", files["build.go"])
	}
	if !strings.Contains(files["wiring.go"], "func New() _starterport.ShelfAPI") || strings.Contains(files["wiring.go"], "dependency0") {
		t.Fatalf("leaf constructor drifted:\n%s", files["wiring.go"])
	}
	policy, err := applicationboundary.ReadPolicy(strings.NewReader(files["architecture.types.json"]))
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Factories[0].Arguments) != 0 {
		t.Fatalf("leaf starter gained dependency arguments: %+v", policy.Factories[0].Arguments)
	}
}

func typedDependencyCompilerProject(t *testing.T) (string, Options) {
	t.Helper()
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Fatal("AG06.3 canonical qualification requires protoc")
	}
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/logistics\n\ngo 1.25.0\n")
	put(t, root, "contracts/proto/inventory.proto", `syntax = "proto3";
package inventory.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/logistics/wire/inventory;inventoryv1";
option (yunka.dsl.v1.domain) = {name:"inventory" version:"v1"};
message ReserveRequest {string id = 1;}
message ReserveResponse {string id = 1;}
service StockAPI {
 option (yunka.dsl.v1.application) = {
  name:"stock"
  operations:{id:"inventory.stock.reserve" use_case:"reserve_stock" public:true
   request_type:"inventory.v1.ReserveRequest" response_type:"inventory.v1.ReserveResponse" application_method:"Reserve"
   execution:{transaction:TRANSACTION_LOCAL idempotency:IDEMPOTENCY_NONE}}
 };
}
`)
	put(t, root, "contracts/proto/dispatch.proto", `syntax = "proto3";
package dispatch.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/logistics/wire/dispatch;dispatchv1";
option (yunka.dsl.v1.domain) = {name:"dispatch" version:"v1"};
message PlanRequest {string id = 1;}
message PlanResponse {string id = 1;}
service RoutesAPI {
 option (yunka.dsl.v1.application) = {
  name:"routes"
  requires:"inventory/stock"
  operations:{id:"dispatch.route.plan" use_case:"plan_route" public:true
   requires_operations:"inventory.stock.reserve" composition:COMPOSITION_LOCAL
   request_type:"dispatch.v1.PlanRequest" response_type:"dispatch.v1.PlanResponse" application_method:"Plan"
   execution:{transaction:TRANSACTION_LOCAL idempotency:IDEMPOTENCY_NONE}}
 };
}
`)
	if err = modulecmd.GenerateWithOptions(modulecmd.Options{Name: "inventory", Root: filepath.Join(root, "modules"), NoConfig: true, Logger: false}); err != nil {
		t.Fatal(err)
	}
	if err = modulecmd.GenerateWithOptions(modulecmd.Options{Name: "dispatch", Root: filepath.Join(root, "modules"), NoConfig: true, Logger: false}); err != nil {
		t.Fatal(err)
	}
	return root, Options{Project: projectflow.Options{Root: root, Protoc: protoc, ProtoPaths: []string{filepath.Join(repo, "contracts/proto")}}, Application: "dispatch/routes", CompositionPackage: "example.com/logistics/bootstrap"}
}

func TestAG063CanonicalProtoApplyAndRegenerate(t *testing.T) {
	root, options := typedDependencyCompilerProject(t)
	ctx := context.Background()
	if _, err := projectflow.Generate(ctx, options.Project); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	planned, err := Run(ctx, options, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("typed dependency plan mutated canonical project")
	}
	applied, err := Run(ctx, options, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Mode != "applied" || len(applied.Files) != len(planned.Files) {
		t.Fatalf("unexpected typed dependency apply report: %+v", applied)
	}
	var build string
	for _, file := range applied.Files {
		if strings.HasSuffix(file.Path, "/build.go") {
			build = file.Content
		}
	}
	if !strings.Contains(build, "RoutesToInventoryStockChildCapability") {
		t.Fatalf("canonical proto did not produce typed child dependency:\n%s", build)
	}
	once := snapshot(t, root)
	for i := 0; i < 2; i++ {
		if _, err := projectflow.Generate(ctx, options.Project); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(once, snapshot(t, root)) {
			t.Fatal("canonical regeneration changed typed developer starter")
		}
	}
	if _, err := projectflow.Check(ctx, options.Project); err != nil {
		t.Fatal(err)
	}
	repeated, err := Run(ctx, options, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range repeated.Files {
		if file.Action != "unchanged" {
			t.Fatalf("repeated typed apply rewrote %s", file.Path)
		}
	}
}
