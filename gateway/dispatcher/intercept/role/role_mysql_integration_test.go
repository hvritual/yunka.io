//go:build integration

package role

import (
	"context"
	"errors"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/dispatcher/intercept/role/db"
)

func TestRoleRequestScopeMySQLCommitAndRollback(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Exec("DELETE FROM "+db.ApiModuleButtonTableName+" WHERE api_uuid LIKE ?", "c723-%").Error
		_ = database.Exec("DELETE FROM "+db.RoleModuleButtonTableName+" WHERE org_uuid LIKE ?", "c723-%").Error
	})

	units, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := requestscope.NewFactory(requestscope.FactoryOptions[*db.Store]{
		UnitOfWork: units,
		Repositories: requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (*db.Store, error) {
			return db.NewStoreFromDB(transaction)
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	committed := db.ApiModuleButton{ApiUUID: "c723-commit", ModuleButtonUUID: "c723-button-commit"}
	if err := requestscope.Execute(context.Background(), scopes, func(scope *requestscope.Scope[*db.Store]) error {
		return scope.Repositories().BatchCreate([]db.ApiModuleButton{committed})
	}); err != nil {
		t.Fatal(err)
	}

	rollbackErr := errors.New("force role rollback")
	rolledBack := db.ApiModuleButton{ApiUUID: "c723-rollback", ModuleButtonUUID: "c723-button-rollback"}
	err = requestscope.Execute(context.Background(), scopes, func(scope *requestscope.Scope[*db.Store]) error {
		if err := scope.Repositories().BatchCreate([]db.ApiModuleButton{rolledBack}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback error=%v", err)
	}

	var committedCount int64
	if err := database.Table(db.ApiModuleButtonTableName).Where("api_uuid = ?", committed.ApiUUID).Count(&committedCount).Error; err != nil {
		t.Fatal(err)
	}
	if committedCount != 1 {
		t.Fatalf("committed row count=%d want=1", committedCount)
	}
	var rolledBackCount int64
	if err := database.Table(db.ApiModuleButtonTableName).Where("api_uuid = ?", rolledBack.ApiUUID).Count(&rolledBackCount).Error; err != nil {
		t.Fatal(err)
	}
	if rolledBackCount != 0 {
		t.Fatalf("rolled back row count=%d want=0", rolledBackCount)
	}
}
