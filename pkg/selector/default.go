package selector

import (
	"math/rand"
	"sync"
	"time"

	"github.com/hvritual/yunka.io/pkg/registry"
	"github.com/hvritual/yunka.io/pkg/registry/cache"
)

type registrySelector struct {
	so Options
	rc cache.Cache

	mu     sync.Mutex
	states map[string]map[string]*nodeState
	rng    *rand.Rand
}

func (c *registrySelector) newCache() cache.Cache {
	ropts := []cache.Option{}
	if c.so.Context != nil {
		if t, ok := c.so.Context.Value("selector_ttl").(time.Duration); ok {
			ropts = append(ropts, cache.WithTTL(t))
		}
	}
	return cache.New(c.so.Registry, ropts...)
}

func (c *registrySelector) Init(opts ...Option) error {
	for _, o := range opts {
		o(&c.so)
	}
	c.so.Adaptive = normalizeAdaptive(c.so.Adaptive)
	c.mu.Lock()
	c.states = make(map[string]map[string]*nodeState)
	seed := c.so.Adaptive.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	c.rng = rand.New(rand.NewSource(seed))
	c.mu.Unlock()

	c.rc.Stop()
	c.rc = c.newCache()
	return nil
}

func (c *registrySelector) Options() Options { return c.so }

func (c *registrySelector) services(service string, opts ...SelectOption) ([]*registry.Service, SelectOptions, error) {
	sopts := SelectOptions{Strategy: c.so.Strategy}
	for _, opt := range opts {
		opt(&sopts)
	}
	services, err := c.rc.GetService(service)
	if err != nil {
		return nil, sopts, err
	}
	if c.so.Adaptive.Enabled {
		c.stateCandidates(service, services)
	}
	for _, filter := range sopts.Filters {
		services = filter(services)
	}
	if len(services) == 0 {
		return nil, sopts, ErrNoneAvailable
	}
	return services, sopts, nil
}

func (c *registrySelector) Select(service string, opts ...SelectOption) (Next, error) {
	services, sopts, err := c.services(service, opts...)
	if err != nil {
		return nil, err
	}
	if !c.so.Adaptive.Enabled {
		return sopts.Strategy(services), nil
	}
	mode := c.so.Adaptive.Mode
	if sopts.AdaptiveMode != nil {
		mode = *sopts.AdaptiveMode
	}
	return func() (*registry.Node, error) {
		selection, err := c.choose(service, services, mode, false)
		if err != nil {
			return nil, err
		}
		return selection.Node, nil
	}, nil
}

func (c *registrySelector) Pick(service string, opts ...SelectOption) (*Selection, error) {
	services, sopts, err := c.services(service, opts...)
	if err != nil {
		return nil, err
	}
	if !c.so.Adaptive.Enabled {
		next := sopts.Strategy(services)
		node, err := next()
		if err != nil {
			return nil, err
		}
		selection := &Selection{Node: cloneNode(node), started: time.Now()}
		selection.finish = func(_ time.Duration, err error, _ Outcome) { c.Mark(service, node, err) }
		return selection, nil
	}
	mode := c.so.Adaptive.Mode
	if sopts.AdaptiveMode != nil {
		mode = *sopts.AdaptiveMode
	}
	return c.choose(service, services, mode, true)
}

func (c *registrySelector) Mark(service string, node *registry.Node, err error) {
	if node == nil || !c.so.Adaptive.Enabled {
		return
	}
	c.mu.Lock()
	states := c.states[service]
	var key string
	for currentKey, state := range states {
		idMatch := node.Id != "" && state.node.Id == node.Id
		addressMatch := node.Address == "" || state.node.Address == node.Address
		if (idMatch && addressMatch) || (node.Id == "" && state.node.Address == node.Address) {
			key = currentKey
			break
		}
	}
	c.mu.Unlock()
	if key != "" {
		c.record(service, key, 0, err, OutcomeAuto, false)
	}
}

func (c *registrySelector) Reset(service string) {
	c.mu.Lock()
	delete(c.states, service)
	c.mu.Unlock()
}

func (c *registrySelector) Close() error {
	c.rc.Stop()
	c.mu.Lock()
	c.states = nil
	c.mu.Unlock()
	return nil
}

func (c *registrySelector) String() string {
	if c.so.Adaptive.Enabled {
		return "registry-adaptive"
	}
	return "registry"
}

func newRegistrySelector(options Options) *registrySelector {
	options.Adaptive = normalizeAdaptive(options.Adaptive)
	seed := options.Adaptive.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	s := &registrySelector{
		so:     options,
		states: make(map[string]map[string]*nodeState),
		rng:    rand.New(rand.NewSource(seed)),
	}
	s.rc = s.newCache()
	return s
}

func NewSelector(opts ...Option) Selector {
	sopts := Options{Strategy: Random, Adaptive: defaultAdaptiveOptions()}
	for _, opt := range opts {
		opt(&sopts)
	}
	if sopts.Registry == nil {
		sopts.Registry = registry.DefaultRegistry
	}
	return newRegistrySelector(sopts)
}

// NewAdaptiveSelector returns a selector with P2C enabled by default while
// keeping NewSelector's legacy random behavior unchanged.
func NewAdaptiveSelector(opts ...Option) Selector {
	sopts := Options{Strategy: Random, Adaptive: defaultAdaptiveOptions()}
	for _, opt := range opts {
		opt(&sopts)
	}
	sopts.Adaptive.Enabled = true
	if sopts.Registry == nil {
		sopts.Registry = registry.DefaultRegistry
	}
	return newRegistrySelector(sopts)
}
