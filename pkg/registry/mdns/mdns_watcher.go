package mdns

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"yunka.io/pkg/registry"
)

type mdnsWatcher struct {
	wo       registry.WatchOptions
	registry *mdnsRegistry
	interval time.Duration
	results  chan *registry.Result
	exit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

func newMDNSWatcher(r *mdnsRegistry, options registry.WatchOptions) *mdnsWatcher {
	interval := 2 * r.queryTimeout()
	if interval < 250*time.Millisecond {
		interval = 250 * time.Millisecond
	}
	watcher := &mdnsWatcher{
		wo:       options,
		registry: r,
		interval: interval,
		results:  make(chan *registry.Result, 64),
		exit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go watcher.run()
	return watcher
}

func (m *mdnsWatcher) Next() (*registry.Result, error) {
	select {
	case result, ok := <-m.results:
		if !ok {
			return nil, registry.ErrWatcherStopped
		}
		return result, nil
	case <-m.exit:
		return nil, registry.ErrWatcherStopped
	}
}

func (m *mdnsWatcher) Stop() {
	m.stopOnce.Do(func() { close(m.exit) })
}

func (m *mdnsWatcher) run() {
	defer close(m.done)
	defer close(m.results)

	previous, _ := m.snapshot()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.exit:
			return
		case <-ticker.C:
			current, err := m.snapshot()
			if err != nil {
				continue
			}
			for _, result := range diffMDNSSnapshots(previous, current) {
				select {
				case m.results <- result:
				case <-m.exit:
					return
				}
			}
			previous = current
		}
	}
}

type mdnsSnapshot map[string]*registry.Service

func (m *mdnsWatcher) snapshot() (mdnsSnapshot, error) {
	result := make(mdnsSnapshot)
	if m.wo.Service != "" {
		services, err := m.registry.GetService(m.wo.Service)
		if err != nil {
			return nil, err
		}
		addSnapshotServices(result, services)
		return result, nil
	}

	names, err := m.registry.ListServices()
	if err != nil {
		return nil, err
	}
	for _, named := range names {
		services, queryErr := m.registry.GetService(named.Name)
		if queryErr != nil {
			continue
		}
		addSnapshotServices(result, services)
	}
	return result, nil
}

func addSnapshotServices(snapshot mdnsSnapshot, services []*registry.Service) {
	for _, service := range services {
		if service == nil {
			continue
		}
		for _, node := range service.Nodes {
			if node == nil {
				continue
			}
			copyService := *service
			copyNode := *node
			copyService.Nodes = []*registry.Node{&copyNode}
			key := service.Name + "\x00" + service.Version + "\x00" + node.Id
			snapshot[key] = &copyService
		}
	}
}

func diffMDNSSnapshots(previous, current mdnsSnapshot) []*registry.Result {
	keys := make([]string, 0, len(previous)+len(current))
	seen := make(map[string]struct{}, len(previous)+len(current))
	for key := range previous {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range current {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	results := make([]*registry.Result, 0)
	for _, key := range keys {
		before, hadBefore := previous[key]
		after, hasAfter := current[key]
		switch {
		case !hadBefore && hasAfter:
			results = append(results, &registry.Result{Action: "create", Service: after})
		case hadBefore && !hasAfter:
			results = append(results, &registry.Result{Action: "delete", Service: before})
		case hadBefore && hasAfter && !sameMDNSService(before, after):
			results = append(results, &registry.Result{Action: "update", Service: after})
		}
	}
	return results
}

func sameMDNSService(left, right *registry.Service) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}
