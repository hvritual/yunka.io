package outboxruntime

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
	"yunka.io/framework/core/eventBus"
	"yunka.io/pkg/logExt"
)

func TestGeneratedDescriptorDeclaresCompleteCapabilities(t *testing.T) {
	descriptor := GeneratedDescriptor()
	if descriptor.Name != ModuleName {
		t.Fatalf("name=%q", descriptor.Name)
	}
	if descriptor.Requirements.ConfigKey != "modules.outboxruntime" || !descriptor.Requirements.Logger || !descriptor.Requirements.EventBus {
		t.Fatalf("requirements=%+v", descriptor.Requirements)
	}
	if len(descriptor.Requirements.Databases) != 1 || descriptor.Requirements.Databases[0].Name != "primary" {
		t.Fatalf("databases=%+v", descriptor.Requirements.Databases)
	}
}

func TestModuleInstancesAreIsolated(t *testing.T) {
	dependencies := func() Dependencies {
		return Dependencies{
			Config:          DefaultConfig(),
			Logger:          logExt.NewBaseLogger(),
			PrimaryDatabase: &gorm.DB{},
			EventBus:        eventBus.NewTrieEventBus(),
		}
	}
	first, err := NewModule(dependencies())
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewModule(dependencies())
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.store == second.store || first.dispatcher == second.dispatcher || first.broker == second.broker {
		t.Fatal("module instances share app-owned runtime state")
	}
	if err := first.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := second.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRejectsShortLease(t *testing.T) {
	config := DefaultConfig()
	config.BatchSize = 10
	config.Concurrency = 1
	config.PublishTimeout = time.Second
	config.LeaseDuration = 5 * time.Second
	if err := config.Validate(); err == nil {
		t.Fatal("short lease accepted")
	}
}
