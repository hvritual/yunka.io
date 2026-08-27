package contract

import (
	"strings"
	"testing"
)

func compositionManifest() Manifest {
	manifest := Manifest{SchemaVersion: ManifestVersion, Files: []File{
		{Name: "customer.proto", Package: "customer.v1", GoPackage: "example.com/biz/contracts/customer/v1;customerv1", Domain: &DomainDeclaration{Name: "customer"}},
		{Name: "deployment.proto", Package: "deployment.v1", GoPackage: "example.com/biz/contracts/deployment/v1;deploymentv1", Domain: &DomainDeclaration{Name: "deployment"}},
	}, Messages: []Message{
		{Name: "GetCustomerRequest", FullName: "customer.v1.GetCustomerRequest", DTO: &DTODeclaration{Kind: "input"}}, {Name: "CustomerDTO", FullName: "customer.v1.CustomerDTO", DTO: &DTODeclaration{Kind: "output"}},
		{Name: "DeployRequest", FullName: "deployment.v1.DeployRequest", DTO: &DTODeclaration{Kind: "input"}}, {Name: "DeployResponse", FullName: "deployment.v1.DeployResponse", DTO: &DTODeclaration{Kind: "output"}},
	}, Services: []Service{
		{Name: "CustomerQueryService", FullName: "customer.v1.CustomerQueryService", Domain: "customer", Application: &ApplicationDeclaration{Name: "customer_query"}, Methods: []Method{{Name: "GetCustomer", FullName: "customer.v1.CustomerQueryService.GetCustomer", Request: "customer.v1.GetCustomerRequest", Response: "customer.v1.CustomerDTO", Operation: &OperationDeclaration{ID: "customer.get", UseCase: "get_customer", Permissions: []string{"customer.read"}, PermissionMode: "all"}}}},
		{Name: "DeploymentService", FullName: "deployment.v1.DeploymentService", Domain: "deployment", Application: &ApplicationDeclaration{Name: "deployment", Requires: []string{"customer/customer_query"}}, Methods: []Method{{Name: "Deploy", FullName: "deployment.v1.DeploymentService.Deploy", Request: "deployment.v1.DeployRequest", Response: "deployment.v1.DeployResponse", Operation: &OperationDeclaration{ID: "deployment.deploy", UseCase: "deploy", Permissions: []string{"customer.read", "deployment.write"}, PermissionMode: "all", RequiresOperations: []string{"customer.get"}, Composition: "local"}}}},
	}}
	for si := range manifest.Services {
		for mi := range manifest.Services[si].Methods {
			method := &manifest.Services[si].Methods[mi]
			method.Authorization = authorizationFromOperation(method.Operation)
		}
	}
	return manifest
}

func TestCompositionLintAndCapabilityPort(t *testing.T) {
	manifest := compositionManifest()
	manifest.Normalize()
	if diagnostics := Lint(manifest); HasErrors(diagnostics) {
		t.Fatalf("lint=%#v", diagnostics)
	}
	files, err := RenderApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil {
		t.Fatal(err)
	}
	var capability string
	for _, file := range files {
		if strings.Contains(file.Path, "deployment_capability_ports") {
			capability = string(file.Content)
		}
	}
	if !strings.Contains(capability, "type DeploymentServiceCapabilities interface") || !strings.Contains(capability, "CustomerCustomerQuery() customerapplication.CustomerQueryService") {
		t.Fatalf("capability port:\n%s", capability)
	}
}

func TestCompositionLintRejectsMissingPermissionClosureAndCycles(t *testing.T) {
	manifest := compositionManifest()
	manifest.Services[1].Methods[0].Operation.Permissions = []string{"deployment.write"}
	if diagnostics := Lint(manifest); !HasErrors(diagnostics) {
		t.Fatal("missing permission closure must fail")
	}
	manifest = compositionManifest()
	manifest.Services[0].Application.Requires = []string{"deployment/deployment"}
	if diagnostics := Lint(manifest); !HasErrors(diagnostics) {
		t.Fatal("application dependency cycle must fail")
	}
}
