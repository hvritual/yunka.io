package platform

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry is a typed, sealable provider registry. A registry is instantiated
// separately for each capability type, so consumers never perform an untyped
// Get(string) any lookup.
type Registry[T any] struct {
	mu     sync.RWMutex
	values map[string]T
	sealed bool
}

func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{values: make(map[string]T)}
}

func (registry *Registry[T]) Register(name string, value T) error {
	if registry == nil {
		return fmt.Errorf("platform: nil provider registry")
	}
	name = strings.TrimSpace(name)
	if !capabilityNamePattern.MatchString(name) {
		return fmt.Errorf("platform: invalid provider capability name %q", name)
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.sealed {
		return fmt.Errorf("platform: provider registry is sealed")
	}
	if _, duplicate := registry.values[name]; duplicate {
		return fmt.Errorf("platform: duplicate provider capability %q", name)
	}
	registry.values[name] = value
	return nil
}

func (registry *Registry[T]) Seal() {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	registry.sealed = true
	registry.mu.Unlock()
}

func (registry *Registry[T]) Snapshot() map[string]T {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	result := make(map[string]T, len(registry.values))
	for name, value := range registry.values {
		result[name] = value
	}
	return result
}

func (registry *Registry[T]) Names() []string {
	if registry == nil {
		return nil
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	result := make([]string, 0, len(registry.values))
	for name := range registry.values {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
