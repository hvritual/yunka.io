package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/pkg/operationplan"
)

func TestArtifactsAreDeterministicAndDriftAware(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Messages:      []Message{{Name: "EchoRequest", FullName: "demo.EchoRequest", Fields: []Field{{Name: "id", JSONName: "id", Number: 1, Kind: "scalar", Type: "string"}}}, {Name: "EchoResponse", FullName: "demo.EchoResponse"}},
		Services:      []Service{{Name: "EchoService", FullName: "demo.EchoService", Methods: []Method{{Name: "Echo", FullName: "demo.EchoService.Echo", Request: "demo.EchoRequest", Response: "demo.EchoResponse", HTTP: []HTTPBinding{{Method: "GET", Path: "/v1/echo/{id}"}}}}}},
	}
	first, err := RenderArtifacts(manifest, ArtifactOptions{OpenAPI: OpenAPIOptions{Title: "Demo", Version: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderArtifacts(manifest, ArtifactOptions{OpenAPI: OpenAPIOptions{Title: "Demo", Version: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Manifest) != string(second.Manifest) || string(first.OpenAPI) != string(second.OpenAPI) || string(first.TypeScript) != string(second.TypeScript) || string(first.OperationPlans) != string(second.OperationPlans) {
		t.Fatal("artifacts are not deterministic")
	}
	if !strings.Contains(string(first.OpenAPI), `"/v1/echo/{id}"`) {
		t.Fatalf("openapi missing path: %s", first.OpenAPI)
	}
	if !strings.Contains(string(first.TypeScript), "class Demo_EchoServiceClient") {
		t.Fatalf("typescript missing service client: %s", first.TypeScript)
	}
	wantOperationPlans := fmt.Sprintf("{\n  \"schemaVersion\": %d,\n  \"operations\": []\n}\n", operationplan.SchemaVersion)
	if string(first.OperationPlans) != wantOperationPlans {
		t.Fatalf("unexpected operation plans: %s", first.OperationPlans)
	}
	dir := t.TempDir()
	if err := WriteArtifacts(dir, first); err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckArtifacts(dir, first); err != nil || len(drift) != 0 {
		t.Fatalf("unexpected drift=%v err=%v", drift, err)
	}
	if err := os.WriteFile(filepath.Join(dir, TypeScriptFilename), []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckArtifacts(dir, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0].File != TypeScriptFilename {
		t.Fatalf("unexpected drift: %#v", drift)
	}
}
