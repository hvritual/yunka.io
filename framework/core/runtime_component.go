package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// RuntimeComponent is a lifecycle-only leaf owned by one App. It deliberately
// carries no service value or lookup surface: business/runtime dependencies
// remain typed and explicit outside this contract.
type RuntimeComponent struct {
	Name         string
	StartFunc    func(context.Context) error
	HealthFunc   func(context.Context) error
	ShutdownFunc func(context.Context) error
}

// RuntimeInventory is immutable, secret-free observed/declared runtime
// inventory supplied when the App is constructed. It is diagnostics evidence,
// not a service registry or runtime source of truth for execution.
type RuntimeInventory struct {
	Routes              []string
	RPCClientConfigured bool
	RPCServerCount      int
}

func normalizeRuntimeComponents(values []RuntimeComponent) ([]RuntimeComponent, error) {
	result := make([]RuntimeComponent, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		if value.Name == "" {
			return nil, errors.New("core: runtime component name is required")
		}
		if value.StartFunc == nil {
			return nil, fmt.Errorf("core: runtime component %q start function is required", value.Name)
		}
		if value.ShutdownFunc == nil {
			return nil, fmt.Errorf("core: runtime component %q shutdown function is required", value.Name)
		}
		if _, duplicate := seen[value.Name]; duplicate {
			return nil, fmt.Errorf("core: duplicate runtime component %q", value.Name)
		}
		seen[value.Name] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func normalizeRuntimeInventory(value RuntimeInventory) (RuntimeInventory, error) {
	if value.RPCServerCount < 0 {
		return RuntimeInventory{}, errors.New("core: runtime RPC server count cannot be negative")
	}
	seen := make(map[string]struct{}, len(value.Routes))
	routes := make([]string, 0, len(value.Routes))
	for _, route := range value.Routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		if _, duplicate := seen[route]; duplicate {
			continue
		}
		seen[route] = struct{}{}
		routes = append(routes, route)
	}
	sort.Strings(routes)
	value.Routes = routes
	return value, nil
}

func (app *App) runtimeComponentSnapshot() []RuntimeComponent {
	if app == nil {
		return nil
	}
	return append([]RuntimeComponent(nil), app.runtimeComponents...)
}

func shutdownRuntimeComponents(ctx context.Context, components []RuntimeComponent) []error {
	var failures []error
	for index := len(components) - 1; index >= 0; index-- {
		component := components[index]
		if component.ShutdownFunc == nil {
			continue
		}
		if err := safeLifecycleCall("shutdown runtime component "+component.Name, func() error {
			return component.ShutdownFunc(ctx)
		}); err != nil {
			failures = append(failures, fmt.Errorf("shutdown runtime component %s: %w", component.Name, err))
		}
	}
	return failures
}

func mergedRuntimeRoutes(legacy []string, inventory RuntimeInventory) []string {
	seen := make(map[string]struct{}, len(legacy)+len(inventory.Routes))
	result := make([]string, 0, len(legacy)+len(inventory.Routes))
	for _, routes := range [][]string{legacy, inventory.Routes} {
		for _, route := range routes {
			route = strings.TrimSpace(route)
			if route == "" {
				continue
			}
			if _, duplicate := seen[route]; duplicate {
				continue
			}
			seen[route] = struct{}{}
			result = append(result, route)
		}
	}
	sort.Strings(result)
	return result
}
