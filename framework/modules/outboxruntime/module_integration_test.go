//go:build integration

package outboxruntime

import (
	"context"
	"os"
	"testing"
	"time"

	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/core/eventBus"
	"yunka.io/pkg/logExt"
)

func TestModuleLifecycleMySQL84(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("YUNKA_TEST_MYSQL_DSN is not set")
	}
	database, err := gorm.Open(mysqlDriver.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	config := DefaultConfig()
	config.AutoMigrate = true
	config.SkipLocked = true
	config.Table = "yunka_outbox_c74_runtime"
	defer database.Migrator().DropTable(config.Table)
	config.WorkerID = "c7-4-outboxruntime"
	config.PollInterval = 10 * time.Millisecond
	module, err := NewModule(Dependencies{
		Config:          config,
		Logger:          logExt.NewBaseLogger(),
		PrimaryDatabase: database,
		EventBus:        eventBus.NewTrieEventBus(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := module.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := module.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if err := module.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}
