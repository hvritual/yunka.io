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
	gorm.io/driver/sqlite v1.5.4
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
	writeTestPO(t, root, "repository_runtime_test.go", generatedRepositoryRuntimeTest)
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

const generatedRepositoryRuntimeTest = `package biz_test

import (
    "context"
    "errors"
    "testing"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    device "example.com/biz/internal/device/domain"
    persistence "example.com/biz/internal/device/infrastructure/persistence"
    "example.com/biz/internal/device/ports"
    "yunka.io/framework/core/identity"
)

func tenantContext(tenant string) context.Context {
    return identity.WithPrincipal(context.Background(), identity.Principal{
        Subject: "user-1", TenantID: tenant, UserID: "user-1", Roles: []string{"operator"},
        AuthMethod: identity.AuthMethodJWT, Authenticated: true,
    })
}

func TestGeneratedRepositoryTenantIsolationAndOptimisticConflict(t *testing.T) {
    database, err := gorm.Open(sqlite.Open("file:c84-domain?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := persistence.AutoMigrate(context.Background(), database); err != nil { t.Fatal(err) }
    repository, err := persistence.NewCoffeeMachineRepository(database)
    if err != nil { t.Fatal(err) }

    ctxA := tenantContext("tenant-a")
    ctxB := tenantContext("tenant-b")
    machine := &device.CoffeeMachine{ID: "machine-1", Serial: "SERIAL-A", SiteID: "site-a", Enabled: true}
    if err := repository.Create(ctxA, machine); err != nil { t.Fatal(err) }
    if machine.TenantID != "tenant-a" || machine.Version != 1 { t.Fatalf("create scope/version drift: %#v", machine) }

    loaded, err := repository.Get(ctxA, machine.ID)
    if err != nil { t.Fatal(err) }
    if loaded.Serial != "SERIAL-A" || loaded.TenantID != "tenant-a" { t.Fatalf("unexpected tenant-a record: %#v", loaded) }
    if _, err := repository.Get(ctxB, machine.ID); !errors.Is(err, ports.ErrNotFound) { t.Fatalf("tenant-b visibility err=%v want ErrNotFound", err) }
    if rows, err := repository.List(ctxB, 10, 0); err != nil || len(rows) != 0 { t.Fatalf("tenant-b list rows=%d err=%v", len(rows), err) }

    loaded.Serial = "SERIAL-B"
    if err := repository.Update(ctxA, &loaded, 1); err != nil { t.Fatal(err) }
    loaded.Serial = "SERIAL-C"
    if err := repository.Update(ctxA, &loaded, 1); !errors.Is(err, ports.ErrConflict) { t.Fatalf("stale update err=%v want ErrConflict", err) }

    current, err := repository.Get(ctxA, machine.ID)
    if err != nil { t.Fatal(err) }
    if current.Serial != "SERIAL-B" || current.Version != 2 { t.Fatalf("optimistic update drift: %#v", current) }
}
`
