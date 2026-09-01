package platform

import (
	"testing"

	"yunka.io/pkg/providerplan"
)

func TestBindManifestBuildsTypedFactoryMapsFromEnvironment(t *testing.T) {
	t.Setenv("APP_MYSQL_DSN", "user:pass@tcp(localhost:3306)/app")
	t.Setenv("INVENTORY_RPC", "127.0.0.1:9000")
	manifest := providerplan.Manifest{
		SchemaVersion: providerplan.SchemaVersion,
		Databases: []providerplan.DatabaseBinding{{Name: "primary", Driver: "mysql", DSNEnv: "APP_MYSQL_DSN"}},
		RPC:       []providerplan.RPCBinding{{Name: "inventory", Driver: "grpc", TargetEnv: "INVENTORY_RPC", Insecure: true}},
	}
	bound, err := BindManifest(manifest, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if bound.Databases["primary"] == nil {
		t.Fatal("primary database factory was not bound")
	}
	if bound.RPC["inventory"] == nil {
		t.Fatal("inventory RPC factory was not bound")
	}
}

func TestBindManifestRequiresDeclaredHostImplementation(t *testing.T) {
	manifest := providerplan.Manifest{SchemaVersion: providerplan.SchemaVersion, Host: providerplan.HostCapabilities{Logger: true}}
	if _, err := BindManifest(manifest, Options{}); err == nil {
		t.Fatal("expected missing host logger failure")
	}
}
