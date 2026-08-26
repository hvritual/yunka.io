package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hvritual/biz/modules/deviceops"
	_ "github.com/hvritual/biz/modules/deviceops/autoload"
	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/framework/kernel"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

type configProvider struct {
	deviceops deviceops.Config
}

func (provider configProvider) Decode(moduleName, key string, target any) error {
	if moduleName != deviceops.ModuleName || key != "modules.deviceops" {
		return fmt.Errorf("unsupported module config %s/%s", moduleName, key)
	}
	config, ok := target.(*deviceops.Config)
	if !ok || config == nil {
		return fmt.Errorf("deviceops config target must be *deviceops.Config")
	}
	*config = provider.deviceops
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("YUNKA_BIZ_MYSQL_DSN")
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}

	config := deviceops.DefaultConfig()
	if value := os.Getenv("YUNKA_BIZ_LISTEN"); value != "" {
		config.ListenAddress = value
	}
	config.AutoMigrate = envBool("YUNKA_BIZ_AUTO_MIGRATE", false)
	config.Bootstrap.Token = os.Getenv("YUNKA_BIZ_BOOTSTRAP_TOKEN")
	if config.Bootstrap.Token != "" {
		config.Bootstrap.TenantID = envOr("YUNKA_BIZ_BOOTSTRAP_TENANT_ID", "tenant-demo")
		config.Bootstrap.TenantName = envOr("YUNKA_BIZ_BOOTSTRAP_TENANT_NAME", "Demo Tenant")
		config.Bootstrap.UserID = envOr("YUNKA_BIZ_BOOTSTRAP_USER_ID", "user-owner")
		config.Bootstrap.Email = envOr("YUNKA_BIZ_BOOTSTRAP_EMAIL", "owner@example.invalid")
	}
	if err := config.Validate(); err != nil {
		return err
	}

	logger := logExt.NewBaseLogger()
	bus := eventBus.NewTrieEventBus()
	provider, err := platform.New(platform.Options{
		Config:   configProvider{deviceops: config},
		Logger:   logger,
		EventBus: bus,
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.MySQLFactory{Configurations: map[string]platform.MySQLConfig{
				"primary": {
					DSN:             dsn,
					MaxOpenConns:    32,
					MaxIdleConns:    8,
					ConnMaxLifetime: 30 * time.Minute,
					ConnMaxIdleTime: 5 * time.Minute,
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("build platform provider: %w", err)
	}

	application, err := kernel.New(kernel.Options{Platform: provider, Catalog: modulecatalog.Default()})
	if err != nil {
		return fmt.Errorf("build application: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := application.Start(ctx); err != nil {
		return fmt.Errorf("start application: %w", err)
	}
	logger.Infof("biz application started on %s", config.ListenAddress)
	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown application: %w", err)
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
