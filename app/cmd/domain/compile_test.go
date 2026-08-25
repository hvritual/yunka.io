package domain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPOFirstDomainCompilesPinnedGRPCDownstream(t *testing.T) {
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("locked protoc is not available")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	goMod := fmt.Sprintf(`module example.com/biz

go 1.25.0

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0
)

replace yunka.io/framework => %s
replace yunka.io/pkg => %s
replace github.com/go-kit/kit v0.10.0 => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg")), filepath.ToSlash(filepath.Join(repositoryRoot, "compat", "go-kit-kit-log")))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o640); err != nil {
		t.Fatal(err)
	}
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

import "time"

type CoffeeMachinePO struct {
	Serial string `+"`gorm:\"column:serial;type:varchar(64)\"`"+`
	Enabled bool
	LastSeen time.Time `+"`gorm:\"column:last_seen\"`"+`
}
`)
	writeTestPO(t, persistence, "device_group.go", `package persistence

type DeviceGroupPO struct { Name string }
`)
	t.Setenv("YUNKA_DOMAIN_TOOL_DIR", filepath.Join(repositoryRoot, ".yunka", "bin"))
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(root, "internal", "device")
	for _, relative := range []string{"transport/rpc/domain.proto", "transport/rpc/pb/domain.pb.go", "transport/rpc/pb/domain_grpc.pb.go", "transport/rpc/zz_yunka_grpc_bridge_gen.go"} {
		if _, err := os.Stat(filepath.Join(domainRoot, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
	bridge, _ := os.ReadFile(filepath.Join(domainRoot, "transport", "rpc", "zz_yunka_grpc_bridge_gen.go"))
	if !strings.Contains(string(bridge), "pb.RegisterDeviceServiceServer") {
		t.Fatalf("typed grpc registration was not generated: %s", bridge)
	}
	if err := Check(filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "./..."}} {
		command := exec.Command("go", args...)
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, output)
		}
	}
}

func TestDomainToolPinsMatchRepositoryLock(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(repositoryRoot, "tools", "toolchain.env"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{"PROTOC_VERSION=" + domainProtocVersion, "PROTOC_GEN_GO_VERSION=v" + domainProtocGenGoVersion, "PROTOC_GEN_GO_GRPC_VERSION=v" + domainProtocGenGRPCVersion} {
		if !strings.Contains(text, expected) {
			t.Fatalf("domain pin %q not aligned with repository lock", expected)
		}
	}
}
