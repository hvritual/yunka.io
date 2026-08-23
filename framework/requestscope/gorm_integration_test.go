//go:build integration

package requestscope

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type integrationRepositories struct {
	database *gorm.DB
	table    string
}

func TestGORMRequestScopeCommitRollbackAndPanic(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	table := fmt.Sprintf("c7_request_scope_%d", time.Now().UnixNano())
	if err := database.Exec("CREATE TABLE `" + table + "` (`id` BIGINT NOT NULL AUTO_INCREMENT, `value` VARCHAR(64) NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Exec("DROP TABLE IF EXISTS `" + table + "`").Error })

	unitOfWork, err := NewGORMFactory(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewFactory(FactoryOptions[integrationRepositories]{
		UnitOfWork: unitOfWork,
		Repositories: GORMRepositories(func(_ context.Context, tx *gorm.DB) (integrationRepositories, error) {
			return integrationRepositories{database: tx, table: table}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := Execute(context.Background(), factory, func(scope *Scope[integrationRepositories]) error {
		repositories := scope.Repositories()
		return repositories.database.Table(repositories.table).Create(map[string]interface{}{"value": "committed"}).Error
	}); err != nil {
		t.Fatal(err)
	}
	businessErr := errors.New("rollback")
	err = Execute(context.Background(), factory, func(scope *Scope[integrationRepositories]) error {
		repositories := scope.Repositories()
		if err := repositories.database.Table(repositories.table).Create(map[string]interface{}{"value": "rolled-back"}).Error; err != nil {
			return err
		}
		return businessErr
	})
	if !errors.Is(err, businessErr) {
		t.Fatalf("error=%v", err)
	}

	func() {
		defer func() {
			if recovered := recover(); recovered != "rollback-panic" {
				t.Fatalf("recovered=%v", recovered)
			}
		}()
		_ = Execute(context.Background(), factory, func(scope *Scope[integrationRepositories]) error {
			repositories := scope.Repositories()
			if err := repositories.database.Table(repositories.table).Create(map[string]interface{}{"value": "panic"}).Error; err != nil {
				return err
			}
			panic("rollback-panic")
		})
	}()

	var count int64
	if err := database.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count=%d want=1", count)
	}
}

func TestGORMFactoryWithTxOptionsUsesSinglePoolConnection(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	factory, err := NewGORMFactory(database, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unit, err := factory.Begin(ctx)
	if err != nil {
		t.Fatalf("begin with TxOptions: %v", err)
	}
	if err := unit.Rollback(cleanupContext(ctx)); err != nil {
		t.Fatal(err)
	}
	if err := unit.Close(); err != nil {
		t.Fatal(err)
	}
}
