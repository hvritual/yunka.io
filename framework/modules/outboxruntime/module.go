package outboxruntime

import (
	"context"
	"errors"
	"fmt"

	"github.com/hvritual/yunka.io/framework/event"
	"github.com/hvritual/yunka.io/framework/event/outbox"
)

const ModuleName = "outboxruntime"

type Module struct {
	dependencies Dependencies
	store        *outbox.GORMStore
	broker       *event.LocalBroker
	dispatcher   *outbox.Dispatcher
	autoMigrate  bool
}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.Logger == nil {
		return nil, fmt.Errorf("%s: logger is required", ModuleName)
	}
	if dependencies.PrimaryDatabase == nil {
		return nil, fmt.Errorf("%s: database primary is required", ModuleName)
	}
	if dependencies.EventBus == nil {
		return nil, fmt.Errorf("%s: event bus is required", ModuleName)
	}
	config := dependencies.Config.normalized()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	store, err := outbox.NewGORMStore(
		dependencies.PrimaryDatabase,
		outbox.WithTable(config.Table),
		outbox.WithSkipLocked(config.SkipLocked),
	)
	if err != nil {
		return nil, err
	}
	broker := event.NewLocalBroker(dependencies.EventBus)
	dispatcher, err := outbox.NewDispatcher(store, broker, outbox.DispatcherConfig{
		WorkerID:               config.WorkerID,
		PollInterval:           config.PollInterval,
		BatchSize:              config.BatchSize,
		Concurrency:            config.Concurrency,
		LeaseDuration:          config.LeaseDuration,
		PublishTimeout:         config.PublishTimeout,
		MaxAttempts:            config.MaxAttempts,
		RetryBase:              config.RetryBase,
		RetryMax:               config.RetryMax,
		RetryJitter:            config.RetryJitter,
		HealthFailureThreshold: config.HealthFailureThreshold,
	})
	if err != nil {
		_ = broker.Close()
		return nil, err
	}
	return &Module{
		dependencies: dependencies,
		store:        store,
		broker:       broker,
		dispatcher:   dispatcher,
		autoMigrate:  config.AutoMigrate,
	}, nil
}

func (*Module) Name() string { return ModuleName }

func (module *Module) Start(ctx context.Context) error {
	if module == nil {
		return errors.New("outboxruntime: module is nil")
	}
	if module.autoMigrate {
		if err := module.store.AutoMigrate(ctx); err != nil {
			return fmt.Errorf("outboxruntime: migrate: %w", err)
		}
	}
	return module.dispatcher.Start(ctx)
}

func (module *Module) Health(ctx context.Context) error {
	if module == nil {
		return errors.New("outboxruntime: module is nil")
	}
	return module.dispatcher.Health(ctx)
}

func (module *Module) Shutdown(ctx context.Context) error {
	if module == nil {
		return nil
	}
	return errors.Join(module.dispatcher.Shutdown(ctx), module.broker.Close())
}

func (module *Module) Store() *outbox.GORMStore {
	if module == nil {
		return nil
	}
	return module.store
}
