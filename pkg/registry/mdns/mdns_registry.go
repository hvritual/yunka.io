// Package mdns is a multicast dns registry
package mdns

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	hash "github.com/mitchellh/hashstructure"
	"yunka.io/pkg/registry"
)

type mdnsTxt struct {
	Service   string
	Version   string
	Endpoints []*registry.Endpoint
	Metadata  map[string]string
}

type mdnsEntry struct {
	hash uint64
	id   string
	node *mdns.Server
}

type mdnsRegistry struct {
	opts registry.Options

	sync.Mutex
	services map[string][]*mdnsEntry
}

func newRegistry(opts ...registry.Option) registry.Registry {
	options := registry.Options{Timeout: 100 * time.Millisecond}
	for _, option := range opts {
		option(&options)
	}
	return &mdnsRegistry{opts: options, services: make(map[string][]*mdnsEntry)}
}

func (m *mdnsRegistry) Init(opts ...registry.Option) error {
	m.Lock()
	defer m.Unlock()
	for _, option := range opts {
		option(&m.opts)
	}
	return nil
}

func (m *mdnsRegistry) Options() registry.Options {
	m.Lock()
	defer m.Unlock()
	return m.opts
}

func (m *mdnsRegistry) Register(service *registry.Service, opts ...registry.RegisterOption) error {
	if service == nil || service.Name == "" {
		return registry.ErrNotFound
	}

	m.Lock()
	defer m.Unlock()

	entries := m.services[service.Name]

	var registerErr error
	for _, node := range service.Nodes {
		if node == nil {
			continue
		}
		h, err := hash.Hash(node, nil)
		if err != nil {
			registerErr = err
			continue
		}

		var entry *mdnsEntry
		entryIndex := -1
		for index, existing := range entries {
			if node.Id == existing.id {
				entry = existing
				entryIndex = index
				break
			}
		}
		if entry != nil && entry.hash == h {
			continue
		}
		if entry != nil && entry.node != nil {
			_ = entry.node.Shutdown()
		}

		txt, err := encode(&mdnsTxt{
			Service:   service.Name,
			Version:   service.Version,
			Endpoints: service.Endpoints,
			Metadata:  node.Metadata,
		})
		if err != nil {
			registerErr = err
			continue
		}

		host, portText, err := net.SplitHostPort(node.Address)
		if err != nil {
			registerErr = err
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			registerErr = err
			continue
		}

		s, err := mdns.NewMDNSService(
			node.Id,
			service.Name,
			"",
			"",
			port,
			[]net.IP{net.ParseIP(host)},
			txt,
		)
		if err != nil {
			registerErr = err
			continue
		}
		srv, err := mdns.NewServer(&mdns.Config{Zone: s})
		if err != nil {
			registerErr = err
			continue
		}

		updated := &mdnsEntry{hash: h, id: node.Id, node: srv}
		if entryIndex >= 0 {
			entries[entryIndex] = updated
		} else {
			entries = append(entries, updated)
		}
	}

	m.services[service.Name] = entries
	return registerErr
}

func (m *mdnsRegistry) Deregister(service *registry.Service) error {
	if service == nil {
		return nil
	}

	m.Lock()
	defer m.Unlock()

	entries := m.services[service.Name]
	if len(entries) == 0 {
		return nil
	}

	removeIDs := make(map[string]struct{}, len(service.Nodes))
	for _, node := range service.Nodes {
		if node != nil {
			removeIDs[node.Id] = struct{}{}
		}
	}

	newEntries := make([]*mdnsEntry, 0, len(entries))
	for _, entry := range entries {
		if _, remove := removeIDs[entry.id]; remove {
			if entry.node != nil {
				_ = entry.node.Shutdown()
			}
			continue
		}
		newEntries = append(newEntries, entry)
	}

	if len(newEntries) == 0 {
		delete(m.services, service.Name)
	} else {
		m.services[service.Name] = newEntries
	}
	return nil
}

func (m *mdnsRegistry) queryTimeout() time.Duration {
	m.Lock()
	defer m.Unlock()
	if m.opts.Timeout <= 0 {
		return 100 * time.Millisecond
	}
	return m.opts.Timeout
}

func (m *mdnsRegistry) GetService(service string) ([]*registry.Service, error) {
	serviceMap := make(map[string]*registry.Service)
	entries := make(chan *mdns.ServiceEntry, 128)
	p := mdns.DefaultParams(service)
	p.Timeout = m.queryTimeout()
	p.Entries = entries

	if err := mdns.Query(p); err != nil {
		return nil, err
	}
	close(entries)
	for entry := range entries {
		if entry == nil {
			continue
		}
		txt, err := decode(entry.InfoFields)
		if err != nil || txt.Service != service {
			continue
		}
		current, ok := serviceMap[txt.Version]
		if !ok {
			current = &registry.Service{
				Name:      txt.Service,
				Version:   txt.Version,
				Endpoints: txt.Endpoints,
			}
			serviceMap[txt.Version] = current
		}
		address := serviceEntryAddress(entry)
		if address == "" {
			continue
		}
		current.Nodes = append(current.Nodes, &registry.Node{
			Id:       strings.TrimSuffix(entry.Name, "."+p.Service+"."+p.Domain+"."),
			Address:  fmt.Sprintf("%s:%d", address, entry.Port),
			Metadata: txt.Metadata,
		})
	}

	services := make([]*registry.Service, 0, len(serviceMap))
	for _, current := range serviceMap {
		services = append(services, current)
	}
	return services, nil
}

func serviceEntryAddress(entry *mdns.ServiceEntry) string {
	if entry == nil {
		return ""
	}
	if len(entry.AddrV4) != 0 {
		return entry.AddrV4.String()
	}
	if len(entry.AddrV6) != 0 {
		return entry.AddrV6.String()
	}
	return ""
}

func (m *mdnsRegistry) ListServices() ([]*registry.Service, error) {
	serviceMap := make(map[string]bool)
	entries := make(chan *mdns.ServiceEntry, 128)
	p := mdns.DefaultParams("_services._dns-sd._udp")
	p.Timeout = m.queryTimeout()
	p.Entries = entries

	if err := mdns.Query(p); err != nil {
		return nil, err
	}
	close(entries)
	for entry := range entries {
		if entry == nil {
			continue
		}
		txt, err := decode(entry.InfoFields)
		if err == nil && txt.Service != "" {
			serviceMap[txt.Service] = true
		}
	}

	services := make([]*registry.Service, 0, len(serviceMap))
	for name := range serviceMap {
		services = append(services, &registry.Service{Name: name})
	}
	return services, nil
}

func (m *mdnsRegistry) Watch(opts ...registry.WatchOption) (registry.Watcher, error) {
	var watchOptions registry.WatchOptions
	for _, option := range opts {
		option(&watchOptions)
	}
	return newMDNSWatcher(m, watchOptions), nil
}

func (m *mdnsRegistry) String() string { return "mdns" }

// NewRegistry returns a new default registry which is mdns.
func NewRegistry(opts ...registry.Option) registry.Registry { return newRegistry(opts...) }
