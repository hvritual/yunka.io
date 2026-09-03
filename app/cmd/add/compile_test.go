package add

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	contractcore "github.com/hvritual/yunka.io/pkg/contract"
)

func TestAX5ScaffoldCompilesThroughCanonicalContractChain(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedDomainProto("tenant", "tenant.v1"),
	})
	if _, err := AddApplication(ApplicationOptions{Root: root, Key: "tenant/lifecycle"}); err != nil {
		t.Fatal(err)
	}
	if _, err := AddOperation(OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend", UseCase: "suspend_tenant",
		Access: "protected", Permissions: []string{"tenant.suspend"}, PermissionMode: "all", Tenant: "required",
		Authentication: []string{"jwt"}, Transaction: "local", Idempotency: "required", Composition: "local",
	}); err != nil {
		t.Fatal(err)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../.."))
	result, err := contractcore.Compile(context.Background(), contractcore.CompileOptions{
		Dir:        filepath.Join(root, "contracts", "proto"),
		ProtoPaths: []string{filepath.Join(repositoryRoot, "contracts", "proto")},
	})
	if err != nil {
		t.Fatalf("canonical compile failed: %v", err)
	}
	if diagnostics := contractcore.Lint(result.Manifest); contractcore.HasErrors(diagnostics) {
		t.Fatalf("canonical lint failed: %#v", diagnostics)
	}
	if _, err := contractcore.RenderArtifacts(result.Manifest, contractcore.ArtifactOptions{
		OpenAPI: contractcore.OpenAPIOptions{Title: "AX5 Qualification", Version: "1.0.0"},
	}); err != nil {
		t.Fatalf("canonical artifact render failed: %v", err)
	}
	files, err := contractcore.RenderC9ApplicationCode(result.Manifest, contractcore.ApplicationCodeOptions{RootImport: "example.com/demo/internal"})
	if err != nil {
		t.Fatalf("canonical application codegen failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("canonical application codegen produced no files")
	}
}
