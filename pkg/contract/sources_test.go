package contract

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompileInventoryIndependentRootsAndDeterministicOrder(t *testing.T) {
	protoc := testProtoc(t)
	repository := t.TempDir()
	writeProtoFixture(t, filepath.Join(repository, "a", "common.proto"), `syntax = "proto3"; package alpha.v1; message Request { string id = 1; } message Response { string value = 1; }`)
	writeProtoFixture(t, filepath.Join(repository, "a", "service.proto"), `syntax = "proto3"; package alpha.v1; import "common.proto"; service AlphaService { rpc Echo(Request) returns (Response); }`)
	writeProtoFixture(t, filepath.Join(repository, "b", "common.proto"), `syntax = "proto3"; package beta.v1; message Request { int64 id = 1; } message Response { bool ok = 1; }`)
	writeProtoFixture(t, filepath.Join(repository, "b", "service.proto"), `syntax = "proto3"; package beta.v1; import "common.proto"; service BetaService { rpc Echo(Request) returns (Response); }`)

	first := SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{
		{Name: "alpha", Root: "a", Files: []string{"service.proto", "common.proto"}},
		{Name: "beta", Root: "b", Files: []string{"common.proto", "service.proto"}},
	}}
	second := SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{first.SourceSets[1], first.SourceSets[0]}}
	firstPath := writeInventory(t, repository, "first.json", first)
	secondPath := writeInventory(t, repository, "second.json", second)

	one, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: firstPath, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	two, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: secondPath, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one.Manifest, two.Manifest) || one.DescriptorSHA != two.DescriptorSHA || !reflect.DeepEqual(one.Files, two.Files) {
		t.Fatalf("inventory order changed result:\none=%#v\ntwo=%#v", one, two)
	}
	if len(one.Manifest.Services) != 2 || one.Manifest.Services[0].FullName != "alpha.v1.AlphaService" || one.Manifest.Services[1].FullName != "beta.v1.BetaService" {
		t.Fatalf("services=%#v", one.Manifest.Services)
	}
	if got, want := one.Files, []string{"a/common.proto", "a/service.proto", "b/common.proto", "b/service.proto"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files=%v want=%v", got, want)
	}
}

func TestCompileInventoryRejectsUnlistedProto(t *testing.T) {
	repository := t.TempDir()
	writeProtoFixture(t, filepath.Join(repository, "api", "one.proto"), `syntax = "proto3"; message One { string id = 1; }`)
	writeProtoFixture(t, filepath.Join(repository, "api", "two.proto"), `syntax = "proto3"; message Two { string id = 1; }`)
	path := writeInventory(t, repository, "sources.json", SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{{Name: "api", Root: "api", Files: []string{"one.proto"}}}})
	_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: path, Protoc: "does-not-matter"})
	if err == nil || !strings.Contains(err.Error(), "inventory drift") {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileInventoryRejectsDuplicateContractOwnership(t *testing.T) {
	protoc := testProtoc(t)
	repository := t.TempDir()
	writeProtoFixture(t, filepath.Join(repository, "a", "common.proto"), `syntax = "proto3"; package same.v1; message Shared { string id = 1; }`)
	writeProtoFixture(t, filepath.Join(repository, "b", "common.proto"), `syntax = "proto3"; package same.v1; message Shared { string id = 1; }`)
	path := writeInventory(t, repository, "sources.json", SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{
		{Name: "a", Root: "a", Files: []string{"common.proto"}},
		{Name: "b", Root: "b", Files: []string{"common.proto"}},
	}})
	_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: path, Protoc: protoc})
	if err == nil || !strings.Contains(err.Error(), "duplicates message same.v1.Shared") {
		t.Fatalf("err=%v", err)
	}
}

func TestCompileInventoryRejectsPathEscape(t *testing.T) {
	repository := t.TempDir()
	path := writeInventory(t, repository, "sources.json", SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{{Name: "bad", Root: "../outside", Files: []string{"api.proto"}}}})
	_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: path, Protoc: "protoc"})
	if err == nil || !strings.Contains(err.Error(), "escapes repository") {
		t.Fatalf("err=%v", err)
	}
}

func testProtoc(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("PROTOC"); configured != "" {
		return configured
	}
	path, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is not available")
	}
	return path
}

func writeProtoFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeInventory(t *testing.T, repository, name string, inventory SourceInventory) string {
	t.Helper()
	data, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repository, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompileRepositoryContractInventory(t *testing.T) {
	protoc := testProtoc(t)
	repository := filepath.Join("..", "..")
	inventory := filepath.Join(repository, "contracts", "sources.json")
	if _, err := os.Stat(inventory); err != nil {
		t.Skip("canonical contract inventory is not present")
	}
	result, err := CompileInventory(context.Background(), InventoryCompileOptions{
		RepositoryRoot: repository,
		InventoryPath:  inventory,
		Protoc:         protoc,
	})
	if err != nil {
		t.Fatal(err)
	}
	services := make(map[string]struct{}, len(result.Manifest.Services))
	for _, service := range result.Manifest.Services {
		services[service.FullName] = struct{}{}
	}
	for _, name := range []string{"ApiService", "UnitService", "io.yunka.gateway.rpc.GatewayService"} {
		if _, ok := services[name]; !ok {
			t.Fatalf("canonical inventory is missing service %s: %#v", name, result.Manifest.Services)
		}
	}
	artifacts, err := RenderArtifacts(result.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(artifacts.OpenAPI), `"io_yunka_gateway_rpc_RuntimeApi"`) {
		t.Fatal("OpenAPI does not contain the namespaced gateway RuntimeApi schema")
	}
	if !strings.Contains(string(artifacts.TypeScript), "Io_Yunka_Gateway_Rpc_RuntimeApi") {
		t.Fatal("TypeScript does not contain the namespaced gateway RuntimeApi type")
	}
}

func TestCompileInventoryRejectsDuplicateSourceSetNameBeforeCompile(t *testing.T) {
	repository := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repository, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeInventory(t, repository, "sources.json", SourceInventory{SchemaVersion: SourceInventoryVersion, SourceSets: []SourceSet{
		{Name: "same", Root: "a", Files: []string{"one.proto"}},
		{Name: "same", Root: "b", Files: []string{"two.proto"}},
	}})
	_, err := CompileInventory(context.Background(), InventoryCompileOptions{RepositoryRoot: repository, InventoryPath: path, Protoc: "not-used"})
	if err == nil || !strings.Contains(err.Error(), "duplicate source set name") {
		t.Fatalf("err=%v", err)
	}
}
