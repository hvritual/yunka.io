package direct

import (
	"yunka.io/pkg/registry"
	"github.com/google/uuid"
	"sync"
)

/**
* @Description: TODO
* @date 2019-09-04
* @version V1.0
*/
type Direct struct {
	Services map[string][]*registry.Service
	sync.RWMutex
}

func (d *Direct) Init(...registry.Option) error {
	return nil
}

func (d *Direct) Options() registry.Options {
	return registry.Options{}
}

func (d *Direct) Register(s *registry.Service, opts ...registry.RegisterOption) error {
	d.Lock()
	if service, ok := d.Services[s.Name]; !ok {
		d.Services[s.Name] = []*registry.Service{s}
	} else {
		d.Services[s.Name] = registry.Merge(service, []*registry.Service{s})
	}
	d.Unlock()

	return nil

}

func (d *Direct) Deregister(s *registry.Service) error {
	d.Lock()
	if service, ok := d.Services[s.Name]; ok {
		if service := registry.Remove(service, []*registry.Service{s}); len(service) == 0 {
			delete(d.Services, s.Name)
		} else {
			d.Services[s.Name] = service
		}
	}
	d.Unlock()

	return nil
}

func (d *Direct) GetService(name string) ([]*registry.Service, error) {
	d.RLock()
	service, ok := d.Services[name]
	d.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}

	return service, nil
}

func (d *Direct) ListServices() ([]*registry.Service, error) {
	var services []*registry.Service
	d.RLock()
	for _, service := range d.Services {
		services = append(services, service...)
	}
	d.RUnlock()
	return services, nil
}

func (d *Direct) Watch(opts ...registry.WatchOption) (registry.Watcher, error) {
	var wo registry.WatchOptions
	for _, o := range opts {
		o(&wo)
	}

	w := &Watcher{
		exit: make(chan bool),
		res:  make(chan *registry.Result),
		id:   uuid.New().String(),
		wo:   wo,
	}

	return w, nil
}

func (d *Direct) String() string {
	return "direct"
}
