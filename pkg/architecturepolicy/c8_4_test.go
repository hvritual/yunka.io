package architecturepolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC84DomainCompilerStopsAtRepositoryCRUD(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(contents)
	}

	for _, removed := range []string{
		"app/cmd/domain/templates.go",
		"app/cmd/domain/rpc_codegen.go",
		"app/cmd/domain/transport.go",
		"app/cmd/domain/persistence_v2.go",
		"app/cmd/po",
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(removed))); !os.IsNotExist(err) {
			t.Fatalf("C8.4 removed generator path must stay absent: %s err=%v", removed, err)
		}
	}

	model := read("app/cmd/domain/model.go")
	for _, forbidden := range []string{"ProtoNumber", "ReservedProtoNumbers", "ReservedProtoNames", "NoREST", "NoRPC", "RESTPrefix"} {
		if strings.Contains(model, forbidden) {
			t.Errorf("Domain Manifest V3 must not own protobuf/transport state %q", forbidden)
		}
	}
	if !strings.Contains(model, "SpecVersion  = 3") {
		t.Error("Domain Manifest canonical writer must be version 3")
	}
	for _, required := range []string{"type legacyRESTSpec struct", "type legacyRPCSpec struct", "Canonical V3 manifests never write them"} {
		if !strings.Contains(model, required) {
			t.Errorf("V1/V2 transport compatibility must remain explicitly read-only; missing %q", required)
		}
	}

	generator := read("app/cmd/domain/generator.go")
	if strings.Contains(generator, "exec.Command") || strings.Contains(generator, "os/exec") {
		t.Error("domain generator must not invoke protoc or any external transport generator")
	}
	for _, required := range []string{
		"return renderMultiStructural(spec, packageImport), nil",
		"spec = canonicalizeSpec(spec)",
		"C8.4 cleanup explicitly recognizes historical generated protobuf output",
	} {
		if !strings.Contains(generator, required) {
			t.Errorf("domain generator lost persistence-only canonicalization/legacy cleanup seam %q", required)
		}
	}
	templates := read("app/cmd/domain/multi_templates.go")
	for _, forbidden := range []string{"multiApplicationTemplate", "multiRESTTemplate", "multiProtoTemplate", "multiGRPCBridgeTemplate", "multiWireTemplate", "DefaultService", "transport/rest", "transport/rpc", "wire/", ".proto"} {
		if strings.Contains(templates, forbidden) {
			t.Errorf("domain templates crossed persistence-only boundary with %q", forbidden)
		}
	}
	for _, required := range []string{"multiEntityTemplate", "multiPortsTemplate", "multiRecordTemplate", "multiRepositoriesTemplate"} {
		if !strings.Contains(templates, required) {
			t.Errorf("domain persistence-only generator missing %q", required)
		}
	}

	command := read("app/cmd/domain/command.go")
	for _, forbidden := range []string{"rest-prefix", "no-rest", "no-rpc"} {
		if strings.Contains(command, forbidden) {
			t.Errorf("domain CLI exposes removed transport flag %q", forbidden)
		}
	}
	yunka := read("app/cmd/yunka.go")
	if strings.Contains(yunka, "app/cmd/po") || strings.Contains(yunka, "po.AppName") || strings.Contains(yunka, "po.Main") {
		t.Error("obsolete PO setter command remains registered")
	}
}

func TestC84PBDSLIsCanonicalTypedDeclarationSurface(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	read := func(relative string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		return string(contents)
	}
	options := read("contracts/proto/yunka/dsl/v1/options.proto")
	for _, required := range []string{
		"DomainDeclaration domain = 51001",
		"DTODeclaration dto = 51002",
		"ApplicationDeclaration application = 51003",
		"OperationDeclaration operation = 51004",
		"repeated string permissions = 3",
		"bool tenant_required = 5",
		"bool public = 7",
	} {
		if !strings.Contains(options, required) {
			t.Errorf("typed DSL contract drifted; missing %q", required)
		}
	}
	contractModel := read("pkg/contract/model.go")
	if !strings.Contains(contractModel, "const ManifestVersion = 2") {
		t.Error("C8.4 Contract Manifest canonical schema must be V2")
	}
	codegen := read("pkg/contract/application_codegen.go")
	for _, required := range []string{
		"application_port_gen.go",
		"operation_policy_gen.go",
		"rpc_adapter_gen.go",
		"rest_adapter_gen.go",
		"GeneratedApplicationMarker",
	} {
		if !strings.Contains(codegen, required) {
			t.Errorf("PB-driven application codegen missing %q", required)
		}
	}
	artifact := read("pkg/contract/application_artifact.go")
	if !strings.Contains(artifact, "refusing to delete developer-owned file") || !strings.Contains(artifact, "CheckApplicationCode") {
		t.Error("generated PB application artifacts must be drift-checked and developer-owned files protected")
	}
}

func TestC84CanonicalProtoDoesNotUseCommentPolicyOrStorageDSL(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	protoRoot := filepath.Join(repoRoot, "contracts", "proto")
	forbidden := []string{
		"@yunka.operation",
		"@yunka.permission",
		"@yunka.permission_mode",
		"@yunka.tenant_required",
		"@yunka.authentication",
		"@yunka.http",
		"@yunka.sql",
		"@yunka.table",
		"gorm:",
		"repository_impl",
		"handler_path",
	}
	if err := filepath.WalkDir(protoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(contents)
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				relative, _ := filepath.Rel(repoRoot, path)
				t.Errorf("canonical PB must not use comment-only policy or storage implementation metadata: %s contains %q", filepath.ToSlash(relative), token)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
