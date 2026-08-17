package resource

import (
	"errors"
	"io"
	"testing"
)

type mockResource struct {
	counter int32
}

func (m *mockResource) Close() error {
	return errors.New("resource close error")
}

func TestManager_Get(t *testing.T) {

	manager := NewManager()
	defer manager.Close()

	var (
		loop    = 100
		counter int32
	)

	for i := 0; i < loop; i++ {
		get, err := manager.Get("foo", func() (io.Closer, error) {
			counter++
			return &mockResource{counter}, nil
		})
		if err != nil {
			t.Errorf("manager get key: get = %v, want = %v", err, nil)
		}

		if v, ok := get.(*mockResource); !ok || v.counter != 1 {
			t.Errorf("manager get counter should be 1, got = %v", v.counter)
		}
	}

}

func TestManager_GetErr(t *testing.T) {

	manager := NewManager()
	defer manager.Close()

	for i := 0; i < 10; i++ {
		_, err := manager.Get("foo", func() (io.Closer, error) {
			return nil, errors.New("fail")
		})
		if err == nil {
			t.Errorf("err should not be nil, got = %v", err)
		}
	}
}
