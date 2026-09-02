package selector

import (
	"errors"

	"github.com/hvritual/yunka.io/pkg/registry"
	registrycache "github.com/hvritual/yunka.io/pkg/registry/cache"
)

// The fake registry below does not implement a live watcher. Disable the
// registry cache for selector tests so direct fixture mutations are observed
// deterministically instead of waiting for the production cache TTL.
func init() {
	registrycache.DefaultTTL = 0
}

// Complete the registry.Registry contract for the W05 test double. Keeping
// these methods in a separate test file avoids coupling the adaptive selector
// tests to whichever subset of Registry methods they happen to exercise.
func (current *fakeRegistry) Init(...registry.Option) error { return nil }
func (current *fakeRegistry) Options() registry.Options     { return registry.Options{} }
func (current *fakeRegistry) Register(*registry.Service, ...registry.RegisterOption) error {
	return nil
}
func (current *fakeRegistry) Deregister(*registry.Service) error { return nil }
func (current *fakeRegistry) ListServices() ([]*registry.Service, error) {
	var services []*registry.Service
	for _, values := range current.services {
		services = append(services, values...)
	}
	return services, nil
}
func (current *fakeRegistry) Watch(...registry.WatchOption) (registry.Watcher, error) {
	return nil, errors.New("fake registry watch is unsupported")
}
func (current *fakeRegistry) String() string { return "fake" }
