#!/usr/bin/env python3
from pathlib import Path

source_path = Path("gateway/dispatcher/intercept/role/db/sqlite_db.go")
test_path = Path("gateway/dispatcher/intercept/role/db/migrate_concurrency_test.go")
text = source_path.read_text(encoding="utf-8")

if "roleSchemaMigrationMu" in text or test_path.exists():
    raise SystemExit("C8.3.3: role migration patch already present")

standard_import_marker = '\t"strings"\n'
if text.count(standard_import_marker) != 1:
    raise SystemExit("C8.3.3: unexpected strings import shape")
text = text.replace(
    standard_import_marker,
    standard_import_marker + '\t"sync"\n\t"time"\n',
    1,
)

gorm_import_marker = '\t"gorm.io/driver/sqlite"\n'
if text.count(gorm_import_marker) != 1:
    raise SystemExit("C8.3.3: unexpected GORM import shape")
text = text.replace(
    gorm_import_marker,
    '\tmysqlDriver "github.com/go-sql-driver/mysql"\n' + gorm_import_marker,
    1,
)

signature = "func Migrate(database *gorm.DB) error {"
start = text.find(signature)
if start < 0 or text.find(signature, start + 1) >= 0:
    raise SystemExit("C8.3.3: expected exactly one Migrate function")
brace = text.find("{", start)
depth = 0
end = -1
for index in range(brace, len(text)):
    char = text[index]
    if char == "{":
        depth += 1
    elif char == "}":
        depth -= 1
        if depth == 0:
            end = index + 1
            break
if end < 0:
    raise SystemExit("C8.3.3: unterminated Migrate function")
fragment = text[start:end]
required_models = (
    "&ApiModuleButton{}",
    "&RoleModuleButton{}",
    "&RolePermission{}",
    "&ButtonPermission{}",
)
if fragment.count("database.AutoMigrate") != 1 or any(model not in fragment for model in required_models):
    raise SystemExit("C8.3.3: unexpected role migration model set")

replacement = '''var roleSchemaMigrationMu sync.Mutex

const roleSchemaMigrationAttempts = 4

func Migrate(database *gorm.DB) error {
	if database == nil {
		return errors.New("role db: GORM database is required")
	}

	// AutoMigrate performs a check-then-create sequence. Multiple application
	// instances can legitimately start against the same schema at once, so
	// serialize callers in this process and retry only idempotent DDL conflicts
	// caused by another process winning the same migration race.
	roleSchemaMigrationMu.Lock()
	defer roleSchemaMigrationMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < roleSchemaMigrationAttempts; attempt++ {
		lastErr = database.AutoMigrate(
			&ApiModuleButton{},
			&RoleModuleButton{},
			&RolePermission{},
			&ButtonPermission{},
		)
		if lastErr == nil {
			return nil
		}
		if !isConcurrentRoleSchemaMigrationError(lastErr) {
			return lastErr
		}
		if attempt+1 < roleSchemaMigrationAttempts {
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
		}
	}
	return lastErr
}

func isConcurrentRoleSchemaMigrationError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1050, 1060, 1061: // table, column, or index already exists
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exists") ||
		strings.Contains(message, "duplicate column name") ||
		strings.Contains(message, "duplicate key name")
}'''

text = text[:start] + replacement + text[end:]
source_path.write_text(text, encoding="utf-8")

test_path.write_text('''package db

import (
	"errors"
	"sync"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateSerializesConcurrentCalls(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:c83_role_migrate?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(8)

	const workers = 8
	start := make(chan struct{})
	results := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer group.Done()
			<-start
			results <- Migrate(database)
		}()
	}
	close(start)
	group.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatalf("concurrent Migrate returned %v", err)
		}
	}
}

func TestConcurrentRoleSchemaMigrationErrorClassification(t *testing.T) {
	for _, number := range []uint16{1050, 1060, 1061} {
		if !isConcurrentRoleSchemaMigrationError(&mysqlDriver.MySQLError{Number: number, Message: "concurrent DDL"}) {
			t.Fatalf("MySQL error %d was not classified as a concurrent migration conflict", number)
		}
	}
	if !isConcurrentRoleSchemaMigrationError(errors.New("table role_permission already exists")) {
		t.Fatal("SQLite-compatible already-exists error was not classified")
	}
	if isConcurrentRoleSchemaMigrationError(errors.New("permission denied")) {
		t.Fatal("unrelated database error was classified as retryable")
	}
}
''', encoding="utf-8")

print("C8.3.3: patched role schema migration concurrency")
