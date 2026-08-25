package domain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedDomainCompilesAsDownstreamModule(t *testing.T) {
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
	if err := Generate(Options{
		Name:   "device",
		Object: "machine",
		Root:   filepath.Join(root, "internal"),
		Fields: []string{"serial:string", "enabled:bool"},
	}); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated downstream domain does not compile: %v\n%s", err, output)
	}
}
