package module

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

type lifecycleInfraA struct{ events *[]string }
type lifecycleInfraB struct{ events *[]string }

func (i *lifecycleInfraA) Start(context.Context) error {
	*i.events = append(*i.events, "start:a")
	return nil
}
func (i *lifecycleInfraA) Shutdown(context.Context) error {
	*i.events = append(*i.events, "shutdown:a")
	return nil
}
func (i *lifecycleInfraA) Health(context.Context) error {
	*i.events = append(*i.events, "health:a")
	return nil
}

func (i *lifecycleInfraB) Start(context.Context) error {
	*i.events = append(*i.events, "start:b")
	return nil
}
func (i *lifecycleInfraB) Shutdown(context.Context) error {
	*i.events = append(*i.events, "shutdown:b")
	return nil
}
func (i *lifecycleInfraB) Health(context.Context) error {
	*i.events = append(*i.events, "health:b")
	return nil
}

func TestModuleManagesSingletonLifecycle(t *testing.T) {
	events := make([]string, 0)
	mod := &module{name: "test", singleInfras: &sync.Map{}}

	a := &lifecycleInfraA{events: &events}
	b := &lifecycleInfraB{events: &events}
	typeA := reflect.TypeOf(a)
	typeB := reflect.TypeOf(b)
	mod.singleInfras.Store(typeA, &infra{rType: typeA, obj: a})
	mod.singleInfras.Store(typeB, &infra{rType: typeB, obj: b})

	if err := mod.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := mod.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if err := mod.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := mod.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}

	want := []string{
		"start:a",
		"start:b",
		"health:a",
		"health:b",
		"shutdown:b",
		"shutdown:a",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}
