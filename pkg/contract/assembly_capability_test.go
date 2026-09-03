package contract

import (
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/assemblyplan"
)

func capabilityAssemblyManifest() Manifest {
	manifest := assemblyManifestFixture()
	for index := range manifest.Services {
		service := &manifest.Services[index]
		if service.Domain == "site" && service.Application != nil && service.Application.Name == "query" {
			service.Application.Capabilities = []CapabilityRequirement{{
				Name: "cache.default", Package: "example.com/contracts/cache", Type: "Cache",
			}}
		}
	}
	manifest.Normalize()
	return manifest
}

func TestCompileAssemblyPlanProjectsTypedApplicationCapability(t *testing.T) {
	manifest := capabilityAssemblyManifest()
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := CompileAssemblyPlan(manifest, operations, []assemblyplan.ModuleInput{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, application := range plan.Applications {
		if application.ID != "site/query" {
			continue
		}
		if len(application.Capabilities) != 1 {
			t.Fatalf("site/query capabilities=%#v", application.Capabilities)
		}
		capability := application.Capabilities[0]
		if capability.Name != "cache.default" || capability.Package != "example.com/contracts/cache" || capability.Type != "Cache" {
			t.Fatalf("unexpected capability=%#v", capability)
		}
		if capability.Evidence.Source != ManifestFilename || !strings.Contains(capability.Evidence.Ref, "applications/site/query/capabilities/cache.default") {
			t.Fatalf("capability evidence=%#v", capability.Evidence)
		}
		found = true
	}
	if !found {
		t.Fatal("site/query Application missing from AssemblyPlan")
	}

	canonical, err := assemblyplan.CanonicalJSON(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(canonical), `"capabilities"`) || !strings.Contains(string(canonical), `"cache.default"`) {
		t.Fatalf("AssemblyPlan artifact omitted capability requirement:\n%s", canonical)
	}
}

func TestCompileAssemblyPlanRejectsInvalidApplicationCapabilityContract(t *testing.T) {
	manifest := capabilityAssemblyManifest()
	for index := range manifest.Services {
		service := &manifest.Services[index]
		if service.Domain == "site" && service.Application != nil && service.Application.Name == "query" {
			service.Application.Capabilities[0].Package = "bad package"
		}
	}
	operations, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CompileAssemblyPlan(manifest, operations, []assemblyplan.ModuleInput{})
	if err == nil || !strings.Contains(err.Error(), "invalid Go package") {
		t.Fatalf("invalid capability contract error=%v", err)
	}
}

func TestCompileAssemblyGeneratesConcreteTypedCapabilityDependency(t *testing.T) {
	manifest := capabilityAssemblyManifest()
	compilation, err := CompileAssembly(manifest, []assemblyplan.ModuleInput{}, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err != nil {
		t.Fatal(err)
	}
	source := string(compilation.GoFiles[0].Content)
	for _, required := range []string{
		`cache "example.com/contracts/cache"`,
		"CacheDefault cache.Cache",
		"func BuildApplicationsWithCapabilities(factories ApplicationFactories, executor operation.Executor, capabilities modulecatalog.CapabilitySet)",
		`modulecatalog.MustCapabilityKey[cache.Cache]("cache.default", "example.com/contracts/cache", "Cache")`,
		"CacheDefault: siteQueryCacheDefaultCapability",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("generated Assembly missing typed capability fragment %q:\n%s", required, source)
		}
	}
	for _, forbidden := range []string{"map[string]any", "Get(\"cache.default\")", "ServiceLocator"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("generated Assembly contains untyped/runtime-locator surface %q:\n%s", forbidden, source)
		}
	}
}

func TestCompileAssemblyRejectsCapabilityFieldCollisionWithApplicationDependency(t *testing.T) {
	manifest := assemblyManifestFixture()
	for index := range manifest.Services {
		service := &manifest.Services[index]
		if service.Domain == "device" && service.Application != nil && service.Application.Name == "transfer" {
			service.Application.Capabilities = []CapabilityRequirement{{
				Name: "site.query", Package: "example.com/contracts/cache", Type: "Cache",
			}}
		}
	}
	manifest.Normalize()
	_, err := CompileAssembly(manifest, []assemblyplan.ModuleInput{}, AssemblyCodeOptions{RootImport: "example.com/product/internal"})
	if err == nil || !strings.Contains(err.Error(), "collides with application dependency") {
		t.Fatalf("capability field collision error=%v", err)
	}
}
