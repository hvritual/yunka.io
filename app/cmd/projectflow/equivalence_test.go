package projectflow

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	modulecmd "yunka.io/app/cmd/module"
)

const equivalenceModule = "example.com/c11equivalence"

func TestHappyPathMatchesExpertGeneration(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is required for the C11.1 equivalence test")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(repositoryRoot, "app")
	protoInclude := filepath.Join(repositoryRoot, "contracts", "proto")

	happyRoot := t.TempDir()
	expertRoot := t.TempDir()
	prepareEquivalenceFixture(t, happyRoot)
	prepareEquivalenceFixture(t, expertRoot)

	if _, err := Generate(context.Background(), Options{
		Root:       happyRoot,
		Protoc:     protoc,
		ProtoPaths: []string{protoInclude},
	}); err != nil {
		t.Fatal(err)
	}

	runExpertYunka(t, appRoot,
		"contract", "generate",
		"--proto-dir", filepath.Join(expertRoot, "contracts", "proto"),
		"--proto-path", protoInclude,
		"--protoc", protoc,
		"--out", filepath.Join(expertRoot, "contracts", "generated"),
		"--application-out", filepath.Join(expertRoot, "internal"),
		"--application-import", equivalenceModule+"/internal",
	)
	runExpertYunka(t, appRoot,
		"assembly", "generate",
		"--proto-dir", filepath.Join(expertRoot, "contracts", "proto"),
		"--proto-path", protoInclude,
		"--protoc", protoc,
		"--module-root", filepath.Join(expertRoot, "modules"),
		"--out", filepath.Join(expertRoot, "contracts", "generated"),
		"--code-out", filepath.Join(expertRoot, "internal"),
		"--code-import", equivalenceModule+"/internal",
	)

	happySnapshot := snapshotGenerated(t, happyRoot)
	expertSnapshot := snapshotGenerated(t, expertRoot)
	if !reflect.DeepEqual(happySnapshot, expertSnapshot) {
		t.Fatalf("happy-path output differs from expert output:\nhappy=%v\nexpert=%v", happySnapshot, expertSnapshot)
	}

	if _, err := Check(context.Background(), Options{
		Root:       happyRoot,
		Protoc:     protoc,
		ProtoPaths: []string{protoInclude},
	}); err != nil {
		t.Fatalf("happy-path check: %v", err)
	}
	runExpertYunka(t, appRoot,
		"contract", "check",
		"--proto-dir", filepath.Join(expertRoot, "contracts", "proto"),
		"--proto-path", protoInclude,
		"--protoc", protoc,
		"--out", filepath.Join(expertRoot, "contracts", "generated"),
		"--application-out", filepath.Join(expertRoot, "internal"),
		"--application-import", equivalenceModule+"/internal",
	)
	runExpertYunka(t, appRoot,
		"assembly", "check",
		"--proto-dir", filepath.Join(expertRoot, "contracts", "proto"),
		"--proto-path", protoInclude,
		"--protoc", protoc,
		"--module-root", filepath.Join(expertRoot, "modules"),
		"--out", filepath.Join(expertRoot, "contracts", "generated"),
		"--code-out", filepath.Join(expertRoot, "internal"),
		"--code-import", equivalenceModule+"/internal",
	)
}

func prepareEquivalenceFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module "+equivalenceModule+"\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(root, "contracts", "proto", "device", "v1", "device.proto"), `syntax = "proto3";
package device.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/c11equivalence/contracts/device/v1;devicev1";
option (yunka.dsl.v1.domain) = { name: "device" version: "v1" };

message GetDeviceRequest { string id = 1; }
message GetDeviceResponse { string id = 1; }

service DeviceApplication {
  option (yunka.dsl.v1.application) = {
    name: "management"
    operations: {
      id: "device.get"
      use_case: "get_device"
      public: true
      request_type: "device.v1.GetDeviceRequest"
      response_type: "device.v1.GetDeviceResponse"
      application_method: "GetDevice"
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
    }
  };
}
`)
	if err := modulecmd.GenerateWithOptions(modulecmd.Options{
		Name:     "device",
		Root:     filepath.Join(root, "modules"),
		NoConfig: true,
		Logger:   false,
	}); err != nil {
		t.Fatal(err)
	}
}

func runExpertYunka(t *testing.T, appRoot string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"run", "./cmd"}, args...)
	command := exec.Command("go", commandArgs...)
	command.Dir = appRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("expert yunka command failed: go %v: %v\n%s", commandArgs, err, output)
	}
}
