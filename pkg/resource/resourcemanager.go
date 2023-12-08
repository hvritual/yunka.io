package resource

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type Manager struct {
	resources   map[string]io.Closer
	SharedCalls SharedCalls
	rw          sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		resources:   map[string]io.Closer{},
		SharedCalls: NewSharedCalls(),
	}
}

func (m *Manager) Close() error {
	m.rw.Lock()
	defer m.rw.Unlock()

	var batchErrs []error

	for _, closer := range m.resources {
		if err := closer.Close(); err != nil {
			batchErrs = append(batchErrs, err)
		}
	}

	var buf bytes.Buffer

	for i := range batchErrs {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(batchErrs[i].Error())
	}

	return errors.New(buf.String())

}

func (m *Manager) Get(key string, create func() (io.Closer, error)) (io.Closer, error) {

	do, err := m.SharedCalls.Do(key, func() (interface{}, error) {

		// if exists
		m.rw.RLock()
		closer, ok := m.resources[key]
		m.rw.RUnlock()
		if ok {
			return closer, nil
		}

		// new resource
		resource, err := create()
		if err != nil {
			return nil, err
		}

		m.rw.Lock()
		m.resources[key] = resource
		m.rw.Unlock()

		return resource, nil

	})
	if err != nil {
		return nil, err
	}

	return do.(io.Closer), nil

}
