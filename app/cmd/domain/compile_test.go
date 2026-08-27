package domain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPOFirstDomainCompilesPersistenceOnlyDownstream(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	goMod := fmt.Sprintf(`module example.com/biz

go 1.25.0

require (
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
	SiteID string `+"`gorm:\"column:site_id;type:varchar(64)\"`"+`
	Enabled bool
	LastSeen time.Time `+"`gorm:\"column:last_seen\"`"+`
}
`)
	writeTestPO(t, persistence, "device_group.go", `package persistence

type DeviceGroupPO struct { Name string }
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(root, "internal", "device")
	for _, relative := range []string{
		"domain/zz_yunka_coffee_machine_entity_gen.go",
		"ports/zz_yunka_repositories_gen.go",
		"infrastructure/persistence/zz_yunka_coffee_machine_record_gen.go",
		"infrastructure/persistence/zz_yunka_repositories_gen.go",
	} {
		if _, err := os.Stat(filepath.Join(domainRoot, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
	for _, forbidden := range []string{"application", "transport", "wire"} {
		if _, err := os.Stat(filepath.Join(domainRoot, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("forbidden generated path exists: %s", forbidden)
		}
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

func TestDomainCompilerHasNoProtobufOrTransportGenerationSurface(t *testing.T) {
	command := Command()
	for _, sub := range command.Subcommands {
		for _, flag := range sub.Flags {
			name := flag.GetName()
			if strings.Contains(name, "rpc") || strings.Contains(name, "rest") || strings.Contains(name, "proto") {
				t.Fatalf("domain command exposes transport/protobuf flag %q", name)
			}
		}
	}
}
