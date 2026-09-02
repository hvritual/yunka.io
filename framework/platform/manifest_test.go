package platform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hvritual/yunka.io/pkg/providerplan"
)

func TestNewFromManifestBuildsProviderWithoutOpeningResources(t *testing.T) {
	t.Setenv("APP_MYSQL_DSN", "user:pass@tcp(localhost:3306)/app")
	manifest := providerplan.Manifest{
		SchemaVersion: providerplan.SchemaVersion,
		Databases: []providerplan.DatabaseBinding{{Name: "primary", Driver: "mysql", DSNEnv: "APP_MYSQL_DSN"}},
	}
	contents, err := providerplan.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "providers.json")
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	provider, err := NewFromManifest(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Fatal("provider was not constructed")
	}
}
