//go:build contractsync

package meta

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	legacyproto "github.com/golang/protobuf/proto"
	descriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	contractcore "yunka.io/pkg/contract"
)

func TestCommittedGatewayDescriptorsMatchCanonicalProtoSource(t *testing.T) {
	repositoryRoot := os.Getenv("YUNKA_REPOSITORY_ROOT")
	if repositoryRoot == "" {
		t.Fatal("YUNKA_REPOSITORY_ROOT is required")
	}
	protoRoot := filepath.Join(repositoryRoot, "gateway", "rpc", "pb")
	compiled, err := contractcore.Compile(context.Background(), contractcore.CompileOptions{
		Dir:    protoRoot,
		Files:  []string{"api_common.proto", "common.proto", "gateway.proto"},
		Protoc: os.Getenv("PROTOC"),
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := generatedGatewayManifest(t)
	compiled.Manifest.Normalize()
	generated.Normalize()
	if !reflect.DeepEqual(generated, compiled.Manifest) {
		diff := contractcore.Compare(generated, compiled.Manifest)
		t.Fatalf("committed gateway generated descriptors diverge from gateway/rpc/pb: diff=%#v\ngenerated=%#v\ncompiled=%#v", diff, generated, compiled.Manifest)
	}
}

func generatedGatewayManifest(t *testing.T) contractcore.Manifest {
	t.Helper()
	roots := []string{"api_common.proto", "common.proto", "gateway.proto"}
	set := &descriptor.FileDescriptorSet{}
	for _, name := range roots {
		compressed := legacyproto.FileDescriptor(name)
		if len(compressed) == 0 {
			t.Fatalf("generated descriptor %s is not registered", name)
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("open generated descriptor %s: %v", name, err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			t.Fatalf("read generated descriptor %s: %v", name, readErr)
		}
		if closeErr != nil {
			t.Fatalf("close generated descriptor %s: %v", name, closeErr)
		}
		file := &descriptor.FileDescriptorProto{}
		if err := legacyproto.Unmarshal(data, file); err != nil {
			t.Fatalf("decode generated descriptor %s: %v", name, err)
		}
		set.File = append(set.File, file)
	}
	data, err := legacyproto.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := contractcore.ManifestFromDescriptorSet(data, roots)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
