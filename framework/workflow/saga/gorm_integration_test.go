//go:build integration

package saga

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

func TestTransactionalOutboxSagaBoundary(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("YUNKA_TEST_MYSQL_DSN required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := outbox.NewGORMStore(db, outbox.WithTable("yunka_c87_saga_outbox"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM yunka_c87_saga_outbox").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS yunka_c87_saga_business (id varchar(64) primary key)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM yunka_c87_saga_business").Error; err != nil {
		t.Fatal(err)
	}
	plan := Plan{ID: "deploy-atomic", IdempotencyKey: "req-atomic", Steps: []Step{{ID: "remote", Command: Command{Topic: "remote.command", Type: "remote.create", Payload: json.RawMessage(`{"id":"x"}`)}}}}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := tx.Exec("INSERT INTO yunka_c87_saga_business(id) VALUES (?)", "rollback").Error; err != nil {
		t.Fatal(err)
	}
	if err := EnqueueTx(ctx, store, tx, plan); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Raw("SELECT count(*) FROM yunka_c87_saga_business WHERE id = ?", "rollback").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled back business count=%d", count)
	}
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 0 {
		t.Fatalf("rolled back outbox pending=%d", snapshot.Pending)
	}

	tx = db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := tx.Exec("INSERT INTO yunka_c87_saga_business(id) VALUES (?)", "commit").Error; err != nil {
		t.Fatal(err)
	}
	if err := EnqueueTx(ctx, store, tx, plan); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 1 {
		t.Fatalf("committed outbox pending=%d", snapshot.Pending)
	}

	tx = db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := EnqueueTx(ctx, store, tx, plan); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 1 {
		t.Fatalf("idempotent retry pending=%d", snapshot.Pending)
	}
}
