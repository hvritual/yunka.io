package providerplan

import (
	"strings"
	"testing"

	"yunka.io/pkg/assemblyplan"
)

func TestValidateModulesRequiresExplicitTypedBindings(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Host:          HostCapabilities{Config: true, Logger: true},
		Databases:     []DatabaseBinding{{Name: "primary", Driver: "mysql", DSNEnv: "APP_MYSQL_DSN"}},
		RPC:           []RPCBinding{{Name: "inventory", Driver: "grpc", TargetEnv: "INVENTORY_RPC", Insecure: true}},
	}
	modules := []assemblyplan.ModuleInput{{
		Name: "orders",
		Requirements: assemblyplan.ModuleRequirements{
			ConfigKey: "orders",
			Logger:    true,
			Databases: []string{"primary"},
			RPC:       []string{"inventory"},
		},
	}}
	if err := ValidateModules(manifest, modules); err != nil {
		t.Fatal(err)
	}
	manifest.Databases = nil
	if err := ValidateModules(manifest, modules); err == nil || !strings.Contains(err.Error(), `database provider "primary"`) {
		t.Fatalf("expected missing typed database binding, got %v", err)
	}
}

func TestValidateRejectsImplicitInsecureRPC(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		RPC: []RPCBinding{{Name: "inventory", Driver: "grpc", TargetEnv: "INVENTORY_RPC"}},
	}
	if err := Validate(manifest); err == nil || !strings.Contains(err.Error(), "must configure TLS") {
		t.Fatalf("expected explicit transport requirement, got %v", err)
	}
}

func TestMarshalCanonicalizesBindingOrder(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Databases: []DatabaseBinding{
			{Name: "zeta", Driver: "mysql", DSNEnv: "ZETA_DSN"},
			{Name: "alpha", Driver: "mysql", DSNEnv: "ALPHA_DSN"},
		},
	}
	contents, err := Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(contents), `"alpha"`) > strings.Index(string(contents), `"zeta"`) {
		t.Fatalf("bindings were not canonicalized: %s", contents)
	}
}
