package contract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestC9GeneratedCrossDomainAcyclicApplicationGraphRuntime(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C9_RUNTIME") != "1" {
		t.Skip("C9 generated runtime fixture is enforced by make dsl-check")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{
			{Name: "access.proto", Package: "access.v1", GoPackage: "example.com/issue181fixture/contracts/access/v1;accessv1", Domain: &DomainDeclaration{Name: "access", Version: "v1"}},
			{Name: "commercial.proto", Package: "commercial.v1", GoPackage: "example.com/issue181fixture/contracts/commercial/v1;commercialv1", Domain: &DomainDeclaration{Name: "commercial", Version: "v1"}},
		},
		Messages: []Message{
			{Name: "CreateTenantRequest", FullName: "access.v1.CreateTenantRequest"},
			{Name: "GetTenantRequest", FullName: "access.v1.GetTenantRequest"},
			{Name: "TenantDTO", FullName: "access.v1.TenantDTO"},
			{Name: "BootstrapSubscriptionRequest", FullName: "commercial.v1.BootstrapSubscriptionRequest"},
			{Name: "SubscriptionDTO", FullName: "commercial.v1.SubscriptionDTO"},
			{Name: "ExplainEntitlementRequest", FullName: "commercial.v1.ExplainEntitlementRequest"},
			{Name: "EntitlementDTO", FullName: "commercial.v1.EntitlementDTO"},
		},
		Services: []Service{
			{
				Name: "TenantLifecycleApplication", FullName: "access.v1.TenantLifecycleApplication", Domain: "access",
				Application: &ApplicationDeclaration{
					Name:     "tenant_lifecycle",
					Requires: []string{"commercial/subscription_management"},
					Operations: []OperationDeclaration{
						{ID: "tenant.get", UseCase: "get_tenant", RequestType: "access.v1.GetTenantRequest", ResponseType: "access.v1.TenantDTO", ApplicationMethod: "GetTenant", Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}},
						{ID: "tenant.create", UseCase: "create_tenant", RequestType: "access.v1.CreateTenantRequest", ResponseType: "access.v1.TenantDTO", ApplicationMethod: "CreateTenant", RequiresOperations: []string{"commercial.subscription.bootstrap"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}},
					},
				},
			},
			{
				Name: "SubscriptionManagementApplication", FullName: "commercial.v1.SubscriptionManagementApplication", Domain: "commercial",
				Application: &ApplicationDeclaration{Name: "subscription_management", Operations: []OperationDeclaration{{ID: "commercial.subscription.bootstrap", UseCase: "bootstrap_subscription", RequestType: "commercial.v1.BootstrapSubscriptionRequest", ResponseType: "commercial.v1.SubscriptionDTO", ApplicationMethod: "BootstrapSubscription", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}},
			},
			{
				Name: "EntitlementManagementApplication", FullName: "commercial.v1.EntitlementManagementApplication", Domain: "commercial",
				Application: &ApplicationDeclaration{Name: "entitlement_management", Requires: []string{"access/tenant_lifecycle"}, Operations: []OperationDeclaration{{ID: "commercial.entitlement.explain", UseCase: "explain_entitlement", RequestType: "commercial.v1.ExplainEntitlementRequest", ResponseType: "commercial.v1.EntitlementDTO", ApplicationMethod: "ExplainEntitlement", RequiresOperations: []string{"tenant.get"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}}}},
			},
		},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/issue181fixture/internal"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}
	accessCapability := byPath["access/application/zz_yunka_tenant_lifecycle_capability_ports_gen.go"]
	commercialCapability := byPath["commercial/application/zz_yunka_entitlement_management_capability_ports_gen.go"]
	if strings.Contains(accessCapability, "internal/commercial/application") || strings.Contains(commercialCapability, "internal/access/application") {
		t.Fatalf("cross-domain capability generation retained whole target application package imports:\naccess=%s\ncommercial=%s", accessCapability, commercialCapability)
	}
	for source, want := range map[string]string{
		accessCapability:     "type TenantLifecycleToCommercialSubscriptionManagementTargetApplication interface",
		commercialCapability: "type EntitlementManagementToAccessTenantLifecycleTargetApplication interface",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated capability missing narrow target interface %q:\n%s", want, source)
		}
	}

	root := t.TempDir()
	writeC84FixtureFile(t, filepath.Join(root, "go.mod"), fmt.Sprintf(`module example.com/issue181fixture

go 1.25.0

require (
	github.com/hvritual/yunka.io/framework v0.0.0
	github.com/hvritual/yunka.io/pkg v0.0.0
)

replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/pkg => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))
	writeC84FixtureFile(t, filepath.Join(root, "contracts", "access", "v1", "types.go"), `package accessv1

type CreateTenantRequest struct{}
type GetTenantRequest struct{}
type TenantDTO struct{}
`)
	writeC84FixtureFile(t, filepath.Join(root, "contracts", "commercial", "v1", "types.go"), `package commercialv1

type BootstrapSubscriptionRequest struct{}
type SubscriptionDTO struct{}
type ExplainEntitlementRequest struct{}
type EntitlementDTO struct{}
`)
	if err := WriteApplicationCode(filepath.Join(root, "internal"), files); err != nil {
		t.Fatal(err)
	}
	writeC84FixtureFile(t, filepath.Join(root, "compile_test.go"), `package issue181fixture

import (
	"context"
	accessv1 "example.com/issue181fixture/contracts/access/v1"
	commercialv1 "example.com/issue181fixture/contracts/commercial/v1"
	accessapplication "example.com/issue181fixture/internal/access/application"
	commercialapplication "example.com/issue181fixture/internal/commercial/application"
)

type tenantLifecycle struct{}
func (*tenantLifecycle) GetTenant(context.Context, *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) { return &accessv1.TenantDTO{}, nil }
func (*tenantLifecycle) CreateTenant(context.Context, *accessv1.CreateTenantRequest) (*accessv1.TenantDTO, error) { return &accessv1.TenantDTO{}, nil }

type subscriptionManagement struct{}
func (*subscriptionManagement) BootstrapSubscription(context.Context, *commercialv1.BootstrapSubscriptionRequest) (*commercialv1.SubscriptionDTO, error) { return &commercialv1.SubscriptionDTO{}, nil }

var _ accessapplication.TenantLifecycleToCommercialSubscriptionManagementTargetApplication = (*subscriptionManagement)(nil)
var _ commercialapplication.EntitlementManagementToAccessTenantLifecycleTargetApplication = (*tenantLifecycle)(nil)
`)
	goTest := exec.Command("go", "test", "-mod=mod", "./...")
	goTest.Dir = root
	goTest.Env = append(os.Environ(), "GOWORK=off")
	if output, err := goTest.CombinedOutput(); err != nil {
		t.Fatalf("generated cross-domain application DAG must compile without Go package cycle: %v\n%s", err, output)
	}
}
