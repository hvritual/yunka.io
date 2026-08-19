package contract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompileDirectoryAndDirectiveHTTP(t *testing.T) {
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		if path, err := exec.LookPath("protoc"); err == nil {
			protoc = path
		}
	}
	if protoc == "" {
		t.Skip("protoc is not available")
	}
	dir := t.TempDir()
	proto := `syntax = "proto3";
package demo.v1;
option go_package = "example/demo/v1;demov1";
message EchoRequest { string id = 1; repeated string tags = 2; }
message EchoResponse { string value = 1; }
service EchoService {
  // @yunka.http GET /v1/echo/{id}
  // @yunka.auth required
  rpc Echo(EchoRequest) returns (EchoResponse);
}
`
	if err := os.WriteFile(filepath.Join(dir, "echo.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(context.Background(), CompileOptions{Dir: dir, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Services) != 1 || len(result.Manifest.Services[0].Methods) != 1 {
		t.Fatalf("unexpected services: %#v", result.Manifest.Services)
	}
	method := result.Manifest.Services[0].Methods[0]
	if len(method.HTTP) != 1 || method.HTTP[0].Method != "GET" || method.HTTP[0].Path != "/v1/echo/{id}" {
		t.Fatalf("unexpected http binding: %#v", method.HTTP)
	}
	if method.Directives["auth"] != "required" {
		t.Fatalf("unexpected directives: %#v", method.Directives)
	}
	if result.DescriptorSHA == "" {
		t.Fatal("descriptor digest is empty")
	}
	if diagnostics := Lint(result.Manifest); HasErrors(diagnostics) {
		t.Fatalf("lint failed: %#v", diagnostics)
	}
}

func TestCompileExistingLegacyProtoSubset(t *testing.T) {
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		if path, err := exec.LookPath("protoc"); err == nil {
			protoc = path
		}
	}
	if protoc == "" {
		t.Skip("protoc is not available")
	}
	dir := filepath.Join("..", "..", "app", "cmd", "rpc", "pb")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("legacy proto fixture is not present")
	}
	result, err := Compile(context.Background(), CompileOptions{Dir: dir, Protoc: protoc})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Services) != 2 {
		t.Fatalf("services=%d want=2", len(result.Manifest.Services))
	}
	if len(result.Manifest.Messages) == 0 || len(result.Manifest.Enums) == 0 {
		t.Fatalf("unexpected empty schema: messages=%d enums=%d", len(result.Manifest.Messages), len(result.Manifest.Enums))
	}
}
