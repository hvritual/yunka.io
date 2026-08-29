package contract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
)

func TestC102GeneratedAssemblyCompilesTypedFactoryBoundary(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C102_RUNTIME") != "1" {
		t.Skip("C10.2 generated compile fixture is enabled by the qualified compiler gate")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manifest := c102CompileFixtureManifest()
	applicationFiles, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/c102fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	filtered := applicationFiles[:0]
	for _, file := range applicationFiles {
		if strings.Contains(filepath.ToSlash(file.Path), "/transport/") {
			continue
		}
		filtered = append(filtered, file)
	}
	if err := WriteApplicationCode(filepath.Join(root, "internal"), filtered); err != nil {
		t.Fatal(err)
	}
	compilation, err := CompileAssembly(manifest, []assemblyplan.ModuleInput{}, AssemblyCodeOptions{RootImport: "example.com/c102fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteAssemblyCompilation(filepath.Join(root, "contracts", "generated"), filepath.Join(root, "internal"), compilation); err != nil {
		t.Fatal(err)
	}

	writeC102File(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/c102fixture

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	yunka.io/framework v0.0.0
	yunka.io/gateway v0.0.0
	yunka.io/pkg v0.0.0
)

replace yunka.io/framework => %s
replace yunka.io/gateway => %s
replace yunka.io/pkg => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))
	writeC102File(t, filepath.Join(root, "contracts", "fixture", "types.go"), `package fixturepb

type TransferRequest struct{}
type TransferResponse struct{}
type ValidateRequest struct{}
type ValidateResponse struct{}
`)
	writeC102File(t, filepath.Join(root, "internal", "device", "transport", "rest", "register.go"), `package rest

import (
	"net/http"
	deviceapplication "example.com/c102fixture/internal/device/application"
	"yunka.io/framework/operation"
)

func RegisterOperationExecutor(*http.ServeMux, deviceapplication.TransferService, operation.Executor) error { return nil }
`)
	writeC102File(t, filepath.Join(root, "internal", "device", "transport", "rpc", "register.go"), `package rpc

import (
	deviceapplication "example.com/c102fixture/internal/device/application"
	"google.golang.org/grpc"
	"yunka.io/framework/operation"
)

func RegisterOperationExecutor(grpc.ServiceRegistrar, deviceapplication.TransferService, operation.Executor) error { return nil }
`)
	writeC102File(t, filepath.Join(root, "factory_test.go"), `package c102fixture

import (
	"context"
	fixturepb "example.com/c102fixture/contracts/fixture"
	assembly "example.com/c102fixture/internal/assembly"
	deviceapplication "example.com/c102fixture/internal/device/application"
	siteapplication "example.com/c102fixture/internal/site/application"
)

type siteService struct{}
func (*siteService) Validate(context.Context, *fixturepb.ValidateRequest) (*fixturepb.ValidateResponse, error) { return &fixturepb.ValidateResponse{}, nil }

type deviceService struct{ target deviceapplication.SiteQueryChildCapability }
func (*deviceService) Transfer(context.Context, *fixturepb.TransferRequest) (*fixturepb.TransferResponse, error) { return &fixturepb.TransferResponse{}, nil }

type factories struct{}
func (factories) BuildSiteQuery(assembly.SiteQueryDependencies) (siteapplication.QueryService, error) { return &siteService{}, nil }
func (factories) BuildDeviceTransfer(dependencies assembly.DeviceTransferDependencies) (deviceapplication.TransferService, error) { return &deviceService{target: dependencies.SiteQuery}, nil }

var _ assembly.ApplicationFactories = factories{}
`)

	positive := exec.Command("go", "test", "-mod=mod", "./...")
	positive.Dir = root
	positive.Env = append(os.Environ(), "GOWORK=off")
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("generated C10.2 assembly fixture failed to compile: %v\n%s", err, output)
	}

	writeC102File(t, filepath.Join(root, "missing_factory_test.go"), `package c102fixture

import assembly "example.com/c102fixture/internal/assembly"

type incompleteFactories struct{}
var _ assembly.ApplicationFactories = incompleteFactories{}
`)
	negative := exec.Command("go", "test", "-mod=mod", "./...")
	negative.Dir = root
	negative.Env = append(os.Environ(), "GOWORK=off")
	output, err := negative.CombinedOutput()
	if err == nil {
		t.Fatalf("missing Application factory unexpectedly compiled:\n%s", output)
	}
	text := string(output)
	if !strings.Contains(text, "ApplicationFactories") || !strings.Contains(text, "BuildDeviceTransfer") {
		t.Fatalf("missing factory failed for an unexpected reason:\n%s", output)
	}
}

func c102CompileFixtureManifest() Manifest {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{{Name: "fixture.proto", Package: "fixture", GoPackage: "example.com/c102fixture/contracts/fixture;fixturepb"}},
		Messages: []Message{
			{Name: "TransferRequest", FullName: "fixture.TransferRequest"},
			{Name: "TransferResponse", FullName: "fixture.TransferResponse"},
			{Name: "ValidateRequest", FullName: "fixture.ValidateRequest"},
			{Name: "ValidateResponse", FullName: "fixture.ValidateResponse"},
		},
		Services: []Service{
			{
				Name: "TransferService", FullName: "fixture.TransferService", Domain: "device",
				Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/query"}},
				Methods: []Method{{
					Name: "Transfer", FullName: "fixture.TransferService.Transfer", Request: "fixture.TransferRequest", Response: "fixture.TransferResponse",
					HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices:transfer", Body: "*"}},
					Operation: &OperationDeclaration{ID: "device.transfer", UseCase: "transfer_device", RequiresOperations: []string{"site.validate"}, Composition: "local"},
				}},
			},
			{
				Name: "QueryService", FullName: "fixture.QueryService", Domain: "site",
				Application: &ApplicationDeclaration{Name: "query", Operations: []OperationDeclaration{{
					ID: "site.validate", UseCase: "validate_transfer_target", RequestType: "fixture.ValidateRequest", ResponseType: "fixture.ValidateResponse", ApplicationMethod: "Validate",
				}}},
			},
		},
	}
	manifest.Normalize()
	return manifest
}

func writeC102File(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
