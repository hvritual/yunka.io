package module

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"yunka.io/framework/core"
)

var (
	_ core.Startable     = (*module)(nil)
	_ core.Shutdowner    = (*module)(nil)
	_ core.HealthChecker = (*module)(nil)
)

func (mod *module) managedSingleInfras() []*infra {
	mod.singleInfraMu.RLock()
	order := append([]reflect.Type(nil), mod.singleInfraOrder...)
	mod.singleInfraMu.RUnlock()

	managed := make([]*infra, 0, len(order))
	seen := make(map[reflect.Type]struct{}, len(order))
	for _, rType := range order {
		value, ok := mod.singleInfras.Load(rType)
		if !ok {
			continue
		}
		item, ok := value.(*infra)
		if !ok || item == nil {
			continue
		}
		managed = append(managed, item)
		seen[rType] = struct{}{}
	}

	// Keep compatibility with singleton infrastructures inserted before lifecycle
	// ordering existed, or by tests/tools that populate the map directly.
	extras := make([]*infra, 0)
	mod.singleInfras.Range(func(key, value interface{}) bool {
		rType, ok := key.(reflect.Type)
		if !ok {
			return true
		}
		if _, ok := seen[rType]; ok {
			return true
		}
		if item, ok := value.(*infra); ok && item != nil {
			extras = append(extras, item)
		}
		return true
	})
	sort.Slice(extras, func(i, j int) bool {
		return extras[i].rType.String() < extras[j].rType.String()
	})
	return append(managed, extras...)
}

func (mod *module) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	mod.lifecycleMu.Lock()
	defer mod.lifecycleMu.Unlock()
	if mod.started {
		return nil
	}
	if mod.stopped {
		return fmt.Errorf("module %s is stopped", mod.name)
	}

	managed := mod.managedSingleInfras()
	for _, item := range managed {
		starter, ok := item.obj.(core.Startable)
		if !ok {
			continue
		}
		if err := safeManagedCall("start "+item.rType.String(), func() error {
			return starter.Start(ctx)
		}); err != nil {
			for i := len(managed) - 1; i >= 0; i-- {
				_ = shutdownManaged(ctx, managed[i].obj)
			}
			mod.stopped = true
			return err
		}
	}

	mod.started = true
	return nil
}

func (mod *module) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	mod.lifecycleMu.Lock()
	defer mod.lifecycleMu.Unlock()
	if mod.stopped {
		return nil
	}

	managed := mod.managedSingleInfras()
	var errs []error
	for i := len(managed) - 1; i >= 0; i-- {
		if err := shutdownManaged(ctx, managed[i].obj); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", managed[i].rType.String(), err))
		}
	}
	mod.stopped = true
	return errors.Join(errs...)
}

func (mod *module) Health(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	mod.lifecycleMu.Lock()
	defer mod.lifecycleMu.Unlock()

	managed := mod.managedSingleInfras()
	var errs []error
	for _, item := range managed {
		checker, ok := item.obj.(core.HealthChecker)
		if !ok {
			continue
		}
		if err := safeManagedCall("health "+item.rType.String(), func() error {
			return checker.Health(ctx)
		}); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", item.rType.String(), err))
		}
	}
	return errors.Join(errs...)
}

func (mod *module) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), core.DefaultShutdownTimeout)
	defer cancel()
	_ = mod.Shutdown(ctx)
}

func shutdownManaged(ctx context.Context, resource interface{}) error {
	if resource == nil {
		return nil
	}
	if shutdowner, ok := resource.(core.Shutdowner); ok {
		return safeManagedCall("shutdown managed resource", func() error {
			return shutdowner.Shutdown(ctx)
		})
	}
	if closer, ok := resource.(io.Closer); ok {
		return safeManagedCall("close managed resource", closer.Close)
	}
	return nil
}

func safeManagedCall(name string, fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panicked: %v", name, recovered)
		}
	}()
	return fn()
}
