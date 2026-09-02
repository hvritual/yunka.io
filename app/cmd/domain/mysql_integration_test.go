//go:build integration

package domain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedRepositoryMySQLTenantIsolationAndOptimisticConflict(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	goMod := fmt.Sprintf(`module example.com/c84mysql

go 1.25.0

require (
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
	github.com/hvritual/yunka.io/framework v0.0.0
)

replace github.com/hvritual/yunka.io/framework => %s
replace github.com/hvritual/yunka.io/pkg => %s
replace github.com/go-kit/kit v0.10.0 => %s
`, filepath.ToSlash(filepath.Join(repositoryRoot, "framework")), filepath.ToSlash(filepath.Join(repositoryRoot, "pkg")), filepath.ToSlash(filepath.Join(repositoryRoot, "compat", "go-kit-kit-log")))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o640); err != nil {
		t.Fatal(err)
	}
	persistenceRoot := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistenceRoot, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct {
	Serial string `+"`gorm:\"column:serial;type:varchar(64)\"`"+`
}
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	writeTestPO(t, root, "repository_mysql_test.go", generatedRepositoryMySQLTest)
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "./..."}} {
		command := exec.Command("go", args...)
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off", "YUNKA_TEST_MYSQL_DSN="+dsn)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, output)
		}
	}
}

const generatedRepositoryMySQLTest = `package c84mysql_test

import (
    "context"
    "errors"
    "os"
    "testing"

    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    device "example.com/c84mysql/internal/device/domain"
    persistence "example.com/c84mysql/internal/device/infrastructure/persistence"
    "example.com/c84mysql/internal/device/ports"
    "github.com/hvritual/yunka.io/framework/core/identity"
)

func mysqlTenantContext(tenant string) context.Context {
    return identity.WithPrincipal(context.Background(), identity.Principal{
        Subject: "user-1", TenantID: tenant, UserID: "user-1", Roles: []string{"operator"},
        AuthMethod: identity.AuthMethodJWT, Authenticated: true,
    })
}

func TestGeneratedRepository(t *testing.T) {
    database, err := gorm.Open(mysql.Open(os.Getenv("YUNKA_TEST_MYSQL_DSN")), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    _ = database.Migrator().DropTable("yk_device_coffee_machine")
    t.Cleanup(func() { _ = database.Migrator().DropTable("yk_device_coffee_machine") })
    if err := persistence.AutoMigrate(context.Background(), database); err != nil { t.Fatal(err) }
    repository, err := persistence.NewCoffeeMachineRepository(database)
    if err != nil { t.Fatal(err) }

    ctxA := mysqlTenantContext("tenant-a")
    ctxB := mysqlTenantContext("tenant-b")
    machine := &device.CoffeeMachine{ID: "machine-1", Serial: "SERIAL-A"}
    if err := repository.Create(ctxA, machine); err != nil { t.Fatal(err) }
    if machine.TenantID != "tenant-a" || machine.Version != 1 { t.Fatalf("create drift: %#v", machine) }
    loaded, err := repository.Get(ctxA, machine.ID)
    if err != nil { t.Fatal(err) }
    if loaded.Serial != "SERIAL-A" || loaded.TenantID != "tenant-a" { t.Fatalf("unexpected record: %#v", loaded) }
    if _, err := repository.Get(ctxB, machine.ID); !errors.Is(err, ports.ErrNotFound) { t.Fatalf("cross-tenant err=%v", err) }

    loaded.Serial = "SERIAL-B"
    if err := repository.Update(ctxA, &loaded, 1); err != nil { t.Fatal(err) }
    loaded.Serial = "SERIAL-C"
    if err := repository.Update(ctxA, &loaded, 1); !errors.Is(err, ports.ErrConflict) { t.Fatalf("stale update err=%v", err) }
    current, err := repository.Get(ctxA, machine.ID)
    if err != nil { t.Fatal(err) }
    if current.Serial != "SERIAL-B" || current.Version != 2 { t.Fatalf("update drift: %#v", current) }
}
`
