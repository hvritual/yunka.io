package projectflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"yunka.io/pkg/fastfeedback"
)

func TestFastFeedbackInputRootsCoverInventoryAndOrderedProtoPaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "contracts", "sources.json"), `{
  "schemaVersion": 1,
  "sourceSets": [
    {"name":"b","root":"contracts/b","files":["b.proto"],"protoPaths":["third/b0","third/b1"]},
    {"name":"a","root":"contracts/a","files":["a.proto"]}
  ]
}`)
	project := resolvedProject{
		Root:          root,
		InventoryPath: filepath.Join(root, "contracts", "sources.json"),
		ModuleRoot:    filepath.Join(root, "modules"),
		AdditionalProtoPaths: []string{
			filepath.Join(root, "extra", "first"),
			filepath.Join(root, "extra", "second"),
		},
	}
	roots := fastFeedbackInputRoots(project)
	labels := make([]string, len(roots))
	for index, item := range roots {
		labels[index] = item.Label
	}
	want := []string{
		"project.profile",
		"project.goMod",
		"project.providers",
		"module.root",
		"contract.inventory",
		"contract.source.a",
		"contract.source.b",
		"contract.source.b.protoPath.000000000",
		"contract.source.b.protoPath.000000001",
		"contract.additionalProtoPath.000000000",
		"contract.additionalProtoPath.000000001",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels=%v want %v", labels, want)
	}
}

func TestGenerateWithFastFeedbackRecordsCacheAndCheckDoesNotWriteIt(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for C11.4-A evidence integration")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/fastfeedback\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "echo.proto"), `syntax = "proto3";
package echo.v1;
message EchoRequest { string value = 1; }
message EchoResponse { string value = 1; }
service EchoService { rpc Echo(EchoRequest) returns (EchoResponse); }
`)
	options := Options{Root: root, Protoc: protoc}
	if _, err := GenerateWithFastFeedback(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(root, filepath.FromSlash(fastfeedback.CacheRelativePath))
	metadata, err := fastfeedback.Load(cachePath)
	if err != nil {
		t.Fatalf("load generated cache: %v", err)
	}
	if metadata.Inputs.Digest == "" || metadata.Outputs.Digest == "" || metadata.Toolchain == "" {
		t.Fatalf("incomplete cache metadata: %#v", metadata)
	}
	if err := os.Remove(cachePath); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(context.Background(), options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("yunka check wrote cache state: %v", err)
	}
}

func TestFastFeedbackWriteFailureDoesNotChangeGenerateCorrectness(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for C11.4-A evidence integration")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "simple.proto"), `syntax = "proto3";
package simple.v1;
message Item { string id = 1; }
`)
	if err := os.MkdirAll(filepath.Join(root, ".yunka"), 0o750); err != nil {
		t.Fatal(err)
	}
	// Block only the cache directory. The canonical workflow remains readable.
	if err := os.WriteFile(filepath.Join(root, ".yunka", "cache"), []byte("blocked"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateWithFastFeedback(context.Background(), Options{Root: root, Protoc: protoc}); err != nil {
		t.Fatalf("cache failure changed generation correctness: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "contracts", "generated", "manifest.json")); err != nil {
		t.Fatalf("canonical output missing: %v", err)
	}
}
