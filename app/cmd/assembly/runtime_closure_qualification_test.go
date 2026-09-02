package assembly

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const c103QualificationModule = "example.com/c103qualification"

func TestC103QualificationFullAssembledRuntimeClosure(t *testing.T) {
	if os.Getenv("YUNKA_REQUIRE_C103_QUALIFICATION") != "1" {
		t.Skip("C10.3 assembled runtime fixture is enforced by the repository qualification gate")
	}

	protoc := requireQualificationTool(t, "PROTOC")
	protocGenGo := requireQualificationTool(t, "PROTOC_GEN_GO")
	protocGenGRPC := requireQualificationTool(t, "PROTOC_GEN_GO_GRPC")
	repositoryRoot := qualificationRepositoryRoot(t)
	appRoot := filepath.Join(repositoryRoot, "app")
	fixtureRoot := t.TempDir()
	protoRoot := filepath.Join(fixtureRoot, "contracts", "proto")
	moduleRoot := filepath.Join(fixtureRoot, "modules")
	contractOut := filepath.Join(fixtureRoot, "contracts", "generated")
	codeOut := filepath.Join(fixtureRoot, "internal")
	codeImport := c103QualificationModule + "/internal"

	copyC103FixtureTree(t, filepath.Join("testdata", "c10_3_runtime"), fixtureRoot)
	writeQualificationFile(t, filepath.Join(fixtureRoot, "go.mod"), fmt.Sprintf(`module %s

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	github.com/hvritual/yunka.io/framework v0.0.0
	github.com/hvritual/yunka.io/gateway v0.0.0
	github.com/hvritual/yunka.io/pkg v0.0.0
)

replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/gateway => %s
replace github.com/hvritual/yunka.io/pkg => %s
`, c103QualificationModule,
		filepath.ToSlash(filepath.Join(repositoryRoot, "framework")),
		filepath.ToSlash(filepath.Join(repositoryRoot, "gateway")),
		filepath.ToSlash(filepath.Join(repositoryRoot, "pkg"))))

	generateC103Modules(t, appRoot, moduleRoot)
	generateC103PB(t, protoc, protocGenGo, protocGenGRPC, repositoryRoot, fixtureRoot, protoRoot)

	files := []string{
		"device/v1/device.proto",
		"site/v1/site.proto",
		"inventory/v1/inventory.proto",
	}
	runQualificationYunka(t, appRoot, contractGenerateArgs(protoc, repositoryRoot, protoRoot, contractOut, codeOut, codeImport, files)...)
	runQualificationYunka(t, appRoot, assemblyGenerateArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)
	assertQualificationAssemblyPlan(t, filepath.Join(contractOut, "assembly-plan.json"))
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)

	// The committed consumer is compiled only after the real Yunka generators
	// have supplied its typed Application/transport/assembly packages.
	runQualificationCommand(t, fixtureRoot, qualificationConsumerEnv(), "go", "test", "-timeout=5m", "-count=1", "-mod=mod", "./...")

	// Runtime execution is evidence only and must not mutate generated facts.
	runQualificationYunka(t, appRoot, assemblyCheckArgs(protoc, repositoryRoot, protoRoot, moduleRoot, contractOut, codeOut, codeImport, files)...)
}

func generateC103Modules(t *testing.T, appRoot, moduleRoot string) {
	t.Helper()
	runQualificationYunka(t, appRoot, "module", "new", "--name", "site", "--root", moduleRoot, "--no-config", "--no-logger")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "inventory", "--root", moduleRoot, "--no-config", "--no-logger", "--depends-on", "site")
	runQualificationYunka(t, appRoot, "module", "new", "--name", "device", "--root", moduleRoot, "--no-config", "--no-logger", "--database", "primary", "--depends-on", "site", "--depends-on", "inventory")
}

func generateC103PB(t *testing.T, protoc, protocGenGo, protocGenGRPC, repositoryRoot, fixtureRoot, protoRoot string) {
	t.Helper()
	args := []string{
		"-I", protoRoot,
		"-I", filepath.Join(repositoryRoot, "contracts", "proto"),
	}
	if include := qualificationProtoInclude(protoc); include != "" {
		args = append(args, "-I", include)
	}
	args = append(args,
		"--plugin=protoc-gen-go="+protocGenGo,
		"--plugin=protoc-gen-go-grpc="+protocGenGRPC,
		"--go_out="+fixtureRoot,
		"--go_opt=module="+c103QualificationModule,
		"--go-grpc_out="+fixtureRoot,
		"--go-grpc_opt=module="+c103QualificationModule+",require_unimplemented_servers=false",
		"device/v1/device.proto",
		"site/v1/site.proto",
		"inventory/v1/inventory.proto",
	)
	runQualificationCommand(t, protoRoot, nil, protoc, args...)
}

func copyC103FixtureTree(t *testing.T, sourceRoot, targetRoot string) {
	t.Helper()
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		target := filepath.Join(targetRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
