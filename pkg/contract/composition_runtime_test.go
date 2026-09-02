package contract

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestC87GeneratedCompositionRuntime(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C87_RUNTIME") != "1" {
		t.Skip("C8.7 fixture is enforced by make dsl-check")
	}
	protoc, goPlugin, grpcPlugin := os.Getenv("PROTOC"), os.Getenv("PROTOC_GEN_GO"), os.Getenv("PROTOC_GEN_GO_GRPC")
	repositoryRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	root := t.TempDir()
	contractsRoot := filepath.Join(root, "contracts")
	writeC84FixtureFile(t, filepath.Join(contractsRoot, "customer", "v1", "customer.proto"), `syntax="proto3";
package customer.v1; import "yunka/dsl/v1/options.proto"; option go_package="example.com/c87fixture/contracts/customer/v1;customerv1"; option (yunka.dsl.v1.domain)={name:"customer"};
message GetCustomerRequest { string id=1; } message CustomerDTO { string id=1; }
service CustomerQueryService { option (yunka.dsl.v1.application)={name:"customer_query"}; rpc GetCustomer(GetCustomerRequest) returns(CustomerDTO) { option (yunka.dsl.v1.operation)={id:"customer.get" use_case:"get_customer" permissions:"customer.read"}; } }
`)
	writeC84FixtureFile(t, filepath.Join(contractsRoot, "deployment", "v1", "deployment.proto"), `syntax="proto3";
package deployment.v1; import "yunka/dsl/v1/options.proto"; option go_package="example.com/c87fixture/contracts/deployment/v1;deploymentv1"; option (yunka.dsl.v1.domain)={name:"deployment"};
message DeployRequest { string customer_id=1; } message DeployResponse { string id=1; }
service DeploymentService { option (yunka.dsl.v1.application)={name:"deployment" requires:"customer/customer_query"}; rpc Deploy(DeployRequest) returns(DeployResponse) { option (yunka.dsl.v1.operation)={id:"deployment.deploy" use_case:"deploy" permissions:"customer.read" permissions:"deployment.write" requires_operations:"customer.get" composition:COMPOSITION_LOCAL}; } }
`)
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/c87fixture

go 1.25.0
require (
 google.golang.org/grpc v1.82.1
 google.golang.org/protobuf v1.36.11
 github.com/hvritual/yunka.io/framework v0.0.0
 github.com/hvritual/yunka.io/gateway v0.0.0
 github.com/hvritual/yunka.io/pkg v0.0.0
)
replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/gateway => %s
replace github.com/hvritual/yunka.io/pkg => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))
	args := []string{"-I", contractsRoot, "-I", filepath.Join(repositoryRoot, "contracts", "proto")}
	if include := standardProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args, "--plugin=protoc-gen-go="+goPlugin, "--plugin=protoc-gen-go-grpc="+grpcPlugin, "--go_out="+root, "--go_opt=module=example.com/c87fixture", "--go-grpc_out="+root, "--go-grpc_opt=module=example.com/c87fixture,require_unimplemented_servers=false", "customer/v1/customer.proto", "deployment/v1/deployment.proto")
	cmd := exec.Command(protoc, args...)
	cmd.Dir = contractsRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}
	compiled, err := Compile(context.Background(), CompileOptions{Dir: contractsRoot, ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")}, Files: []string{"customer/v1/customer.proto", "deployment/v1/deployment.proto"}, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	for si := range compiled.Manifest.Services {
		for mi := range compiled.Manifest.Services[si].Methods {
			m := &compiled.Manifest.Services[si].Methods[mi]
			if m.Operation.ID == "customer.get" {
				m.HTTP = []HTTPBinding{{Method: "GET", Path: "/v1/customers/{id}"}}
			} else {
				m.HTTP = []HTTPBinding{{Method: "POST", Path: "/v1/deployments", Body: "*"}}
			}
		}
	}
	compiled.Manifest.Normalize()
	if diagnostics := Lint(compiled.Manifest); HasErrors(diagnostics) {
		t.Fatalf("lint=%#v", diagnostics)
	}
	files, err := RenderApplicationCode(compiled.Manifest, ApplicationCodeOptions{RootImport: "example.com/c87fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}
	goTest := exec.Command("go", "test", "-mod=mod", "./...")
	goTest.Dir = root
	goTest.Env = append(os.Environ(), "GOWORK=off")
	if out, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("generated composition compile failed: %v\n%s", err, out)
	}
}
