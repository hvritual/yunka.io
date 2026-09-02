package contract

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

func c102AssemblyFixture(t *testing.T) (Manifest, assemblyplan.Plan) {
	t.Helper()
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Services: []Service{
			{
				Name: "TransferService", FullName: "demo.device.TransferService", Domain: "device",
				Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/query"}},
				Methods: []Method{{
					Name: "Transfer", FullName: "demo.device.TransferService.Transfer", Request: "demo.TransferRequest", Response: "demo.TransferResponse",
					HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices:transfer", Body: "*"}},
					Operation: &OperationDeclaration{
						ID: "device.transfer", UseCase: "transfer_device", RequiresOperations: []string{"site.validate"}, Composition: "local",
						Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "none"},
					},
				}},
			},
			{
				Name: "QueryService", FullName: "demo.site.QueryService", Domain: "site",
				Application: &ApplicationDeclaration{
					Name: "query",
					Operations: []OperationDeclaration{{
						ID: "site.validate", UseCase: "validate_transfer_target", RequestType: "demo.ValidateRequest", ResponseType: "demo.ValidateResponse", ApplicationMethod: "Validate",
					}},
				},
			},
		},
	}
	manifest.Normalize()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	modules := []assemblyplan.ModuleInput{{
		Name: "device", Version: "v1", Requirements: assemblyplan.ModuleRequirements{ConfigKey: "modules.device", Logger: true, Databases: []string{"primary"}},
		Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "modulecatalog", Ref: "modules/device"},
	}}
	plan, err := CompileAssemblyPlan(manifest, operations, modules)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, plan
}

func TestRenderAssemblyCodeBuildsTypedFactoriesCapabilitiesAndCanonicalTransports(t *testing.T) {
	manifest, plan := c102AssemblyFixture(t)
	files, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != AssemblyCodePath {
		t.Fatalf("unexpected generated assembly files: %#v", files)
	}
	source := string(files[0].Content)
	for _, required := range []string{
		"type ApplicationFactories interface",
		"BuildDeviceTransfer(DeviceTransferDependencies) (deviceapplication.TransferService, error)",
		"BuildSiteQuery(SiteQueryDependencies) (siteapplication.QueryService, error)",
		"NewTransferToSiteQueryChildCapability(applications.SiteQuery, executor)",
		"devicerest.RegisterOperationExecutor",
		"devicerpc.RegisterOperationExecutor",
		"func ExpectedModuleRequirements() modulecatalog.RequirementSet",
		"func KernelOptions(dependencies KernelDependencies) kernel.Options",
		"const AssemblyPlanDigest",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("generated assembly missing %q:\n%s", required, source)
		}
	}
	for _, forbidden := range []string{"siterest.", "siterpc.", "reflect.", "map[string]any", "ServiceLocator", "serviceLocator"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("generated assembly contains forbidden or inferred surface %q:\n%s", forbidden, source)
		}
	}
}

func TestRenderAssemblyCodeIsSourceOrderIndependent(t *testing.T) {
	manifest, plan := c102AssemblyFixture(t)
	first, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	manifest.Services[0], manifest.Services[1] = manifest.Services[1], manifest.Services[0]
	plan.Applications[0], plan.Applications[1] = plan.Applications[1], plan.Applications[0]
	plan.Operations[0], plan.Operations[1] = plan.Operations[1], plan.Operations[0]
	second, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first[0].Content, second[0].Content) {
		t.Fatalf("assembly generation depends on source enumeration order:\nfirst=%s\nsecond=%s", first[0].Content, second[0].Content)
	}
}

func TestRenderAssemblyCodeRejectsStaleAssemblyPlanJoin(t *testing.T) {
	manifest, _ := c102AssemblyFixture(t)
	stale := manifest
	stale.Services = append([]Service(nil), manifest.Services...)
	for index := range stale.Services {
		if stale.Services[index].Domain != "device" {
			continue
		}
		application := *stale.Services[index].Application
		application.Requires = nil
		stale.Services[index].Application = &application
		methods := append([]Method(nil), stale.Services[index].Methods...)
		operation := *methods[0].Operation
		operation.RequiresOperations = nil
		methods[0].Operation = &operation
		stale.Services[index].Methods = methods
	}
	stale.Normalize()
	operations, err := CompileOperationPlans(stale)
	if err != nil {
		t.Fatal(err)
	}
	stalePlan, err := CompileAssemblyPlan(stale, operations, []assemblyplan.ModuleInput{{
		Name: "device", Version: "v1", Requirements: assemblyplan.ModuleRequirements{ConfigKey: "modules.device", Logger: true, Databases: []string{"primary"}},
		Evidence: assemblyplan.Evidence{Ownership: assemblyplan.OwnershipReused, Source: "modulecatalog", Ref: "modules/device"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderAssemblyCode(manifest, stalePlan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err == nil || !strings.Contains(err.Error(), "does not match canonical") {
		t.Fatalf("expected stale AssemblyPlan rejection, got %v", err)
	}
}

func TestAssemblyCodeWriteCheckAndSecondGenerationAreZeroDrift(t *testing.T) {
	manifest, plan := c102AssemblyFixture(t)
	files, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := WriteAssemblyCode(root, files); err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckAssemblyCode(root, files); err != nil || len(drift) != 0 {
		t.Fatalf("unexpected initial assembly drift=%v err=%v", drift, err)
	}
	first, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(AssemblyCodePath)))
	if err != nil {
		t.Fatal(err)
	}
	secondFiles, err := RenderAssemblyCode(manifest, plan, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteAssemblyCode(root, secondFiles); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(AssemblyCodePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("second assembly generation drifted:\nfirst=%s\nsecond=%s", first, second)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(AssemblyCodePath)), []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckAssemblyCode(root, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0].File != AssemblyCodePath {
		t.Fatalf("unexpected drift result: %#v", drift)
	}
}
