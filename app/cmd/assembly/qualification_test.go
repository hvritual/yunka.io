package assembly

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
)

const c102QualificationModule = "example.com/c102qualification"

func TestC102QualificationGeneratedFixtureClosure(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C102_QUALIFICATION") != "1" {
		t.Skip("C10.2 qualification fixture is enforced by the repository qualification gate")
	}

	protoc := requireQualificationTool(t, "PROTOC")
	protocGenGo := requireQualificationTool(t, "PROTOC_GEN_GO")
	protocGenGRPC := requireQualificationTool(t, "PROTOC_GEN_GO_GRPC")
	repositoryRoot := qualificationRepositoryRoot(t)
	appRoot := filepath.Join(repositoryRoot, "app")
	fixtureRoot := t.TempDir()
	protoRoot := filepath.Join(fixtureRoot, "contracts", "proto")
	moduleRoot := filepath.Join(fixtureRoot, "modules")
	contractOut := filepath.Join(fixtureRoot, "contracts", "generated")
	codeOut := filepath.Join(fixtureRoot, "internal")
	codeImport := c102QualificationModule + "/internal"

	writeQualificationFixture(t, fixtureRoot, repositoryRoot)
	generateQualificationModules(t, appRoot, moduleRoot)
	generateQualificationPB(t, protoc, protocGenGo, protocGenGRPC, repositoryRoot, fixtureRoot, protoRoot)

	firstOrder := []string{
		"device/v1/device.proto",
		"site/v1/site.proto",
		"inventory/v1/inventory.proto",
	}
	secondOrder := []string{
		"inventory/v1/inventory.proto",
		"device/v1/device.proto",
		"site/v1/site.proto",
	}

	// Required C10.2 qualification sequence.
	runQualificationYunka(t, appRoot, contractGenerateArgs(protoc, repositoryRoot, protoRoot, contractOut, codeOut, codeImport, firstOrder)...)
	runQualificationYunka(t, appRoot, assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, firstOrder)...)
	assertQualificationAssemblyPlan(t, filepath.Join(contractOut, "assembly-plan.json"))
	runQualificationCommand(t, fixtureRoot, qualificationConsumerEnv(), "go", "test", "-mod=mod", "./...")
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, firstOrder)...)

	firstSnapshot := snapshotQualificationOutputs(t, fixtureRoot)

	// A second generation deliberately changes source enumeration order. The
	// canonical bytes must remain exactly identical.
	runQualificationYunka(t, appRoot, contractGenerateArgs(protoc, repositoryRoot, protoRoot, contractOut, codeOut, codeImport, secondOrder)...)
	runQualificationYunka(t, appRoot, assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, secondOrder)...)
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, secondOrder)...)
	secondSnapshot := snapshotQualificationOutputs(t, fixtureRoot)
	if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
		t.Fatalf("C10.2 second generation drifted:\n%s", qualificationSnapshotDiff(firstSnapshot, secondSnapshot))
	}
}

func qualificationRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func requireQualificationTool(t *testing.T, name string) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(name))
	if path == "" {
		t.Fatalf("%s is required by C10.2 qualification", name)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("%s=%s is not an executable file: %v", name, path, err)
	}
	return path
}

func writeQualificationFixture(t *testing.T, root, repositoryRoot string) {
	t.Helper()
	writeQualificationFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module %s

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0
	yunka.io/gateway v0.0.0
	yunka.io/pkg v0.0.0
)

replace yunka.io/framework => %s
replace yunka.io/gateway => %s
replace yunka.io/pkg => %s
`, c102QualificationModule,
		filepath.ToSlash(filepath.Join(repositoryRoot, "framework")),
		filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")),
		filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))

	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "site", "v1", "site.proto"), `syntax = "proto3";
package site.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/site/v1;sitev1";
option (yunka.dsl.v1.domain) = { name: "site" version: "v1" };

message ValidateTransferTargetRequest { string site_id = 1; }
message ValidateTransferTargetResponse { bool allowed = 1; }

service SiteApplication {
  option (yunka.dsl.v1.application) = {
    name: "management"
    operations: {
      id: "site.validate_transfer_target"
      use_case: "validate_transfer_target"
      public: true
      request_type: "site.v1.ValidateTransferTargetRequest"
      response_type: "site.v1.ValidateTransferTargetResponse"
      application_method: "ValidateTransferTarget"
      execution: { transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)

	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "inventory", "v1", "inventory.proto"), `syntax = "proto3";
package inventory.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/inventory/v1;inventoryv1";
option (yunka.dsl.v1.domain) = { name: "inventory" version: "v1" };

message ReserveRequest { string sku = 1; }
message ReserveResponse { bool reserved = 1; }

service InventoryApplication {
  option (yunka.dsl.v1.application) = {
    name: "catalog"
    operations: {
      id: "inventory.reserve"
      use_case: "reserve_inventory"
      public: true
      request_type: "inventory.v1.ReserveRequest"
      response_type: "inventory.v1.ReserveResponse"
      application_method: "Reserve"
      execution: { transaction: TRANSACTION_NONE idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)

	writeQualificationFile(t, filepath.Join(root, "contracts", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c102qualification/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };

message GetDeviceRequest { string id = 1; }
message DeviceDTO { string id = 1; string serial = 2; }
message TransferDeviceRequest { string id = 1; string site_id = 2; string sku = 3; }
message TransferDeviceResponse { bool transferred = 1; }

service DeviceQueryService {
  option (yunka.dsl.v1.application) = { name: "query" };
  rpc GetDevice(GetDeviceRequest) returns (DeviceDTO) {
    option (yunka.dsl.v1.operation) = {
      id: "device.get"
      use_case: "get_device"
      public: true
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
    };
  }
}

service DeviceTransferService {
  option (yunka.dsl.v1.application) = {
    name: "transfer"
    requires: "site/management"
    requires: "inventory/catalog"
  };
  rpc TransferDevice(TransferDeviceRequest) returns (TransferDeviceResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "device.transfer"
      use_case: "transfer_device"
      public: true
      requires_operations: "site.validate_transfer_target"
      requires_operations: "inventory.reserve"
      composition: COMPOSITION_LOCAL
      execution: { transaction: TRANSACTION_LOCAL idempotency: IDEMPOTENCY_NONE }
    };
  }
}
`)

	writeQualificationFile(t, filepath.Join(root, "qualification_test.go"), `package qualification

import (
    "context"
    "strings"
    "testing"

    devicev1 "example.com/c102qualification/contracts/device/v1"
    inventoryv1 "example.com/c102qualification/contracts/inventory/v1"
    sitev1 "example.com/c102qualification/contracts/site/v1"
    generatedassembly "example.com/c102qualification/internal/assembly"
    deviceapplication "example.com/c102qualification/internal/device/application"
    inventoryapplication "example.com/c102qualification/internal/inventory/application"
    siteapplication "example.com/c102qualification/internal/site/application"
)

type siteService struct{}
func (*siteService) ValidateTransferTarget(context.Context, *sitev1.ValidateTransferTargetRequest) (*sitev1.ValidateTransferTargetResponse, error) {
    return &sitev1.ValidateTransferTargetResponse{Allowed: true}, nil
}

type inventoryService struct{}
func (*inventoryService) Reserve(context.Context, *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
    return &inventoryv1.ReserveResponse{Reserved: true}, nil
}

type deviceQueryService struct{}
func (*deviceQueryService) GetDevice(_ context.Context, request *devicev1.GetDeviceRequest) (*devicev1.DeviceDTO, error) {
    return &devicev1.DeviceDTO{Id: request.GetId(), Serial: "qualified"}, nil
}

type deviceTransferService struct {
    site deviceapplication.SiteManagementChildCapability
    inventory deviceapplication.InventoryCatalogChildCapability
}
func (*deviceTransferService) TransferDevice(context.Context, *devicev1.TransferDeviceRequest) (*devicev1.TransferDeviceResponse, error) {
    return &devicev1.TransferDeviceResponse{Transferred: true}, nil
}

type factories struct{}
func (factories) BuildSiteManagement(generatedassembly.SiteManagementDependencies) (siteapplication.SiteApplication, error) {
    return &siteService{}, nil
}
func (factories) BuildInventoryCatalog(generatedassembly.InventoryCatalogDependencies) (inventoryapplication.InventoryApplication, error) {
    return &inventoryService{}, nil
}
func (factories) BuildDeviceQuery(generatedassembly.DeviceQueryDependencies) (deviceapplication.QueryApplication, error) {
    return &deviceQueryService{}, nil
}
func (factories) BuildDeviceTransfer(dependencies generatedassembly.DeviceTransferDependencies) (deviceapplication.TransferApplication, error) {
    return &deviceTransferService{site: dependencies.SiteManagement, inventory: dependencies.InventoryCatalog}, nil
}

var _ generatedassembly.ApplicationFactories = factories{}

func TestGeneratedExplicitModuleCatalog(t *testing.T) {
    catalog, err := generatedassembly.NewCatalog()
    if err != nil { t.Fatal(err) }
    plan, err := catalog.Seal()
    if err != nil { t.Fatal(err) }
    if got := strings.Join(plan.Names(), ","); got != "site,inventory,device" {
        t.Fatalf("module startup order=%s", got)
    }
    requirements := generatedassembly.ExpectedModuleRequirements()
    if len(requirements.Modules) != 3 || len(requirements.Databases) != 1 || requirements.Databases[0].Name != "primary" || len(requirements.RPC) != 1 || requirements.RPC[0].Name != "inventory_backend" || !requirements.EventBus {
        t.Fatalf("unexpected module requirements: %#v", requirements)
    }
}
`)
}

func generateQualificationModules(t *testing.T, appRoot, moduleRoot string) {
	t.Helper()
	runQualificationYunka(t, appRoot, "module", "new", "--name", "site", "--root", moduleRoot, "--no-config", "--no-logger")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "inventory", "--root", moduleRoot, "--no-config", "--no-logger", "--rpc", "inventory_backend", "--depends-on", "site")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "device", "--root", moduleRoot, "--no-config", "--no-logger", "--database", "primary", "--event-bus", "--depends-on", "site", "--depends-on", "inventory")
}

func generateQualificationPB(t *testing.T, protoc, protocGenGo, protocGenGRPC, repositoryRoot, fixtureRoot, protoRoot string) {
	t.Helper()
	args := []string{
		"-I", protoRoot,
		"-I", filepath.Join(repositoryRoot, "contracts", "proto"),
	}
	if include := qualificationProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args,
		"--plugin=protoc-gen-go="+protocGenGo,
		"--plugin=protoc-gen-go-grpc="+protocGenGRPC,
		"--go_out="+fixtureRoot,
		"--go_opt=module="+c102QualificationModule,
		"--go-grpc_out="+fixtureRoot,
		"--go-grpc_opt=module="+c102QualificationModule+",require_unimplemented_servers=false",
		"device/v1/device.proto",
		"site/v1/site.proto",
		"inventory/v1/inventory.proto",
	)
	runQualificationCommand(t, protoRoot, nil, protoc, args...)
}

func qualificationProtoInclude(protoc string) string {
	absolute, err := filepath.Abs(protoc)
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(filepath.Dir(absolute)), "include")
	if _, err := os.Stat(filepath.Join(candidate, "google", "protobuf", "descriptor.proto")); err == nil {
		return candidate
	}
	return ""
}

func contractGenerateArgs(protoc, repositoryRoot, protoRoot, contractOut, codeOut, codeImport string, files []string) []string {
	args := []string{
		"contract", "generate",
		"--proto-dir", protoRoot,
		"--proto-path", filepath.Join(repositoryRoot, "contracts", "proto"),
		"--protoc", protoc,
		"--out", contractOut,
		"--application-out", codeOut,
		"--application-import", codeImport,
	}
	return appendQualificationFiles(args, files)
}

func assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport string, files []string) []string {
	args := []string{
		"assembly", "generate",
		"--proto-dir", protoRoot,
		"--proto-path", filepath.Join(repositoryRoot, "contracts", "proto"),
		"--protoc", protoc,
		"--module-root", moduleRoot,
		"--out", contractOut,
		"--code-out", codeOut,
		"--code-import", codeImport,
	}
	return appendQualificationFiles(args, files)
}

func assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport string, files []string) []string {
	args := assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)
	args[1] = "check"
	return args
}

func appendQualificationFiles(args []string, files []string) []string {
	for _, file := range files {
		args = append(args, "--file", file)
	}
	return args
}

func runQualificationYunka(t *testing.T, appRoot string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"run", "./cmd"}, args...)
	return runQualificationCommand(t, appRoot, nil, "go", commandArgs...)
}

func qualificationConsumerEnv() []string {
	return []string{"GOWORK=off"}
}

func runQualificationCommand(t *testing.T, directory string, extraEnv []string, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), extraEnv...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("qualification command failed: %s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func assertQualificationAssemblyPlan(t *testing.T, path string) {
	t.Helper()
	plan, err := assemblyplan.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := assemblyplan.Inspect(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Applications) != 4 || len(summary.Modules) != 3 || len(summary.ExternalOperations) != 2 || len(summary.InternalOperations) != 2 {
		t.Fatalf("unexpected qualification assembly summary: %#v", summary)
	}
	if len(plan.ApplicationDependencies) != 2 || len(plan.OperationDependencies) != 2 || len(plan.ModuleDependencies) != 3 {
		t.Fatalf("unexpected qualification dependency inventory: applications=%#v operations=%#v modules=%#v", plan.ApplicationDependencies, plan.OperationDependencies, plan.ModuleDependencies)
	}
}

func snapshotQualificationOutputs(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, relativeRoot := range []string{"contracts/generated", "internal"} {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(contents)
			result[filepath.ToSlash(relative)] = fmt.Sprintf("%x", digest[:])
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func qualificationSnapshotDiff(first, second map[string]string) string {
	keys := map[string]struct{}{}
	for key := range first {
		keys[key] = struct{}{}
	}
	for key := range second {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	var builder strings.Builder
	for _, key := range ordered {
		if first[key] != second[key] {
			fmt.Fprintf(&builder, "%s: first=%s second=%s\n", key, first[key], second[key])
		}
	}
	return builder.String()
}

func writeQualificationFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
