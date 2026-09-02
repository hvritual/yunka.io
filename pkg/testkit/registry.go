package testkit

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/hvritual/yunka.io/pkg/registry"
)

type Registry struct {
	mu       sync.RWMutex
	options  registry.Options
	services map[string]map[string]*registry.Service
	watchers map[uint64]*registryWatcher
	nextID   uint64
}

func NewRegistry() *Registry {
	return &Registry{services: make(map[string]map[string]*registry.Service), watchers: make(map[uint64]*registryWatcher)}
}

func (current *Registry) Init(options ...registry.Option) error {
	current.mu.Lock()
	defer current.mu.Unlock()
	for _, option := range options {
		option(&current.options)
	}
	return nil
}
func (current *Registry) Options() registry.Options {
	current.mu.RLock()
	defer current.mu.RUnlock()
	return current.options
}

func (current *Registry) Register(service *registry.Service, _ ...registry.RegisterOption) error {
	if service == nil || strings.TrimSpace(service.Name) == "" {
		return errors.New("testkit registry: service name is required")
	}
	clone := cloneService(service)
	current.mu.Lock()
	versions := current.services[clone.Name]
	if versions == nil {
		versions = make(map[string]*registry.Service)
		current.services[clone.Name] = versions
	}
	action := "create"
	if _, exists := versions[clone.Version]; exists {
		action = "update"
	}
	versions[clone.Version] = clone
	watchers := current.watchersSnapshotLocked(clone.Name)
	current.mu.Unlock()
	emitRegistryResult(watchers, &registry.Result{Action: action, Service: cloneService(clone)})
	return nil
}

func (current *Registry) Deregister(service *registry.Service) error {
	if service == nil || strings.TrimSpace(service.Name) == "" {
		return errors.New("testkit registry: service name is required")
	}
	current.mu.Lock()
	versions := current.services[service.Name]
	existing := versions[service.Version]
	if existing == nil {
		current.mu.Unlock()
		return nil
	}
	deleted := cloneService(existing)
	if len(service.Nodes) == 0 {
		delete(versions, service.Version)
	} else {
		remove := make(map[string]struct{}, len(service.Nodes))
		for _, node := range service.Nodes {
			if node != nil {
				remove[node.Id] = struct{}{}
			}
		}
		kept := make([]*registry.Node, 0, len(existing.Nodes))
		deleted.Nodes = nil
		for _, node := range existing.Nodes {
			if _, ok := remove[node.Id]; ok {
				deleted.Nodes = append(deleted.Nodes, cloneNode(node))
				continue
			}
			kept = append(kept, cloneNode(node))
		}
		existing.Nodes = kept
		if len(existing.Nodes) == 0 {
			delete(versions, service.Version)
		}
	}
	if len(versions) == 0 {
		delete(current.services, service.Name)
	}
	watchers := current.watchersSnapshotLocked(service.Name)
	current.mu.Unlock()
	emitRegistryResult(watchers, &registry.Result{Action: "delete", Service: deleted})
	return nil
}

func (current *Registry) GetService(name string) ([]*registry.Service, error) {
	current.mu.RLock()
	defer current.mu.RUnlock()
	versions := current.services[name]
	if len(versions) == 0 {
		return nil, registry.ErrNotFound
	}
	keys := make([]string, 0, len(versions))
	for version := range versions {
		keys = append(keys, version)
	}
	sort.Strings(keys)
	result := make([]*registry.Service, 0, len(keys))
	for _, version := range keys {
		result = append(result, cloneService(versions[version]))
	}
	return result, nil
}

func (current *Registry) ListServices() ([]*registry.Service, error) {
	current.mu.RLock()
	defer current.mu.RUnlock()
	names := make([]string, 0, len(current.services))
	for name := range current.services {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]*registry.Service, 0)
	for _, name := range names {
		versions := current.services[name]
		keys := make([]string, 0, len(versions))
		for version := range versions {
			keys = append(keys, version)
		}
		sort.Strings(keys)
		for _, version := range keys {
			result = append(result, cloneService(versions[version]))
		}
	}
	return result, nil
}

func (current *Registry) Watch(options ...registry.WatchOption) (registry.Watcher, error) {
	config := registry.WatchOptions{}
	for _, option := range options {
		option(&config)
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	current.nextID++
	watcher := &registryWatcher{id: current.nextID, service: strings.TrimSpace(config.Service), parent: current, events: make(chan *registry.Result, 64), done: make(chan struct{})}
	current.watchers[watcher.id] = watcher
	if config.Context != nil {
		go func() {
			select {
			case <-config.Context.Done():
				watcher.Stop()
			case <-watcher.done:
			}
		}()
	}
	return watcher, nil
}

func (current *Registry) String() string { return "testkit" }

func (current *Registry) watchersSnapshotLocked(service string) []*registryWatcher {
	result := make([]*registryWatcher, 0)
	for _, watcher := range current.watchers {
		if watcher.service == "" || watcher.service == service {
			result = append(result, watcher)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })
	return result
}

func emitRegistryResult(watchers []*registryWatcher, result *registry.Result) {
	for _, watcher := range watchers {
		select {
		case <-watcher.done:
		case watcher.events <- cloneResult(result):
		}
	}
}

type registryWatcher struct {
	id      uint64
	service string
	parent  *Registry
	events  chan *registry.Result
	done    chan struct{}
	once    sync.Once
}

func (watcher *registryWatcher) Next() (*registry.Result, error) {
	select {
	case <-watcher.done:
		return nil, registry.ErrWatcherStopped
	default:
	}
	select {
	case <-watcher.done:
		return nil, registry.ErrWatcherStopped
	case result := <-watcher.events:
		return cloneResult(result), nil
	}
}
func (watcher *registryWatcher) Stop() {
	watcher.once.Do(func() {
		watcher.parent.mu.Lock()
		delete(watcher.parent.watchers, watcher.id)
		watcher.parent.mu.Unlock()
		close(watcher.done)
	})
}

func cloneResult(result *registry.Result) *registry.Result {
	if result == nil {
		return nil
	}
	return &registry.Result{Action: result.Action, Service: cloneService(result.Service)}
}
func cloneService(service *registry.Service) *registry.Service {
	if service == nil {
		return nil
	}
	clone := *service
	if service.Metadata != nil {
		clone.Metadata = make(map[string]string, len(service.Metadata))
		for key, value := range service.Metadata {
			clone.Metadata[key] = value
		}
	}
	clone.Nodes = make([]*registry.Node, 0, len(service.Nodes))
	for _, node := range service.Nodes {
		clone.Nodes = append(clone.Nodes, cloneNode(node))
	}
	clone.Endpoints = append([]*registry.Endpoint(nil), service.Endpoints...)
	return &clone
}
func cloneNode(node *registry.Node) *registry.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if node.Metadata != nil {
		clone.Metadata = make(map[string]string, len(node.Metadata))
		for key, value := range node.Metadata {
			clone.Metadata[key] = value
		}
	}
	return &clone
}
