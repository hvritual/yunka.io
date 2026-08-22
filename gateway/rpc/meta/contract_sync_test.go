//go:build contractsync

package meta

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	contractcore "yunka.io/pkg/contract"
)

func TestCommittedGatewayDescriptorsMatchCanonicalProtoSource(t *testing.T) {
	repositoryRoot := os.Getenv("YUNKA_REPOSITORY_ROOT")
	if repositoryRoot == "" {
		t.Fatal("YUNKA_REPOSITORY_ROOT is required")
	}
	protoRoot := filepath.Join(repositoryRoot, "contracts", "proto")
	roots := []string{"gateway/api_common.proto", "gateway/common.proto", "gateway/gateway.proto"}
	compiled, err := contractcore.Compile(context.Background(), contractcore.CompileOptions{
		Dir: protoRoot, Files: roots, Protoc: os.Getenv("PROTOC"),
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := generatedGatewayManifest(t, roots)
	compiled.Manifest.Normalize()
	generated.Normalize()
	if !reflect.DeepEqual(generated, compiled.Manifest) {
		diff := contractcore.Compare(generated, compiled.Manifest)
		t.Fatalf("committed gateway descriptors diverge from contracts/proto: diff=%#v", diff)
	}
}

func generatedGatewayManifest(t *testing.T, roots []string) contractcore.Manifest {
	t.Helper()
	descriptors := map[string]protoreflect.FileDescriptor{
		"gateway/api_common.proto": File_gateway_api_common_proto,
		"gateway/common.proto":     File_gateway_common_proto,
		"gateway/gateway.proto":    File_gateway_gateway_proto,
	}
	set := &descriptorpb.FileDescriptorSet{}
	for _, name := range roots {
		descriptor := descriptors[name]
		if descriptor == nil {
			t.Fatalf("generated descriptor %s is nil", name)
		}
		file := protodesc.ToFileDescriptorProto(descriptor)
		file.SourceCodeInfo = nil
		set.File = append(set.File, file)
	}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := contractcore.ManifestFromDescriptorSet(data, roots)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
